package auction

import (
	"fmt"
	"lazy-fanta/internal/costants"
	"lazy-fanta/internal/model"
	"strings"
)

// maxBid returns the most a coach can bid on the next player: the budget minus
// 1 credit reserved for every player still to buy after this one.
// ponytail: flat 25-player count per spec; role-aware reservation
// (reserve 1 credit per unfilled SquadSlots[role]) is possible if MaxBid
// should differ by role.
func maxBid(budget, bought int) int {
	remaining := costants.SquadSize - bought
	if remaining <= 0 {
		return 0
	}
	mb := budget - (remaining - 1)
	if mb < 0 {
		return 0
	}
	return mb
}

// canBuy checks the coach still has a free squad slot for the role.
func canBuy(c *model.Coach, role costants.Role) error {
	limit := costants.SquadSlots[role]
	if limit == 0 {
		return fmt.Errorf("unknown role %q", role)
	}
	bought := 0
	for _, p := range c.Players {
		if p.Role == role {
			bought++
		}
	}
	if bought >= limit {
		return fmt.Errorf("no slot left for %s (max %d per squad)", role, limit)
	}
	return nil
}

// SplitArgs splits a REPL line into args, honoring double quotes ("Serie A"
// stays one arg) and keeping empty quoted args ("" is an empty arg, not nothing).
func SplitArgs(line string) []string {
	var args []string
	var cur strings.Builder
	inQuotes := false
	flush := func() {
		args = append(args, cur.String())
		cur.Reset()
	}
	for _, r := range line {
		switch {
		case r == '"':
			if inQuotes {
				// closing quote: keep an empty quoted segment as an empty arg
				inQuotes = false
				if cur.Len() == 0 {
					flush()
				}
			} else {
				inQuotes = true
			}
		case (r == ' ' || r == '\t') && !inQuotes:
			if cur.Len() > 0 {
				flush()
			}
		default:
			cur.WriteRune(r)
		}
	}
	if cur.Len() > 0 {
		flush()
	}
	return args
}

// JoinPlayerFlag merges unquoted words after --player/-p into one arg, so
// `--player Lionel Messi` becomes `--player "Lionel Messi"`. Stops at the
// next flag token. Idempotent on already-quoted single-token values.
func JoinPlayerFlag(args []string) []string {
	out := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		out = append(out, args[i])
		if args[i] != "--player" && args[i] != "-p" {
			continue
		}
		var words []string
		j := i + 1
		for j < len(args) && !strings.HasPrefix(args[j], "-") {
			words = append(words, args[j])
			j++
		}
		if len(words) > 0 {
			out = append(out, strings.Join(words, " "))
		}
		i = j - 1
	}
	return out
}
