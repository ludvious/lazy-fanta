# lazy-fanta — Codebase Guide

Fast, practical onboarding for a new agent/session. Read this top-to-bottom in ~5 minutes; you won't need to grep the tree blind.

## What this is

A Go CLI for running a **fantacalcio auction** (Italian fantasy-football draft). It tracks coaches, their budgets, and purchases, persisting each session to JSON and exporting to CSV.

- **Language / module:** Go 1.24, module `lazy-fanta`
- **One dependency:** `spf13/cobra` v1.8.0 (CLI framework) + 2 transitive deps (pflag, mousetrap). Nothing else.
- **State model:** everything is in-memory in one `model.Session` struct; JSON is just a serialization of it.

## Build & run

```sh
go build -o lazy-fanta ./cmd/lazy-fanta
go test ./...            # all packages; no test framework, plain stdlib testing

./lazy-fanta                        # interactive REPL (recommended)
./lazy-fanta auction list           # one-shot command
./lazy-fanta purchase new --by John --player Messi --role fw --cost 120
```

Two execution modes share the same cobra command tree:
- **One-shot:** `os.Args[1:]` is passed through `JoinPlayerFlag` then `rootCmd.Execute()`.
- **REPL:** a `bufio.Scanner` loop reads lines, tokenizes them, and re-runs the same `rootCmd`.

## Directory map

| Path | Package | Purpose |
|---|---|---|
| `cmd/lazy-fanta/root.go` | `main` | Entry point, REPL loop, cobra root wiring |
| `internal/commands/` | `commands` | Cobra subcommands (auction/coach/purchase) + global `currentSession` state |
| `internal/auction/` | `auction` | **Domain logic**: purchase validation, budget math, coach ops, display/query helpers |
| `internal/model/` | `model` | Data structs (no logic) |
| `internal/storage/` | `storage` | JSON persistence, CSV export, session listing/resolution |
| `internal/costants/` | `costants` | Constants + `Role` type. **Note: package name is a typo of "constants".** |
| `internal/parsing/` | `parsing` | Role shorthand → `Role` mapping |
| `data/session/*.json` | — | Persisted sessions (gitignored-able runtime data) |
| `data/export/*.csv` | — | `auction end` export |

**Layering rule:** `commands` → `auction` + `storage` + `parsing` → `model` + `costants`. `model`/`costants` are leaf packages. Commands never touch JSON directly; they call `storage` and `auction`, then `storage.SaveSession` explicitly.

## Data model (`internal/model/model.go`)

```
Session{ Filepath, ID, AuctionName, UpdatedAt, Status, Purchases[], Coaches[] }
  Coach{ Name, Info, Players[] }
    Info{ InitialBudget, Budget, MaxBid, TotalSpent }
    Player{ Name, Role, Cost }          // a coach's squad member
  Purchase{ ID, CoachName, PlayerName, PlayerRole, Cost, Timestamp }  // the ledger
```

Key facts:
- `Player` = current squad; `Purchase` = append-only-ish ledger. They're kept in sync manually (see `AddPurchase`/`RemovePurchase`).
- Roles are stored as **full English strings** (`"Goalkeeper"`, `"Defender"`, `"Midfielder"`, `"Forward"`), not the shorthands. Shorthands exist only at the CLI boundary.
- `Session.Filepath` and `ID` are baked in at creation (`auction.NewAuction`).
- `UpdatedAt` is a string formatted `2006-01-02 15:04:05`; `Purchase.Timestamp` is a full `time.Time`.

## Domain rules (`internal/auction/` + `internal/costants/`)

- Budget: **500** credits/coach (`costants.DefaultBudget`).
- Squad size: **25** — 3 GK / 8 DF / 8 MD / 6 FW (`costants.SquadSlots`).
- **Max bid** = `budget - (remainingSlots - 1)`: reserve 1 credit for every slot still to fill. Implemented in `auction.maxBid` (unexported, flat 25-player count — see the `ponytail:` comment for the role-aware upgrade path).
- A purchase is rejected if: coach missing, role slot full (`canBuy`), cost < 1, cost > budget, or cost > max bid.

## Core flows (the parts you'll actually touch)

### Purchase lifecycle — `auction.AddPurchase`
1. `findCoach` (case-insensitive) → index.
2. `canBuy` checks the role still has a free slot.
3. Validate `cost >= 1`, `<= Budget`, `<= MaxBid`.
4. Append `Player` to `Coach.Players`; append `Purchase` to `Session.Purchases` with `NextPurchaseID`.
5. Update `Budget -= cost`, `TotalSpent += cost`, `MaxBid = maxBid(...)`.

**`AddPurchase` does NOT persist.** Every command wrapper calls `storage.SaveSession` after a successful mutation. If you add a new mutating command, you must save.

- `RemovePurchase(id)`: refunds budget, removes the matching `Player`, drops the ledger entry.
- `EditPurchase(...)`: remove old → re-add new; rolls back on failure (re-adds the old values). Note the re-add can fail silently if the rollback itself is invalid — see the `_ =` in `EditPurchase`.
- `RenameCoach`: renames the coach **and** rewrites `CoachName` in every matching `Purchase`.

### Session lifecycle — `storage`
- `NewAuction(name)` → builds ID `slug-xxxx` (`NewAuctionID`), sets `Filepath = data/session/<id>.json`.
- `SaveSession` → `MkdirAll` + `json.MarshalIndent` → temporary file in the target directory → `Sync`/close → rename over the destination. Temporary files are removed on failure.
- `ResolveSession(key)` resolves by **path → ID → name** (name match picks the most recent `UpdatedAt`).
- `ExportCSV` writes `data/export/<id>.csv` (purchases only).
- `auction update` reloads the exact `currentSession.Filepath`, replacing the active pointer only after a successful read and parse.
- `auction watch [path|ID|auctionName] --interval` resolves and loads its selector before polling, or watches the active session when no selector is supplied. It defaults to `2s`, clears the visible terminal screen before its initial display and each successful redraw, and `Ctrl-C`/SIGTERM exits cleanly.

### Live watch workflow

Run the watcher and the auction in separate terminals against the same JSON session:

```sh
# Terminal 1
./lazy-fanta auction watch <session-ID-or-path>

# Terminal 2
./lazy-fanta
> auction load <session-ID-or-path>
> coach add --name John
> purchase new --by John --player Messi --role fw --cost 120
```

The mutation commands use atomic session saves. The watcher uses modification time plus file size to detect changes, reports transient stat/read/parse failures, retries on the next poll, and clears/redraws the complete squad display only after a successful reload. The ANSI clear sequence depends on terminal support, and a rewrite with identical modification-time and file-size metadata can still be missed.

## REPL loop — `cmd/lazy-fanta/root.go`
```
scanner.Scan → SplitArgs(line) → JoinPlayerFlag(args) → rootCmd.Find → Execute → ResetFlags
```
- `SplitArgs` (in `auction/validator.go`) is a hand-rolled tokenizer honoring double quotes and empty `""` args.
- `JoinPlayerFlag` merges unquoted words after `--player`/`-p` so `--player Lionel Messi` works without quotes. It stops at the next `-`-prefixed token.
- `ResetFlags` zeroes the shared flag variables between REPL iterations (flags live in package-level vars, reused across `Execute` calls).

## Conventions & gotchas (read before editing)

1. **`currentSession` is a package-global** in `internal/commands`. Every command nil-checks it. This is why one-shot mode requires `auction load`/`auction new` first within the same process — it's the same global.
2. **Package name typo:** `internal/costants` (not "constants"). Import paths and package names must match — don't "fix" the import to `constants` without renaming the dir.
3. **Validation lives in `auction`, presentation in `commands`.** Commands do some light duplicate validation (trim/empty checks) for friendly errors, but the authoritative rules are in `auction`.
4. **Flag state is shared across commands**: e.g. `coachName` is bound to `coach add --name`, `coach edit --name`, and `coach squad --name`. `ResetFlags` must clear it between REPL commands.
5. **Case-insensitive matching** is used for coach names and `purchase find` (via `strings.EqualFold`), but stored names keep their original casing.
6. **`purchase list --cost` is a sort toggle** (descending by cost), not a filter; `--role` is the only filter.
7. **Tests are plain stdlib**: table-driven, in-package (`package auction`, `package storage`). No fixtures, no mocks. Add one `Test*` in the right package when you change logic.
8. **`ponytail:` comments** mark deliberate simplifications with a named ceiling (e.g. flat squad-size reservation in `maxBid`). Grep `ponytail:` for the known-debt ledger.

## Where to change X

| You want to… | Edit |
|---|---|
| Add a CLI command | `internal/commands/<domain>.go` + register in `init()` |
| Change auction/budget rules | `internal/auction/validator.go` (`maxBid`, `canBuy`), `auction.go` (`AddPurchase`) |
| Change the JSON/CSV format | `internal/model/model.go` (struct tags) + `internal/storage/sessions.go` |
| Change role shorthands/aliases | `internal/parsing/parsing.go` |
| Add a constant (budget, squad size) | `internal/costants/costants.go` |
| Change REPL tokenizing | `auction.SplitArgs` / `auction.JoinPlayerFlag` in `internal/auction/validator.go` |
| Change what `auction end` exports | `storage.ExportCSV` in `internal/storage/sessions.go` |

## Known simplification debt

From a prior ponytail audit (not yet applied): `RoleSlot` type + `RoleSlots` + `RoleSlot.String` are a three-symbol layer with one caller (`RoleSlotsSummary`) and can be inlined; `model.Info.InitialBudget` is written but never read; `addCoachCmd` duplicates the duplicate-name check already in `auction.AddCoach`. None of these affect behavior.
