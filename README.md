# lazy-fanta
Tired of using the standard MS Excel? Or the usual fancy third-party apps you have to pay for?
You need only a minimal CLI for running a fantacalcio auction. Records coaches, budgets, and purchases, persisting each session to JSON.

## Build & run

```sh
go build -o lazy-fanta ./cmd/lazy-fanta
./lazy-fanta              # interactive REPL (raccomended)
./lazy-fanta auction list # one-shot command
```

## Interactive mode

Running `./lazy-fanta` with no arguments drops into a REPL. Type commands as below, `q`/`quit` to exit.

## Commands Examples

### Sessions

```sh
auction new <name>                         # create a session
auction load [name|ID|path]                # load a session
auction update                             # reload the active session from disk
auction watch [name|ID|path] [flags]       # watch an interactive dashboard for file changes
auction watch <session-ID-or-path> --interval 500ms
auction watch <name|ID|path> --name <coach> # watch one coach's squad
auction watch <name|ID|path> --tmux        # open one tmux window with a pane per coach
auction list                               # list saved sessions
auction info                    # show the active session
auction save                    # persist the active session
auction clear                   # delete the active session file
auction end                     # end the active session and export the data (CSV + JSON)
```

### Coaches

```sh
coach add --name <name>                        # add one coach
coach addlist --num 3 Bob Alice John           # add N coaches at once
coach edit --name <OldName> <NewName>          # rename a coach
coach squad --name <CoachName>                 # show a coach's squad and budget details
```

Shorthands: `-n` (num)

### Purchases

```sh
purchase new --by <coach> --player <name> --role <role> --cost <n>  # add purchased player to a coach squad
purchase edit --by <coach> --player <name> --role <role> --cost <n> # edit purchased player, fix all values (delete and re-create a purchase with updated values and add to coach players)
purchase remove --id <purchaseID>                                   # delete purchased player by its purchaseID
purchase list  --role[Optional] --cost[Optional]                    # show all purchased players; --cost sorts by cost (highest first)
purchase find --player <name>                                       # find and show a specific player purchased(if it was) by its name
```

FlagsShorthands: `-b` (by), `-p` (player), `-r` (role), `-c` (cost).

*Multi names player can be typed without quotes: `--player Lionel Messi`. Of course, it works with brackets as well `--p "Lionel Messi"`*

Roles: `gk`, `df`, `md`, `fw` (Italian aliases like `por`, `dif`, `cen`, `att` also accepted).

`auction watch` has this syntax:

```sh
./lazy-fanta auction watch [name|ID|path] [flags]
```

The selector can be an auction name, session ID, or session JSON path. From a shell, provide a selector explicitly. In the REPL, load a session first and then the selector can be omitted:

```text
> auction load auctsumm26
> auction watch
```

Normal watch examples:

```sh
./lazy-fanta auction watch auctsumm26
./lazy-fanta auction watch auctisumm26ID
./lazy-fanta auction watch data/session/auctisumm26ID.json
./lazy-fanta auction watch auctsumm26 --name John
./lazy-fanta auction watch auctsumm26 --interval 500ms
```

The watcher is intended for an interactive terminal. It clears and redraws the latest snapshot after a successful session-file change, showing aligned coach summary columns and complete role-grouped squads. It polls every 2 seconds by default; use `--interval 500ms` (or any Go duration) to change that. Use `--name <coach>` to show only one coach's squad. The watcher exits with `Ctrl-C`.

From a shell, `--tmux` opens one attached tmux window with one split pane per coach:

```sh
./lazy-fanta auction watch ludo --tmux
./lazy-fanta auction watch ludo --tmux --interval 500ms
```

The tmux session is named `lazy-fanta-<session-ID>`. Its `watch` window contains one watcher pane for every coach, arranged with a tiled layout. An existing session is attached instead of duplicated. Tmux mode cannot be combined with `--name`; it requires `tmux` to be installed and a session with at least one coach.

Press `Ctrl-b d` to detach, then return with `tmux attach -t lazy-fanta-<session-ID>`. Panes reflect the coaches present when the command starts; roster changes do not create or remove tmux panes automatically.

### Two-terminal workflow

Use the same session file from two terminals:

```sh
# Terminal 1: keep the live squad sheet visible
./lazy-fanta auction watch <session-ID-or-path>

# Terminal 2: run the REPL and mutate that session
./lazy-fanta
> auction load <session-ID-or-path>
> coach add --name John
> purchase new --by John --player Messi --role fw --cost 120
```

Mutating commands save the session atomically; the watcher retries transient file read or parse errors and redraws after a successful reload.

## Rules Example

- Budget: **500** credits per coach
- Squad size: **25** players — 3 GK / 8 DF / 8 MD / 6 FW
- Max bid reserves 1 credit for every slot still to fill: `max bid = budget − (remaining slots − 1)`
- A purchase is rejected if it exceeds budget, max bid, or the role's remaining slots

Auction sessions are stored as JSON under `data/session/`.

## Export

`auction end` marks the session Ended, re-saves its JSON, and writes all purchases to `data/export/<session-id>.csv`. The session JSON is always stored under `data/session/`.
