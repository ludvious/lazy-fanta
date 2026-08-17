# Auction Watch tmux Mode Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an optional `--tmux` mode to `auction watch` that opens one tmux window per coach, runs that coach's filtered watch in each window, and attaches to the tmux session.

**Architecture:** Keep the existing single-terminal watch path unchanged. When `--tmux` is supplied, resolve the session once, derive a stable tmux session name from the resolved session ID, and use Go's standard `os/exec` to create or attach to tmux. Each window runs the current executable directly with `auction watch <selector> --name <coach>`; no shell string interpolation or new dependency is needed.

**Tech Stack:** Go 1.24, Cobra, standard library `os/exec`, `os`, `io`, `time`, and the existing `testing` package; tmux must be available on `PATH` at runtime.

**Spec:** Approved design in the conversation; no separate spec file.

## Global Constraints

- Preserve normal `auction watch [path|ID|auctionName]` behavior when `--tmux` is omitted.
- `--tmux` is an optional boolean flag on `auction watch` and works from the shell without entering the REPL.
- Resolve the auction selector before creating or attaching to tmux.
- Use the stable tmux session name `lazy-fanta-<resolved-session-ID>`.
- Attach automatically after creating a new tmux session or when reusing an existing one.
- Reusing an existing tmux session must not create duplicate windows or watchers.
- A new tmux session has exactly one window per coach at launch time; it does not dynamically add or remove windows later.
- Each child watcher uses `--name <coach>` and must not receive `--tmux`.
- Pass a non-default parent `--interval` value through to every child watcher.
- Reject `--tmux --name <coach>` because tmux mode already creates one window for every coach.
- Return a clear error when tmux is unavailable, the resolved session has no coaches, or the watch interval is invalid.
- Pass executable arguments directly to `exec.Command`; do not use `sh -c` or manually concatenate shell commands.
- Reset the tmux flag between REPL commands.
- Do not add a dependency, tmux panes, dynamic roster synchronization, or unrelated refactors.
- This checkout has no Git metadata, so use test/build verification instead of commits.

---

### Task 1: Add failing tmux launcher tests and flag reset coverage

**Files:**
- Modify: `internal/commands/auction_test.go`

**Interfaces:**
- The tests define the planned helpers `tmuxSessionName`, `tmuxWatchArgs`, and `launchTmuxWatch`.
- Tests use real `model.Session` values and a temporary fake `tmux` executable that records its arguments; no production mocks or new dependencies are needed.

- [ ] **Step 1: Add the tmux flag registration assertion**

Add a test that reads `watchSessionCmd.Flags().Lookup("tmux")` and fails if the flag is missing or is not a boolean flag. Keep the existing command registration test unchanged otherwise.

- [ ] **Step 2: Add the stable session-name test**

Add:

```go
func TestTmuxSessionNameUsesSessionID(t *testing.T) {
	sess := &model.Session{ID: "ludo-5300"}
	if got := tmuxSessionName(sess); got != "lazy-fanta-ludo-5300" {
		t.Fatalf("tmux session name = %q, want %q", got, "lazy-fanta-ludo-5300")
	}
}
```

- [ ] **Step 3: Add the child command argument test**

Add a test for:

```go
func tmuxWatchArgs(executable, selector, coachName string, interval time.Duration) []string
```

For `/tmp/lazy-fanta`, selector `data/session/ludo-5300.json`, coach `John`, and `500*time.Millisecond`, assert the result is exactly:

```go
[]string{
	"/tmp/lazy-fanta", "auction", "watch", "data/session/ludo-5300.json",
	"--name", "John", "--interval", "500ms",
}
```

Also assert that a `2*time.Second` interval does not add an unnecessary `--interval` argument, since that is the child watcher's default.

- [ ] **Step 4: Add the fake-tmux launcher test for one window per coach**

Create a temporary executable named `tmux` with this exact shell behavior:

```sh
#!/bin/sh
printf '%s\n' "$*" >> "$TMUX_LOG"
if [ "$1" = "has-session" ] && [ "${TMUX_EXISTS:-0}" = "1" ]; then
    exit 0
fi
if [ "$1" = "has-session" ]; then
    exit 1
fi
exit 0
```

Make it executable, prepend its directory to `PATH`, and set `TMUX_LOG` to a temporary file. Call `launchTmuxWatch` with a session whose ID is `ludo-5300`, filepath is `data/session/ludo-5300.json`, and coaches are `John`, `Jane`, and `Luca`.

Assert that the recorded calls contain, in order:

1. `has-session` for `lazy-fanta-ludo-5300`.
2. One `new-session` call naming `John` and running `auction watch data/session/ludo-5300.json --name John`.
3. Two `new-window` calls naming `Jane` and `Luca`, each running the same selector with its own `--name`.
4. One `attach-session` call for `lazy-fanta-ludo-5300`.

The test should assert there are exactly three coach window creation calls and that no recorded child command contains `--tmux`.

- [ ] **Step 5: Add the existing-session reuse test**

Set `TMUX_EXISTS=1` for the same fake executable and call `launchTmuxWatch`. Assert that the log contains `has-session` followed only by `attach-session`; it must not contain `new-session` or `new-window`.

- [ ] **Step 6: Add empty-roster and missing-tmux error tests**

Add one test that passes a resolved session with zero coaches and asserts `launchTmuxWatch` returns an error without creating a tmux session.

Add one test that temporarily sets `PATH` to a directory without `tmux` and asserts the returned error identifies the missing tmux executable. Restore the test environment with `t.Setenv` so the test cannot affect other tests.

- [ ] **Step 7: Extend REPL reset coverage**

Set `watchTmux = true` in `TestResetFlagsRestoresREPLDefaults`, call `ResetFlags`, and assert:

```go
if watchTmux {
	t.Fatal("tmux flag was not reset")
}
```

- [ ] **Step 8: Run the focused tests and confirm RED**

Run:

```sh
gofmt -w internal/commands/auction_test.go
go test ./internal/commands
```

Expected: compilation failures because `watchTmux`, `tmuxSessionName`, `tmuxWatchArgs`, and `launchTmuxWatch` do not exist yet, plus the new flag is not registered. Confirm the failures are caused by the missing feature rather than test syntax or fake-executable setup errors.

---

### Task 2: Implement tmux watch orchestration and the `--tmux` flag

**Files:**
- Modify: `internal/commands/auction.go`
- Test: `internal/commands/auction_test.go`

**Interfaces:**
- Add the package variable:

```go
var watchTmux bool
```

- Add these helpers:

```go
func tmuxSessionName(sess *model.Session) string
func tmuxWatchArgs(executable, selector, coachName string, interval time.Duration) []string
func launchTmuxWatch(sess *model.Session, selector string, interval time.Duration) error
```

- Keep `printWatchSnapshot` and `watchSession` unchanged for non-tmux watching.

- [ ] **Step 1: Add the watch-specific flag state and registration**

Add `watchTmux bool` beside `watchInterval` and `watchCoachName`. In `init`, bind:

```go
watchSessionCmd.Flags().BoolVar(&watchTmux, "tmux", false, "Open one watch window per coach in tmux")
```

Add `watchTmux = false` to `ResetFlags`.

- [ ] **Step 2: Implement stable tmux naming and child arguments**

Implement:

```go
func tmuxSessionName(sess *model.Session) string {
	return "lazy-fanta-" + sess.ID
}
```

Implement `tmuxWatchArgs` using direct executable arguments. Always include:

```go
[]string{executable, "auction", "watch", selector, "--name", coachName}
```

Append `"--interval", interval.String()` only when `interval != 2*time.Second`. This preserves the default without adding redundant child flags and propagates explicit non-default intervals.

- [ ] **Step 3: Implement runtime validation and selector fallback**

At the start of `launchTmuxWatch`:

1. Reject `interval <= 0` with the same `watch interval must be greater than zero` wording used by `watchSession`.
2. Reject an empty coach roster with a descriptive error such as `cannot launch tmux watch: session has no coaches`.
3. Resolve `tmux` with `exec.LookPath("tmux")`; return a descriptive error if unavailable.
4. Resolve the current executable with `os.Executable()`; return its error if lookup fails.
5. Use the incoming selector for child processes. If it is empty (the active session was selected inside the REPL), fall back to `sess.Filepath`, because child processes are separate processes and do not inherit `currentSession`.

- [ ] **Step 4: Reuse an existing tmux session**

Run `tmux has-session -t <session-name>` with stdout and stderr discarded. If it exits successfully, run `tmux attach-session -t <session-name>` with the parent process's stdin, stdout, and stderr attached, then return its error.

Do not create or reconcile windows when the session already exists; this is the approved duplicate-prevention behavior.

- [ ] **Step 5: Create the first tmux window**

Build the first child command from the first coach and run:

```text
tmux new-session -d -s <session-name> -n <coach-name> -- <executable> auction watch <selector> --name <coach-name> [--interval <duration>]
```

Use `exec.Command` with separate arguments. Do not construct a shell command string. Inherit the parent standard streams for tmux commands that may report an error.

- [ ] **Step 6: Create the remaining coach windows**

For each coach after the first, run:

```text
tmux new-window -t <session-name> -n <coach-name> -- <executable> auction watch <selector> --name <coach-name> [--interval <duration>]
```

Use the exact coach name for the window title and filter. Stop and return a descriptive error if any window creation fails.

- [ ] **Step 7: Attach after creation**

After all windows are created, run:

```text
tmux attach-session -t <session-name>
```

Attach stdin/stdout/stderr so the user sees the new tmux session directly from the invoking shell. Return the attach command's exit error.

- [ ] **Step 8: Route the Cobra handler into tmux mode**

In `watchSessionCmd.RunE`, keep selector resolution through `selectWatchSession`. After it succeeds:

```go
if watchTmux {
	if watchCoachName != "" {
		return fmt.Errorf("--tmux cannot be combined with --name")
	}
	return launchTmuxWatch(sess, selector, watchInterval)
}
```

Leave the existing signal context and `watchSession` call as the non-tmux path. Do not pass `watchTmux` to child watchers; they run the normal filtered watch path.

- [ ] **Step 9: Run focused tests to verify GREEN**

Run:

```sh
gofmt -w internal/commands/auction.go internal/commands/auction_test.go
go test ./internal/commands
```

Expected: all command tests pass, including fake-tmux creation, existing-session reuse, errors, snapshot behavior, and REPL reset behavior.

---

### Task 3: Document the shell command and tmux lifecycle

**Files:**
- Modify: `README.md`

**Interfaces:**
- Documentation must describe the new shell-only convenience mode without changing existing command syntax.

- [ ] **Step 1: Add the command example**

Under the session command examples, add:

```sh
auction watch <name|ID|path> --tmux       # open one attached tmux window per coach
```

Add a short example:

```sh
./lazy-fanta auction watch ludo --tmux
```

State that the tmux session is named `lazy-fanta-<session-ID>`, windows are titled with coach names, and an existing session is reused instead of duplicated.

- [ ] **Step 2: Document interval forwarding and detach behavior**

Add:

```sh
./lazy-fanta auction watch ludo --tmux --interval 500ms
```

Explain that the interval is forwarded to each coach watcher. Document `Ctrl-b d` to detach and `tmux attach -t lazy-fanta-<session-ID>` to return later. State that window creation reflects the coaches present when the command starts; roster changes do not create or remove tmux windows automatically.

- [ ] **Step 3: Check the documentation diff by reading it**

Read the edited session/watch sections and confirm the examples use the direct shell executable, not REPL syntax, and do not imply that `--tmux` can be combined with `--name`.

---

### Task 4: Verify the complete feature

**Files:**
- No additional files.

- [ ] **Step 1: Run the full test suite**

```sh
go test ./...
```

Expected: all packages pass.

- [ ] **Step 2: Run race tests**

```sh
go test -race ./...
```

Expected: all packages pass without race reports.

- [ ] **Step 3: Run vet and build**

```sh
go vet ./...
go build -o lazy-fanta ./cmd/lazy-fanta
```

Expected: both commands complete successfully.

- [ ] **Step 4: Perform a real tmux smoke test**

Use an existing session with at least two coaches:

```sh
./lazy-fanta auction watch ludo-5300 --tmux --interval 500ms
```

After attachment, verify the windows from another shell:

```sh
tmux list-windows -t lazy-fanta-ludo-5300
```

Confirm there is one window per coach, each window title matches its coach, and each dashboard contains only that coach's squad. Press `Ctrl-b d` to detach, then re-run the command and confirm it attaches to the existing session without adding windows.

- [ ] **Step 5: Verify normal and error behavior**

Confirm normal watch is unchanged:

```sh
./lazy-fanta auction watch ludo-5300 --name John --interval 500ms
```

In a separate shell, verify the invalid combination fails before tmux creation:

```sh
./lazy-fanta auction watch ludo-5300 --tmux --name John
```

Expected: a clear `--tmux cannot be combined with --name` error. Also verify a selector for a session with no coaches returns an error rather than creating an empty tmux session.

- [ ] **Step 6: Clean up the manual tmux session**

After testing, remove the session so it does not affect later runs:

```sh
tmux kill-session -t lazy-fanta-ludo-5300
```

No Git commit step is included because this checkout has no Git metadata; leave the verified source and documentation changes in the workspace.
