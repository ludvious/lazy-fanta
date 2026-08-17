# Auction Watch tmux Split Starvation Fix Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix `auction watch --tmux` failing after 4 panes on an 80x24 terminal. With `ludo-5300` (7 coaches) the current code dies at coach #5 ("mattia") with `tmux split-window for "mattia" failed: exit status 1` after tmux reports `size or position no space for a new pane`.

**Root cause (verified empirically):** each `split-window -t <session>:0` splits the *active* pane, which is the pane just created. Without re-layout between splits, panes shrink geometrically (80x24 → 80x12 → 80x5 → 80x2 → 80x2), tmux refuses the 5th split, the loop aborts, and the after-loop `select-layout tiled` never runs. The half-built tmux session persists, so re-running the command attaches to the broken 4-pane session instead of rebuilding it.

**Evidence (lab tests on tmux 3.7b, 80x24 window):**

| approach | result |
|---|---|
| current code (tiled only at end) | fails at pane 5 |
| `split-window -f`, tiled at end | fails at pane 8 |
| tiled after **each** split | 13 panes OK |
| `-f` + tiled after each split | 21 panes OK |

**Chosen fix (Option B):** move `select-layout tiled` inside the split loop, after each `split-window`. Re-tiling after each split redistributes equal space to every pane, so the active pane always has room for the next split. Same tmux invocation count as the broken path; no new flags or dependencies.

**Tech Stack:** Go 1.24, standard library `os/exec`; tmux 3.7b at runtime.

## Global Constraints

- Preserve the existing `has-session` reuse path: an existing `lazy-fanta-<session-ID>` session is attached, never rebuilt.
- Keep `new-session` and `attach-session` calls unchanged.
- Do not add `-f` (Option A is insufficient; Option C is deferred until rosters exceed ~15 coaches).
- Do not change `tmuxSessionName`, `tmuxWatchArgs`, or the non-tmux watch path.
- This checkout has no Git metadata: use test/build verification instead of commits.

---

### Task 1: Update the fake-tmux test to expect per-split tiling (RED)

**Files:**
- Modify: `internal/commands/auction_test.go`

- [ ] **Step 1: Rewrite call-sequence expectations in `TestLaunchTmuxWatchCreatesOnePanePerCoach`**

The current test asserts exactly 6 calls: `has-session`, coach call 1, coach calls 2–3, one `select-layout`, `attach-session`. Change it to assert exactly 7 calls in this order:

1. `has-session -t lazy-fanta-ludo-5300`
2. `new-session -d -s lazy-fanta-ludo-5300 -n watch -- … --name John`
3. `split-window -t lazy-fanta-ludo-5300:0 -- … --name Jane`
4. `select-layout -t lazy-fanta-ludo-5300:0 tiled`
5. `split-window -t lazy-fanta-ludo-5300:0 -- … --name Luca`
6. `select-layout -t lazy-fanta-ludo-5300:0 tiled`
7. `attach-session -t lazy-fanta-ludo-5300`

General rule: one `select-layout` per coach after the first, interleaved immediately after each `split-window`. Keep the existing assertions for child command contents, no `--tmux` in children, and exactly `len(sess.Coaches)` pane-creation calls.

- [ ] **Step 2: Run the focused test and confirm RED**

```sh
gofmt -w internal/commands/auction_test.go
go test ./internal/commands -run TestLaunchTmuxWatchCreatesOnePanePerCoach
```

Expected: failure because the current implementation emits one `select-layout` only after the loop, not after each split.

---

### Task 2: Apply per-split tiled layout in the launcher (GREEN)

**Files:**
- Modify: `internal/commands/auction.go`

- [ ] **Step 1: Tile inside the loop**

In `launchTmuxWatch`, inside the `for _, coach := range sess.Coaches[1:]` loop, after the successful `split-window` call, run:

```go
if err := runTmux("select-layout", "-t", name+":0", "tiled"); err != nil {
    return fmt.Errorf("tmux select-layout failed: %w", err)
}
```

- [ ] **Step 2: Delete the after-loop tiled block**

Remove the existing `if len(sess.Coaches) > 1 { … select-layout … }` block that runs after the loop.

- [ ] **Step 3: Run focused tests to verify GREEN**

```sh
gofmt -w internal/commands/auction.go internal/commands/auction_test.go
go test ./internal/commands
```

Expected: all command tests pass, including the reuse, empty-roster, and missing-tmux tests unchanged.

---

### Task 3: Verify the complete fix

- [ ] **Step 1: Full suite, race, vet, build**

```sh
go test ./...
go test -race ./...
go vet ./...
go build -o lazy-fanta ./cmd/lazy-fanta
```

- [ ] **Step 2: Real tmux smoke test**

```sh
tmux kill-session -t lazy-fanta-ludo-5300 2>/dev/null
./lazy-fanta auction watch ludo-5300 --tmux
```

From another shell while attached:

```sh
tmux list-panes -t lazy-fanta-ludo-5300 -F "#{pane_width}x#{pane_height} #{pane_current_command}"
```

Expected: 7 panes in a tiled grid (no `no space for a new pane` errors, no 2-row slivers), one coach per pane. Detach with `Ctrl-b d`, re-run the command, and confirm it attaches to the existing session without creating new panes.

- [ ] **Step 3: Cleanup**

```sh
tmux kill-session -t lazy-fanta-ludo-5300
```

Also check for the leftover `lazy-fanta-test2-8e09` session (11 windows, from earlier testing) and kill it if no longer needed.

---

## Deferred (do not implement now)

- **Option C (`-f` on split-window):** belt-and-braces for rosters > ~15 coaches. Add only if per-split tiling ever starves on very large rosters.
- No Git commit step: this checkout has no Git metadata; leave verified source changes in the workspace.
