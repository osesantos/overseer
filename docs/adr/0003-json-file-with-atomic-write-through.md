# ADR 0003: JSON File with Atomic Write-Through

## Status

Accepted

## Context

Overseer needs persistent storage for sessions. The simplest approach that supports the bootstrap's scope is a single JSON file. More complex solutions (SQLite, embedded databases) add dependencies and complexity without benefit at this scale. The storage adapter must be swappable in the future via the hexagonal port interface.

## Decision

Use a single JSON file with atomic write-through on every mutation:

- File location: `$XDG_DATA_HOME/overseer/data.json` (via `internal/shared/paths.DataFile()`)
- Schema: `{"schemaVersion": 1, "sessions": [...]}`
- Every mutation: update in-memory map → write entire file atomically via `internal/shared/paths.AtomicWrite` (tmp + rename)
- On startup: if file is corrupted (invalid JSON), rename to `data.corrupted.<unix-ts>.json` and start fresh
- `SchemaVersion` field is present for future migration tooling (no migration code in bootstrap)
- Last-writer-wins on concurrent processes — documented assumption, not enforced with file locking

## Consequences

- Simple, debuggable persistence (human-readable JSON)
- Atomic writes prevent partial-write corruption
- Corruption recovery is automatic and non-destructive (corrupted file preserved)
- Future storage backends (SQLite, remote) can be added by implementing `session.Repository`
- Multi-instance coordination is explicitly out of scope; last-writer-wins is documented
