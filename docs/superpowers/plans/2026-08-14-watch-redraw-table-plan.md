# Interactive Auction Watch Redraw and Table Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `auction watch` behave like a live terminal dashboard by clearing the visible screen before each snapshot and rendering complete squads in a compact, readable table-style layout.

**Architecture:** Keep the existing polling, selector resolution, safe reload, signature tracking, and signal handling unchanged. Add one small watch-render helper in `internal/commands` that clears the terminal and prints the current session, then simplify `auction.PrintAllSquads` into aligned coach-summary columns followed by compact role-grouped player rows.

**Tech Stack:** Go 1.24, Cobra, standard-library terminal escape sequences, `fmt`, `strings`, and the existing `testing` package. No new dependency.

**Spec:** Approved design in the conversation: interactive terminal redraw plus readable watch table.

## Global Constraints

- Preserve all existing auction commands and their behavior.
- `auction watch` is intended for an interactive terminal and may use ANSI terminal control sequences.
- Clear and redraw on the initial display and after every successful detected session update.
- Do not append a new full snapshot when no file change is detected.
- Preserve complete squad information: auction metadata, coach budget, max bid, spending, roster size, role slots, and every player grouped by role.
- Keep transient stat/read/parse error retry behavior and clean Ctrl-C/SIGTERM shutdown unchanged.
- Do not test the infinite polling loop directly; test the render boundary and existing helper behavior.
- Use only focused tests and rerun `go test ./...` and `go build ./cmd/lazy-fanta`.
- Do not add TTY-detection libraries or a new rendering abstraction.

---

### Task 1: Add failing tests for the dashboard render contract

**Files:**
- Modify: `internal/auction/auction_test.go`
- Modify: `internal/commands/auction_test.go`

**Interfaces:**
- The auction test captures `PrintAllSquads` output and verifies the table labels, aligned summary values, role slots, role order, and player names.
- The commands test captures the watch snapshot output and verifies it begins with the clear-screen sequence before the latest session data.

- [ ] **Step 1: Replace the old per-coach formatting assertions with the table contract**

Keep the existing representative session, remove assertions that require the old `Budget: 400`, `Max bid: 376`, `Spent: 100`, and `Roster: 2/25` labels, and add assertions for the intended summary header and row values:

```go
for _, want := range []string{
    "Coach", "Budget", "Max bid", "Spent", "Roster",
    "John", "400", "376", "100", "2/25",
    "Slots:", "GK(1/3)", "DF(0/8)", "MD(0/8)", "FW(1/6)",
    "Goalkeeper:", "Keeper (1 M)",
    "Forward:", "Messi (100 M)",
} {
    if !strings.Contains(text, want) {
        t.Errorf("output missing %q:\n%s", want, text)
    }
}
```

Also verify that the role labels occur in goalkeeper, defender, midfielder, forward order. Use `strings.Index` and fail if an earlier role appears after a later role.

- [ ] **Step 2: Add a focused watch-snapshot test**

Add a test that calls the new package-local snapshot helper with a small session and captures stdout. The first bytes must be the clear-and-home sequence, followed by the auction name:

```go
const clearScreen = "\033[2J\033[H"
if !strings.HasPrefix(string(output), clearScreen) {
    t.Fatalf("watch output does not start with screen clear: %q", output)
}
if !strings.Contains(string(output), "Final") {
    t.Fatalf("watch output does not contain the rendered session: %s", output)
}
```

Restore `os.Stdout` with `t.Cleanup` or an equivalent cleanup path even when the assertion fails.

- [ ] **Step 3: Run the focused tests and confirm the new contract fails**

Run:

```sh
cd /home/ludovg/projects/lazy-fanta
go test ./internal/auction ./internal/commands
```

Expected: the existing renderer test or the new snapshot test fails because the table labels and snapshot helper are not yet implemented. Do not change production code until the failure is observed.

---

### Task 2: Replace verbose squad rendering with a readable table layout

**Files:**
- Modify: `internal/auction/utils.go`
- Test: `internal/auction/auction_test.go`

**Interfaces:**
- Preserve `func PrintAllSquads(sess *model.Session)`.
- Preserve `SquadByRole`, `RoleSlots`, `RoleSlotsSummary`, `roleOrder`, and all existing domain behavior.
- The renderer remains stdout-based, matching the existing CLI presentation helpers.

- [ ] **Step 1: Add the coach summary header and aligned rows**

Keep the auction metadata at the top, then print a fixed-column summary before the player details:

```go
fmt.Println()
fmt.Printf("%-18s %8s %8s %7s %7s\n", "Coach", "Budget", "Max bid", "Spent", "Roster")
fmt.Printf("%-18s %8s %8s %7s %7s\n", "------------------", "------", "-------", "-----", "------")
```

For each coach, print `Name`, `Info.Budget`, `Info.MaxBid`, `Info.TotalSpent`, and `len(Players)/costants.SquadSize` using the same widths. Preserve the auction-level metadata (`Auction`, `Status`, and `Updated`) while changing only the per-coach presentation format.

- [ ] **Step 2: Print slots and grouped players compactly**

For each coach, retain the slot summary and print one line per role. Use the existing `playersByRole` map and `roleOrder`; do not sort or mutate the coach’s player slice. The output shape should be:

```text
  Slots: GK(1/3), DF(0/8), MD(0/8), FW(1/6)
    Goalkeeper: Keeper (1 M)
    Defender: -
    Midfielder: -
    Forward: Messi (100 M)
```

When a role has multiple players, keep them on the same role line separated by `, `. This keeps all players visible while reducing vertical output. Keep empty roles visible as `-`.

- [ ] **Step 3: Run the auction tests**

Run:

```sh
cd /home/ludovg/projects/lazy-fanta
go test ./internal/auction
```

Expected: PASS, including the existing role-ordering/domain tests and the updated output assertions.

---

### Task 3: Clear and redraw the watch dashboard on every snapshot

**Files:**
- Modify: `internal/commands/auction.go`
- Test: `internal/commands/auction_test.go`

**Interfaces:**
- Add one package-local helper:

```go
func printWatchSnapshot(sess *model.Session)
```

- `printWatchSnapshot` emits `"\033[2J\033[H"` and then calls `auction.PrintAllSquads(sess)`.
- `watchSession` remains the only polling loop and continues to accept the existing context, path, and interval arguments.

- [ ] **Step 1: Implement the snapshot helper**

Use the standard terminal clear-and-home sequence without adding a dependency:

```go
func printWatchSnapshot(sess *model.Session) {
    fmt.Print("\033[2J\033[H")
    auction.PrintAllSquads(sess)
}
```

This is intentionally terminal-oriented because `auction watch` is being used interactively.

- [ ] **Step 2: Route the initial render through the helper**

Replace the direct initial `auction.PrintAllSquads(currentSession)` call in `watchSession` with:

```go
printWatchSnapshot(currentSession)
```

This ensures the dashboard starts at the top of a clean visible screen.

- [ ] **Step 3: Route successful changed-file renders through the helper**

After a successful `reloadSession(path)`, update the stored signature and call `printWatchSnapshot(currentSession)`. Keep the existing order that leaves the active session and previous signature unchanged when reload fails:

```go
if err := reloadSession(path); err != nil {
    fmt.Fprintf(os.Stderr, "watch: reload failed: %v\n", err)
    continue
}
last = next
haveSignature = true
printWatchSnapshot(currentSession)
```

Do not redraw when the signature is unchanged or when stat/reload fails.

- [ ] **Step 4: Run the command tests**

Run:

```sh
cd /home/ludovg/projects/lazy-fanta
go test ./internal/commands
```

Expected: PASS, including registration, flag reset, safe reload, selector resolution, file signature, and snapshot-clear tests.

---

### Task 4: Document the interactive dashboard behavior

**Files:**
- Modify: `README.md`
- Modify: `docs/CODEBASE.md`

**Interfaces:**
- Documentation must describe the existing command syntax and the updated display behavior without introducing new flags or dependencies.

- [ ] **Step 1: Update the README watch description**

State that `auction watch` is intended for an interactive terminal, clears and redraws the latest snapshot after a successful file change, and displays aligned coach summary columns plus complete role-grouped squads.

Keep the existing selector and interval examples:

```text
auction watch [name|ID|path]
auction watch <session-ID-or-path> --interval 500ms
```

- [ ] **Step 2: Update the codebase guide**

Update the live-watch section to say that the watcher clears the visible terminal screen before its initial display and each successful redraw. Preserve the existing limitations: ANSI behavior depends on terminal support, and modification-time plus file-size detection can miss a rewrite with identical metadata.

- [ ] **Step 3: Review the diff for scope**

Confirm that only the renderer, watch snapshot wiring, focused tests, and documentation changed. Do not add content hashing, TTY detection, alternate-screen mode, cursor hiding, or a new table package in this task.

---

### Task 5: Run full verification

**Files:**
- No additional files.

- [ ] **Step 1: Run the complete test suite**

```sh
cd /home/ludovg/projects/lazy-fanta
go test ./...
```

Expected: all packages pass.

- [ ] **Step 2: Build the CLI**

```sh
cd /home/ludovg/projects/lazy-fanta
go build ./cmd/lazy-fanta
```

Expected: the command completes successfully.

- [ ] **Step 3: Perform a manual interactive smoke test**

Create or use a session, then run:

```sh
./lazy-fanta auction watch <session-ID-or-path> --interval 500ms
```

In a second terminal, mutate and save the same session. Confirm that the first terminal shows one refreshed dashboard rather than appending another snapshot, and that the refreshed table contains the new player or coach data.

- [ ] **Step 4: Report remaining limitations**

Report that the dashboard requires an ANSI-capable interactive terminal, very large squads can still exceed a small terminal viewport, and unchanged modification-time/file-size metadata can still prevent detection of a rewrite.
