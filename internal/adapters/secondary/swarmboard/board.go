// Package swarmboard provides the JSONL-backed message board that a swarm
// session's agents use to talk to each other, plus the bootstrap descriptor they
// read to discover it.
package swarmboard

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/dnlopes/overseer/internal/core/domain"
	"github.com/dnlopes/overseer/internal/shared/paths"
)

const currentSchemaVersion = 1

// maxLineBytes caps how long a single board line may be while scanning. A
// message is capped at 8000 characters by the domain, but JSON escaping can
// inflate that several times over, so the default 64KiB scanner budget is too
// tight to rely on.
const maxLineBytes = 1 << 20

// record is the on-disk shape of one board message. Each line carries its own
// schema version: the board is append-only, so a file header could not be
// rewritten without losing the crash-safety of a plain append.
type record struct {
	SchemaVersion int       `json:"v"`
	Seq           int       `json:"seq"`
	SessionID     string    `json:"sessionId"`
	Author        string    `json:"author"`
	Role          string    `json:"role"`
	Content       string    `json:"content"`
	CreatedAt     time.Time `json:"createdAt"`
}

// descriptorFile is the on-disk shape of the bootstrap contract. The post/read
// hints are deliberately rendered here rather than in the domain: which wire
// calls reach the board is a transport detail this adapter owns.
type descriptorFile struct {
	SchemaVersion int      `json:"v"`
	SessionID     string   `json:"sessionId"`
	BoardURL      string   `json:"boardUrl"`
	AgentCount    int      `json:"agentCount"`
	Roster        []string `json:"roster"`
	Goal          string   `json:"goal"`
	Method        []string `json:"method"`
	Conventions   []string `json:"conventions"`
	Post          string   `json:"post"`
	Read          string   `json:"read"`
}

// boardMethod is how to work; boardConventions is how to write it up. They are
// separate lists because they are tuned for different reasons and it should be
// obvious which one you are changing.
//
// Every rule here answers something measured on a real run rather than general
// prompt advice — the motivating observation is on each line.
var boardMethod = []string{
	// Two runs: the one where agents falsified their own claims 11 times had 5
	// reversals; the one where they never did had 10. Pre-testing a claim is the
	// cheapest way to cut correction churn.
	"Before you post a claim, try to break it yourself and say what survived. A claim you have already attacked is worth ten you have not.",
	// An agent had to cede an axis it had duplicated, and three agents once claimed
	// the same write at the same time and had to invent a tie-break on the spot.
	"Read the board before claiming anything, and claim an axis nobody else holds. If two of you claim the same thing, the earliest claim wins — the later one withdraws without arguing.",
	// The single most decision-relevant paragraph either run produced was an agent
	// concluding there was no evidence in either direction, and proving it.
	"Say how confident you are and what would change your mind. \"I cannot tell from the evidence, and here is why each candidate fails\" is a strong answer, not a failure.",
	// One run had to be told, two thirds of the way in, to stop auditing and start
	// converging. Left alone a swarm keeps opening findings.
	"Once several findings are open, prefer closing one to opening another. Answer the goal; do not just keep enumerating problems with it.",
	// Neither run posted a verdict unprompted. The review artefact from the second
	// exists only because the operator asked for it explicitly.
	"When the goal is answered, post a verdict without being asked: what holds, what does not, what is still unknown, and what you recommend. Then stop.",
}

// boardConventions are the posting rules every agent reads at bootstrap.
//
// They live in the descriptor rather than the typed briefing for two reasons: a
// long tmux send-keys string is the brittle part of bootstrapping, and rules the
// agent can re-read beat rules it saw once.
//
// The rules target prose, not evidence. Left ungoverned, agents write ~3KB essays
// per post — measured on a real five-agent run — and the board becomes unreadable
// at exactly the moment it matters. But the numbers and held-out tests inside
// those essays are what catch a bad premise, so the instruction is to compress
// the wording and keep the data.
var boardConventions = []string{
	"Open with one bold line: the claim, and any @agent-N it is aimed at.",
	"Keep the whole post under ~12 lines. Compress prose, never evidence.",
	"Prefer numbers, short tables and file paths over sentences explaining them.",
	"Cite another post as #12 — that is the number the board shows — instead of restating it.",
	"Post only when something changed: a finding, a correction, a claim, or a blocker.",
	"No progress reports, no preamble, no summarising your own post at the end.",
	"Say you were wrong in one line and move on.",
}

var (
	_ domain.SwarmBoardRepository  = (*Board)(nil)
	_ domain.SwarmDescriptorWriter = (*Board)(nil)
)

// Board stores each swarm session's messages in its own append-only JSONL file.
//
// Writes are serialised through mu, which also makes reads safe: because every
// writer goes through the same lock, a reader holding it never observes a line
// this process is midway through appending.
//
// lastSeq caches the per-session high-water mark so an append does not have to
// rescan the file. It is populated lazily from disk on first touch, which is
// what lets a restarted Overseer continue an existing board's sequence.
type Board struct {
	resolver paths.Resolver
	logger   *slog.Logger

	mu      sync.Mutex
	lastSeq map[uuid.UUID]int
}

func New(resolver paths.Resolver, logger *slog.Logger) *Board {
	return &Board{
		resolver: resolver,
		logger:   logger,
		lastSeq:  make(map[uuid.UUID]int),
	}
}

// Append assigns the next sequence number for msg's session and writes it as one
// JSONL line. Seq assignment and the write happen under the same lock, so
// sequences stay gapless and unique even when several agents post at once.
func (b *Board) Append(_ context.Context, msg domain.SwarmMessage) (domain.SwarmMessage, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	seq, err := b.nextSeqLocked(msg.SessionID)
	if err != nil {
		return domain.SwarmMessage{}, err
	}

	line, err := json.Marshal(record{
		SchemaVersion: currentSchemaVersion,
		Seq:           seq,
		SessionID:     msg.SessionID.String(),
		Author:        msg.Author,
		Role:          string(msg.Role),
		Content:       msg.Content,
		CreatedAt:     msg.CreatedAt,
	})
	if err != nil {
		return domain.SwarmMessage{}, fmt.Errorf("swarmboard: marshal message: %w", err)
	}

	path := b.resolver.SwarmBoardFile(msg.SessionID)
	if err := paths.EnsureDir(filepath.Dir(path)); err != nil {
		return domain.SwarmMessage{}, fmt.Errorf("swarmboard: ensure dir: %w", err)
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return domain.SwarmMessage{}, fmt.Errorf("swarmboard: open board: %w", err)
	}
	defer f.Close()

	if _, err := f.Write(append(line, '\n')); err != nil {
		return domain.SwarmMessage{}, fmt.Errorf("swarmboard: append message: %w", err)
	}

	b.lastSeq[msg.SessionID] = seq
	msg.Seq = seq
	return msg, nil
}

func (b *Board) ListSince(_ context.Context, sessionID uuid.UUID, since int) ([]domain.SwarmMessage, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	records, err := b.readLocked(sessionID)
	if err != nil {
		return nil, err
	}

	out := make([]domain.SwarmMessage, 0, len(records))
	for _, rec := range records {
		if rec.Seq <= since {
			continue
		}
		out = append(out, domain.SwarmMessage{
			Seq:       rec.Seq,
			SessionID: sessionID,
			Author:    rec.Author,
			Role:      domain.SwarmRole(rec.Role),
			Content:   rec.Content,
			CreatedAt: rec.CreatedAt,
		})
	}
	return out, nil
}

func (b *Board) Count(_ context.Context, sessionID uuid.UUID) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	records, err := b.readLocked(sessionID)
	if err != nil {
		return 0, err
	}
	return len(records), nil
}

// Purge removes the session's whole swarm directory — board and bootstrap
// descriptor alike — and drops its cached sequence so a session id that somehow
// comes back starts from one. A missing directory is not an error: teardown may
// run for a swarm that never posted anything.
func (b *Board) Purge(_ context.Context, sessionID uuid.UUID) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if err := os.RemoveAll(b.resolver.SwarmDir(sessionID)); err != nil {
		return fmt.Errorf("swarmboard: purge board: %w", err)
	}
	delete(b.lastSeq, sessionID)
	return nil
}

// Write persists the bootstrap descriptor, replacing any previous one. Unlike
// the board, this is a whole-file write, so it goes through AtomicWrite.
func (b *Board) Write(_ context.Context, desc domain.SwarmDescriptor) error {
	path := b.resolver.SwarmDescriptorFile(desc.SessionID)
	if err := paths.EnsureDir(filepath.Dir(path)); err != nil {
		return fmt.Errorf("swarmboard: ensure dir: %w", err)
	}

	messagesURL := fmt.Sprintf("%s/v1/sessions/%s/messages", desc.BoardURL, desc.SessionID)
	data, err := json.MarshalIndent(descriptorFile{
		SchemaVersion: currentSchemaVersion,
		SessionID:     desc.SessionID.String(),
		BoardURL:      desc.BoardURL,
		AgentCount:    desc.AgentCount,
		Roster:        desc.Roster,
		Goal:          desc.Goal,
		Method:        boardMethod,
		Conventions:   boardConventions,
		Post: fmt.Sprintf(
			"curl -sS -X POST %s -H 'Content-Type: application/json' "+
				`-d '{"author":"<your-agent-id>","content":"<your message>"}'`,
			messagesURL,
		),
		Read: fmt.Sprintf("curl -sS '%s?since=<last-seq-you-saw>'", messagesURL),
	}, "", "  ")
	if err != nil {
		return fmt.Errorf("swarmboard: marshal descriptor: %w", err)
	}

	return paths.AtomicWrite(path, data)
}

// nextSeqLocked returns the sequence number to use for the session's next
// message, seeding the cache from disk the first time a session is touched.
func (b *Board) nextSeqLocked(sessionID uuid.UUID) (int, error) {
	if seq, ok := b.lastSeq[sessionID]; ok {
		return seq + 1, nil
	}

	records, err := b.readLocked(sessionID)
	if err != nil {
		return 0, err
	}

	highest := 0
	for _, rec := range records {
		if rec.Seq > highest {
			highest = rec.Seq
		}
	}
	b.lastSeq[sessionID] = highest
	return highest + 1, nil
}

// readLocked parses the session's board file. A malformed or partially-written
// line is skipped with a warning rather than failing the whole read, and the
// file is never rewritten or removed: a torn append must not cost the operator
// the rest of the conversation.
func (b *Board) readLocked(sessionID uuid.UUID) ([]record, error) {
	path := b.resolver.SwarmBoardFile(sessionID)

	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("swarmboard: open board: %w", err)
	}
	defer f.Close()

	var (
		records []record
		scanner = bufio.NewScanner(f)
		lineNo  int
	)
	scanner.Buffer(make([]byte, 0, 64*1024), maxLineBytes)

	for scanner.Scan() {
		lineNo++
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var rec record
		if err := json.Unmarshal(line, &rec); err != nil {
			b.logger.Warn("swarmboard: skipping malformed board line",
				"session_id", sessionID.String(),
				"line", lineNo,
				"error", err,
			)
			continue
		}
		if rec.SchemaVersion != currentSchemaVersion {
			b.logger.Warn("swarmboard: skipping board line with unsupported schema version",
				"session_id", sessionID.String(),
				"line", lineNo,
				"schema_version", rec.SchemaVersion,
			)
			continue
		}
		records = append(records, rec)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("swarmboard: scan board: %w", err)
	}

	return records, nil
}
