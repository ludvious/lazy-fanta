# Auction Watch Coach Filter Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an optional `--name <coachname>` filter to `auction watch` so the dashboard can show one coach's squad while preserving the existing full-session watch.

**Architecture:** Keep session selection, polling, atomic reload, screen clearing, and signal handling unchanged. Pass the optional coach name through the watch render path. Add one small auction presentation function that renders session metadata and a single matching coach, using the existing role/slot formatting. An omitted name continues to render the current complete dashboard.

**Tech Stack:** Go 1.24, Cobra, standard library, existing `testing` package. No new dependency.

**Spec:** Approved design in the conversation; no separate spec file because this is a bounded change.

## Global Constraints

- Preserve `auction watch [path|ID|auctionName]` without `--name`.
- `--name` is optional and filters by coach name when supplied.
- Coach matching is case-insensitive, consistent with existing auction operations.
- An unknown coach must fail before the polling loop starts.
- Preserve the existing interval, file-signature polling, atomic reload, ANSI clear/redraw, and SIGINT/SIGTERM behavior.
- Reset the watch-specific name between REPL commands.
- Do not refactor unrelated session, model, or command code.
- This checkout has no Git metadata, so use test/build verification instead of commits.

---

### Task 1: Add failing tests for filtered watch rendering and flag reset

**Files:**
- Modify: `internal/auction/auction_test.go`
- Modify: `internal/commands/auction_test.go`

**Interfaces:**
- The tests define the required `auction.PrintSquad` behavior and the command-level watch snapshot behavior.
- Tests use real sessions and stdout capture; no mocks are needed.

- [ ] **Step 1: Add the auction renderer test**

Create a session with two coaches, each with at least one player. Capture stdout while calling the planned function:

```go
func TestPrintSquadShowsOnlySelectedCoach(t *testing.T)
```

The test should call:

```go
if err := PrintSquad(sess, "john"); err != nil {
    t.Fatal(err)
}
```

Assert that the output contains the session metadata, John’s name, John’s slot summary, and John’s player. Assert that it does not contain the other coach’s name or player. Use a case-variant selector (`"john"`) to prove matching is case-insensitive.

- [ ] **Step 2: Add the unknown-coach renderer test**

Add:

```go
func TestPrintSquadRejectsUnknownCoach(t *testing.T)
```

Call `PrintSquad` with a missing name and assert that it returns an error. Do not require an exact error string beyond confirming an error is returned.

- [ ] **Step 3: Add command snapshot tests**

Add a command-package test for the planned snapshot signature:

```go
func TestPrintWatchSnapshotFiltersByCoach(t *testing.T)
```

Capture stdout and call the snapshot with `"John"`. Assert that output begins with `"\033[2J\033[H"`, includes John, and excludes another coach.

Update the existing full-watch snapshot test to pass an empty coach name and verify that full-session output still contains the session data.

- [ ] **Step 4: Add the watch flag reset assertion**

Extend `TestResetFlagsRestoresREPLDefaults` with the watch-specific variable, for example:

```go
watchCoachName = "John"
ResetFlags()
if watchCoachName != "" {
    t.Fatalf("watch coach name was not reset: %q", watchCoachName)
}
```

- [ ] **Step 5: Run the focused tests and confirm RED**

Run:

```sh
go test ./internal/auction ./internal/commands
```

Expected: compilation or test failures because `PrintSquad`, the filtered snapshot signature, and the watch-specific state do not exist yet. Confirm the failures are caused by the missing feature, not test mistakes.

---

### Task 2: Implement single-coach squad rendering

**Files:**
- Modify: `internal/auction/utils.go`
- Test: `internal/auction/auction_test.go`

**Interfaces:**
- Add:

```go
func PrintSquad(sess *model.Session, coachName string) error
```

- Preserve `PrintAllSquads(sess *model.Session)` and its existing output.

- [ ] **Step 1: Implement case-insensitive coach lookup**

In `PrintSquad`, find the first coach whose name matches `coachName` using `strings.EqualFold`. Return a descriptive error when no coach matches. Do not mutate the session or coach slice.

- [ ] **Step 2: Reuse the existing display shape for one coach**

Print the same session metadata used by `PrintAllSquads`, then print only the selected coach’s summary row, slots, and four role lines in the existing role order. Keep empty roles visible as `-`.

Use the existing `RoleSlotsSummary`, `roleOrder`, and role-grouping logic. Do not add a renderer type, table dependency, or alternate output format.

- [ ] **Step 3: Run the auction tests**

Run:

```sh
go test ./internal/auction
```

Expected: PASS, including the new selected-coach and unknown-coach tests plus all existing domain/rendering tests.

---

### Task 3: Add `auction watch --name` and route filtered snapshots

**Files:**
- Modify: `internal/commands/auction.go`
- Test: `internal/commands/auction_test.go`

**Interfaces:**
- Add a watch-specific package variable:

```go
var watchCoachName string
```

- Bind the watch command’s `--name` flag to that variable.
- Change the internal render boundary to carry the filter:

```go
func printWatchSnapshot(sess *model.Session, coachName string) error
func watchSession(ctx context.Context, path string, interval time.Duration, coachName string) error
```

- [ ] **Step 1: Add the watch-specific flag**

In `init`, bind `--name` on `watchSessionCmd`:

```go
watchSessionCmd.Flags().StringVar(&watchCoachName, "name", "", "Show only this coach's squad")
```

Keep the existing `--interval` flag unchanged.

- [ ] **Step 2: Route full and filtered rendering**

Implement `printWatchSnapshot` as follows:

```go
func printWatchSnapshot(sess *model.Session, coachName string) error {
    fmt.Print("\033[2J\033[H")
    if coachName == "" {
        auction.PrintAllSquads(sess)
        return nil
    }
    return auction.PrintSquad(sess, coachName)
}
```

This keeps the existing full watch unchanged while making the filtered path explicit.

- [ ] **Step 3: Validate the coach before polling**

In `watchSession`, call `printWatchSnapshot` before creating the ticker. If it returns an error, return it immediately. This ensures an unknown `--name` fails before the infinite watch loop starts.

Keep the existing initial signature handling and polling logic unchanged. On successful reloads, call the same filtered snapshot function. If a later reload removes or renames the selected coach, report the render error to stderr and continue polling rather than panic.

- [ ] **Step 4: Pass the flag through the command handler**

Change the watch command’s call from:

```go
return watchSession(ctx, sess.Filepath, watchInterval)
```

to:

```go
return watchSession(ctx, sess.Filepath, watchInterval, watchCoachName)
```

Do not change session selector resolution.

- [ ] **Step 5: Reset the watch name for REPL use**

Add:

```go
watchCoachName = ""
```

to `ResetFlags`.

- [ ] **Step 6: Run command tests**

Run:

```sh
go test ./internal/commands
```

Expected: PASS, including registration, reset behavior, full snapshots, filtered snapshots, safe reload behavior, and existing watch helper tests.

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
go build ./cmd/lazy-fanta
```

Expected: both commands complete successfully.

- [ ] **Step 4: Perform a manual command smoke test**

Use an existing session containing at least two coaches:

```sh
./lazy-fanta auction watch <session-ID-or-path> --name John --interval 500ms
```

Confirm that the dashboard contains John’s squad but not the other coaches’ squads. Then verify that omitting `--name` still displays the complete session dashboard.

- [ ] **Step 5: Verify error behavior**

Run:

```sh
./lazy-fanta auction watch <session-ID-or-path> --name Missing
```

Expected: an unknown-coach error is returned without entering the polling loop.
