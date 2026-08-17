# Auction Watch and Atomic Sessions Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [x]`) syntax for tracking.

**Goal:** Add safe session reload, `auction update`, live `auction watch`, complete squad rendering, atomic session saves, focused tests, and documentation.

**Architecture:** `internal/commands` owns active-session replacement and watch orchestration. Watch resolves a selector once, records a small `(modification time, size)` signature, and reloads/redraws only after a changed file; failed reloads leave the active session and prior signature untouched. `internal/storage` writes JSON to a temporary file in the target directory, syncs and closes it, then renames it over the destination. `internal/auction` owns the all-squad presentation using existing role ordering/slot helpers.

**Tech Stack:** Go 1.24, Cobra, standard library (`os`, `signal`, `syscall`, `time`, `encoding/json`, `testing`).

**Spec:** User-approved design in the task request.

## Global Constraints

- Preserve existing one-shot commands and REPL behavior.
- `auction watch` defaults to a two-second interval and resets that flag after each REPL command.
- Watch must resolve/load selectors before entering its polling loop, print an initial display, retry transient polling/reload errors, and exit on interrupt/SIGTERM.
- Failed reloads must not replace `currentSession`.
- `storage.SaveSession` must be atomic and clean up temporary files on failure.
- Do not test the infinite watch loop directly; test selection, signatures/change detection, reload, and storage behavior.
- The checkout is not Git-backed; use test/build checkpoints instead of worktrees or commits.

---

### Task 1: Add focused red tests for storage atomicity and command helpers

**Files:**
- Modify: `internal/storage/sessions_test.go`
- Create: `internal/commands/auction_test.go`

**Interfaces:**
- Tests will define the intended `fileSignature`, signature comparison, watch-session selection, and safe reload behavior in the `commands` package.
- Storage tests will exercise the existing `SaveSession` API against temporary directories and verify replacement content plus absence of temporary leftovers.

- [x] **Step 1: Add storage tests**

Add tests that save an initial session, save replacement content to the same path, load it back, and assert the new content is complete. Also assert the target directory contains no temporary files matching the save prefix after success. Add a failure case using a destination whose parent is a regular file and assert `SaveSession` returns an error without leaving a temp file in the usable target directory.

- [x] **Step 2: Add command helper tests**

Add tests for:

```go
func TestReloadSessionReplacesOnlyAfterSuccessfulLoad(t *testing.T)
func TestReloadSessionKeepsActiveSessionOnParseFailure(t *testing.T)
func TestSelectWatchSessionUsesSelectorOrActiveSession(t *testing.T)
func TestFileSignatureChangesWhenFileChanges(t *testing.T)
```

Use `t.TempDir`, real JSON session files, and the package-global `currentSession`; restore the global with `t.Cleanup`. The selector test must cover successful ID/path selection, unknown selector failure, and no-selector active-session requirement.

- [x] **Step 3: Run the focused tests and confirm expected RED failures**

Run:

```sh
go test ./internal/storage ./internal/commands
```

Expected: storage tests may expose the current non-atomic behavior, and command tests fail to compile or fail because the new helper symbols do not exist. Do not add production code before observing these failures.

---

### Task 2: Implement atomic session saving

**Files:**
- Modify: `internal/storage/sessions.go`
- Test: `internal/storage/sessions_test.go`

**Interfaces:**
- Preserve `func SaveSession(session *model.Session) error`.
- The implementation uses `filepath.Dir(session.Filepath)` and `os.CreateTemp` in that directory; it does not add dependencies or change JSON fields.

- [x] **Step 1: Replace direct `os.WriteFile` with atomic write flow**

Marshal the session, create the target directory, create a temporary file beside the destination, set mode `0644`, write all bytes, call `Sync`, close the file, and rename the temporary path over `session.Filepath`. Defer cleanup of the temporary path until rename succeeds. Return wrapped errors for creation, writing, syncing, closing, and renaming.

- [x] **Step 2: Run storage tests**

Run:

```sh
go test ./internal/storage
```

Expected: PASS, including replacement-content and temporary-file cleanup tests.

- [x] **Step 3: Run the full existing suite**

Run:

```sh
go test ./...
```

Expected: PASS.

---

### Task 3: Implement safe reload, update, selection, signatures, and watch polling

**Files:**
- Modify: `internal/commands/auction.go`
- Modify: `internal/commands` command tests from Task 1

**Interfaces:**
- Add unexported helpers with these signatures:

```go
type fileSignature struct {
    modTime time.Time
    size    int64
}

func reloadSession(path string) error
func selectWatchSession(selector string) (*model.Session, error)
func sessionFileSignature(path string) (fileSignature, error)
func watchSession(ctx context.Context, path string, interval time.Duration, out, errOut io.Writer) error
```

- Add `auction update` and `auction watch [path|ID|auctionName] --interval` to the existing `SessionCmd`.
- `watchSession` is the only loop; tests cover its pure/helper boundaries rather than running the infinite loop.

- [x] **Step 1: Implement `reloadSession` and selector helper**

`reloadSession` must call `storage.LoadSession(path)` and assign `currentSession` only after no error. `selectWatchSession` must resolve a non-empty selector with `storage.ResolveSession`, reload the resolved file path through `reloadSession`, and return the active session; with an empty selector it must return the existing active session or an error. This ensures selector errors happen before polling.

- [x] **Step 2: Convert load/update paths to safe reload behavior**

Keep existing user-facing behavior, but make the load handler return errors where practical and use `reloadSession` after resolving a selector. Add `auction update` as a `RunE` command that requires an active session, reloads `currentSession.Filepath`, and reports success. A failed update must preserve the prior pointer/content.

- [x] **Step 3: Add watch duration flag and REPL reset**

Declare `watchInterval time.Duration`, bind it with `DurationVar` using `2*time.Second`, and add `watchInterval = 2*time.Second` to `ResetFlags`. Validate non-positive intervals before entering the loop.

- [x] **Step 4: Implement signature and polling behavior**

`sessionFileSignature` returns modification time and size from `os.Stat`; equality is ordinary struct equality. `watchSession` prints the initial squads before polling, obtains the initial signature when possible, then uses a ticker. On each tick it retries stat errors, treats a changed signature as a reload attempt, keeps the prior active session/signature on reload failure, and on success clears/redraws the complete screen and updates the signature. Use `signal.NotifyContext` for `os.Interrupt` and `syscall.SIGTERM`, and return nil on context cancellation. Report transient errors to `errOut` and continue.

- [x] **Step 5: Run command tests and then the full suite**

Run:

```sh
go test ./internal/commands -run 'Test(Reload|SelectWatch|FileSignature)'
go test ./...
```

Expected: PASS.

---

### Task 4: Add complete squad rendering

**Files:**
- Modify: `internal/auction/utils.go`
- Modify: `internal/commands/auction.go`
- Modify: `internal/auction/auction_test.go` if a focused rendering assertion is useful

**Interfaces:**
- Add `func PrintAllSquads(sess *model.Session)` in `internal/auction`.
- It prints auction metadata, then every coach's budget, max bid, spending, roster size, role slot summary, and players grouped in goalkeeper/defender/midfielder/forward order.

- [x] **Step 1: Add a focused output test if needed**

Capture stdout only if the existing package conventions make it straightforward; assert representative metadata, coach stats, slot summary, and role-grouped players. Keep the test focused on observable output, not formatting whitespace.

- [x] **Step 2: Implement the smallest renderer**

Use existing `roleOrder`, `SquadByRole`, `RoleSlotsSummary`, `costants.SquadSize`, and `roleShort`/role names. Print empty role groups too so every role is visible. Avoid a new renderer type or dependency.

- [x] **Step 3: Wire watch initial/redraw output through `auction.PrintAllSquads`**

Ensure the initial display is always printed and redraw happens only after a successful changed-file reload.

- [x] **Step 4: Run auction and command tests**

Run:

```sh
go test ./internal/auction ./internal/commands
```

Expected: PASS.

---

### Task 5: Update README and codebase guide

**Files:**
- Modify: `README.md`
- Modify: `docs/CODEBASE.md`

**Interfaces:**
- Documentation describes `auction update`, `auction watch`, the two-second `--interval` default, selector forms, and the two-terminal workflow.

- [x] **Step 1: Update command examples and behavior notes**

Add the new commands alongside session commands, explain that watch reloads the exact session file and redraws on changes, and document the duration flag.

- [x] **Step 2: Add the two-terminal workflow**

Show terminal 1 running `auction watch <ID-or-path>` and terminal 2 using the REPL/one-shot commands against the same session file, noting that mutations save atomically and the watcher retries transient reads.

- [x] **Step 3: Run final verification**

Run:

```sh
cd /home/ludovg/projects/lazy-fanta
go test ./...
go build ./cmd/lazy-fanta
```

Expected: both commands succeed with no test failures or build errors. Report any platform-specific limitation, such as terminal escape sequences in non-interactive output.
