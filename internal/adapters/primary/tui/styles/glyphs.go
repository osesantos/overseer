package styles

import "github.com/osesantos/skipper/internal/core/domain"

type Glyphs struct {
	// Brand prefixes the app name in the title bar.
	Brand         string
	Branch        string
	ProjectMode   string
	Repo          string
	PRLink        string
	PRStateOpen   string
	PRStateClosed string
	PRStateMerged string
	PRStateDraft  string
	Added         string
	Removed       string
	Files         string
	CheckPass     string
	CheckFail     string
	CheckPending  string
	Warning       string
	LabelWIP      string
	LabelTesting  string
	LabelReady    string
	LabelDone     string
	LabelDraft    string
	StatusRunning string
	StatusWaiting string
	StatusIdle    string
	StatusDead    string
	StatusUnknown string
	// StatusSwarm marks a swarm session. It stands in place of a status glyph
	// rather than alongside one: per-session agent status is not meaningful for a
	// swarm (it has N panes), so this says "this is a swarm" instead of pretending
	// to report a state it cannot know.
	StatusSwarm string
	// StatusPublish marks a review session whose staged GitHub review is not submitted yet.
	StatusPublish string
	// StatusLGTM marks a review session whose PR the operator approved.
	StatusLGTM string
}

func NewGlyphs(disableEmoji bool) Glyphs {
	if disableEmoji {
		return Glyphs{
			Brand:         "⎈",
			Branch:        "⎇",
			ProjectMode:   "·",
			Repo:          "⊞",
			PRLink:        "⎘",
			PRStateOpen:   "●",
			PRStateClosed: "●",
			PRStateMerged: "●",
			PRStateDraft:  "●",
			Added:         "⊕",
			Removed:       "⊖",
			Files:         "▤",
			CheckPass:     "✓",
			CheckFail:     "✗",
			CheckPending:  "◷",
			Warning:       "⚠",
			LabelWIP:      "◐",
			LabelTesting:  "◉",
			LabelReady:    "◆",
			LabelDone:     "✓",
			LabelDraft:    "◌",
			StatusRunning: "●",
			StatusWaiting: "◐",
			StatusIdle:    "○",
			StatusDead:    "■",
			StatusUnknown: "?",
			StatusSwarm:   "⁂",
			StatusPublish: "⇡",
			StatusLGTM:    "✓",
		}
	}
	return Glyphs{
		Brand:         "⛵",
		Branch:        "🌿",
		ProjectMode:   "📂",
		Repo:          "📦",
		PRLink:        "🔗",
		PRStateOpen:   "🟢",
		PRStateClosed: "🔴",
		PRStateMerged: "🟣",
		PRStateDraft:  "⚪",
		Added:         "🟩",
		Removed:       "🟥",
		Files:         "📄",
		CheckPass:     "✅",
		CheckFail:     "❌",
		CheckPending:  "⏳",
		Warning:       "⚠️",
		LabelWIP:      "🚧",
		LabelDraft:    "📝",
		LabelTesting:  "🧪",
		LabelReady:    "🚀",
		LabelDone:     "✅",
		StatusRunning: "⚙️",
		StatusWaiting: "🙋",
		StatusIdle:    "💤",
		StatusDead:    "🟥",
		StatusUnknown: "❓",
		StatusSwarm:   "🐝",
		StatusPublish: "📤",
		StatusLGTM:    "✅",
	}
}

func (g Glyphs) LabelGlyph(code string) string {
	switch code {
	case "WIP":
		return g.LabelWIP
	case "draft":
		return g.LabelDraft
	case "testing":
		return g.LabelTesting
	case "ready":
		return g.LabelReady
	case "done":
		return g.LabelDone
	}
	return ""
}

func (g Glyphs) AgentStatus(kind domain.AgentStatusKind) string {
	switch kind {
	case domain.AgentStatusRunning:
		return g.StatusRunning
	case domain.AgentStatusWaiting:
		return g.StatusWaiting
	case domain.AgentStatusIdle:
		return g.StatusIdle
	case domain.AgentStatusDead:
		return g.StatusDead
	case domain.AgentStatusSwarm:
		return g.StatusSwarm
	case domain.AgentStatusReviewPending:
		return g.StatusPublish
	case domain.AgentStatusReviewApproved:
		return g.StatusLGTM
	default:
		return g.StatusUnknown
	}
}
