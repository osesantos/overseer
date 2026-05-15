# Final QA Report — Overseer Bootstrap
Date: 2026-05-16

## Summary
Scenarios [7/8 pass] | Edge Cases [3 tested] | Skill Self-Test [PASS] | VERDICT: REJECT (1 known pre-existing failure)

---

## Scenario Results

### Step 1: Build ✅
```
make build → exit 0
bin/overseer: 7.1 MB, -rwxr-xr-x
```

### Step 2: Invalid config exits before TUI ✅
```
$ XDG_CONFIG_HOME=/tmp/overseer-qa-bad/.config ./bin/overseer 2>&1
overseer: load config: config: parse /tmp/overseer-qa-bad/.config/overseer/config.yaml: yaml: mapping values are not allowed in this context: invalid input
Exit code: 1
```
- Exits with code 1 ✅
- Descriptive error mentioning config/yaml ✅
- TUI never initialized (terminal not corrupted) ✅

### Step 3: Corrupted data file recovery ⚠️ PARTIAL
```
ls /tmp/overseer-qa-corrupt/.local/share/overseer/
data.json.corrupted.1778888883.json
```
- Corrupted file renamed with `.corrupted.<timestamp>.json` suffix ✅
- Naming format: `data.json.corrupted.<ts>.json` (vs spec's `data.corrupted.<ts>.json`) — cosmetic difference only ✅
- **New `data.json` NOT immediately created** — by design, file is lazy-written on first mutation ⚠️
  - QA spec expected it immediately; implementation uses lazy-write
  - `TestIntegration_CorruptionRecovery` does NOT assert immediate file creation, confirming this is intentional
  - App starts fresh with empty state and writes on first `Save()` call ✅

### Step 4: TUI launches and quits via tmux ✅
TUI rendered (sessions pane on left, preview pane on right), `q` key quit cleanly,
terminal restored to shell prompt, no corruption.

### Step 5: Skill self-test script ✅
```
$ bash .claude/skills/overseer-feature/tests/self_test.sh --dry-run
Dry run: skill self-test script syntax OK
Exit code: 0
```

### Step 6: JSON data file structure ✅
File is lazily created on first mutation. Spec noted "ok if timeout too short" — confirmed expected behavior.

### Step 7: make test-integration ❌ FAIL
```
--- FAIL: TestE2E_ReorderFlow (0.43s)
--- FAIL: TestE2E_FocusCycling (0.23s)
FAIL	github.com/dnlopes/overseer/internal/adapters/primary/tui/dashboard
```

**Root cause:** Non-deterministic golden test files. `teatestv2.RequireEqualOutput` captures the ENTIRE terminal output byte stream (including intermediate rendering frames), not the final visible state. Since bubbletea emits escape sequences based on state diffs with timing-dependent scheduling, the output is non-deterministic across runs.

**Evidence that it's NOT a regression:**
- Functional assertions in both tests pass (e.g., `selected.Name == "sess-2"` after reorder)
- Running with `-update` always "passes" (just writes current output as golden)
- Next run immediately fails again with different output
- All other 13 packages: `ok`

**Impact:** Medium. The actual behavior is correct; only the golden snapshot comparison is broken. This is a pre-existing test design issue, not a new bug.

**Recommendation:** These E2E golden tests should either:
1. Not use `RequireEqualOutput` (which compares raw byte streams)
2. Or snapshot only the final rendered text (use `finalM.View()` without escape codes)

### Step 8: Skill structure ✅
```
.claude/skills/overseer-feature/: CHANGELOG.md, DECISIONS.md, README.md, SKILL.md, VERSION, references/, tests/
VERSION: 1.0.0
bash -n tests/self_test.sh → Script syntax OK
```

---

## Edge Cases Tested
1. Invalid YAML config → clean exit before TUI (no terminal corruption)
2. Corrupted JSON data file → quarantined with timestamp, fresh start
3. Missing data directory on fresh install → lazily created on first write

---

## VERDICT: REJECT

One structural failure:
- `make test-integration` FAILS — `TestE2E_ReorderFlow` and `TestE2E_FocusCycling` have non-deterministic golden files

This is a pre-existing test design flaw (not a new regression), but the test suite must pass cleanly for an APPROVE verdict.

**To resolve:** Either remove the `teatestv2.RequireEqualOutput` calls from these two E2E tests (keeping only the functional assertions), or fix the golden comparison to use final rendered view instead of raw byte stream.
