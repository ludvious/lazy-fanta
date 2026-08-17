package auction

import (
	"fmt"
	"lazy-fanta/internal/costants"
	"lazy-fanta/internal/model"
	"lazy-fanta/internal/storage"
	"os"
	"sort"
	"strings"
)

func Greet() {
	fmt.Println("")
	fmt.Println(" ███                          ███▀██                       ")
	fmt.Println("▀███    ▄█▀█▄ ▀▀▀█▄ ▄█ █▄    ▄███▄   ▄█▀█▄ ██▀█▄ ▄██▄ ▄█▀█▄")
	fmt.Println(" ███ ▄▄ ██▀██ ▄█▀▀  ██ ██     ███    ██▀██ ██ ██  ██  ██▀██")
	fmt.Println("  ▀▀▀▀▀ ▀▀ ▀▀  ▀▀▀▀  ▀▀██     ▀▀▀    ▀▀ ▀▀ ▀▀ ▀▀  ▀▀  ▀▀ ▀▀")
	fmt.Println("                     ▀▀▀                                   ")
	fmt.Println("v0.1.0")
	fmt.Println("")
}

func PrintSessionList() {
	sessions, err := storage.ListSessions()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}
	if len(sessions) == 0 {
		fmt.Println("  No saved auction sessions. Create one with 'auction new <name>'.")
		return
	}
	fmt.Println("  Available auction sessions:")
	for _, s := range sessions {
		fmt.Printf("  - %-20s (%-16s) [%s] Status: %s\n", s.AuctionName, s.ID, s.UpdatedAt, s.Status)
	}
}

func PrintInfo(sess *model.Session) {
	fmt.Printf("  Name: %s (ID:%s)\n  Status: %s\n", sess.AuctionName, sess.ID, sess.Status)
	for _, c := range sess.Coaches {
		fmt.Printf("  - %-18s players %2d/%-2d  budget %4d  max bid %4d  spent %4d\n",
			c.Name, len(c.Players), costants.SquadSize,
			c.Info.Budget, c.Info.MaxBid, c.Info.TotalSpent)
	}
}

// PrintAllSquads prints the auction metadata and every coach's complete squad.
func PrintAllSquads(sess *model.Session) {
	fmt.Printf("Auction: %s (ID: %s)\nStatus: %s | Updated: %s\n", sess.AuctionName, sess.ID, sess.Status, sess.UpdatedAt)
	fmt.Println()
	fmt.Printf("%-18s %8s %8s %7s %7s\n", "Coach", "Budget", "Max bid", "Spent", "Roster")
	fmt.Printf("%-18s %8s %8s %7s %7s\n", "------------------", "------", "-------", "-----", "------")
	for _, c := range sess.Coaches {
		fmt.Printf("%-18s %8d %8d %7d %2d/%-4d\n",
			c.Name, c.Info.Budget, c.Info.MaxBid, c.Info.TotalSpent, len(c.Players), costants.SquadSize)
	}
	for _, c := range sess.Coaches {
		printCoachSquad(&c)
	}
}

// PrintSquad prints the auction metadata and one coach's complete squad.
func PrintSquad(sess *model.Session, coachName string) error {
	idx := findCoach(sess, coachName)
	if idx == -1 {
		return fmt.Errorf("coach %q not found in session", coachName)
	}

	fmt.Printf("Auction: %s (ID: %s)\nStatus: %s | Updated: %s\n", sess.AuctionName, sess.ID, sess.Status, sess.UpdatedAt)
	fmt.Println()
	fmt.Printf("%-18s %8s %8s %7s %7s\n", "Coach", "Budget", "Max bid", "Spent", "Roster")
	fmt.Printf("%-18s %8s %8s %7s %7s\n", "------------------", "------", "-------", "-----", "------")
	c := &sess.Coaches[idx]
	fmt.Printf("%-18s %8d %8d %7d %2d/%-4d\n",
		c.Name, c.Info.Budget, c.Info.MaxBid, c.Info.TotalSpent, len(c.Players), costants.SquadSize)
	printCoachSquad(c)
	return nil
}

func printCoachSquad(c *model.Coach) {
	fmt.Printf("\nCoach: %s\n", c.Name)
	fmt.Printf("  Slots: %s\n", RoleSlotsSummary(c))

	playersByRole := make(map[costants.Role][]model.Player, len(roleOrder))
	for _, p := range c.Players {
		playersByRole[p.Role] = append(playersByRole[p.Role], p)
	}
	for _, role := range roleOrder {
		players := playersByRole[role]
		if len(players) == 0 {
			fmt.Printf("    %s: -\n", role)
			continue
		}
		playersText := make([]string, len(players))
		for i, p := range players {
			playersText[i] = fmt.Sprintf("%s (%d M)", p.Name, p.Cost)
		}
		fmt.Printf("    %s: %s\n", role, strings.Join(playersText, ", "))
	}
}

// roleOrder is the display order of roles on a squad sheet.
var roleOrder = []costants.Role{costants.Goalkeeper, costants.Defender, costants.Midfielder, costants.Forward}

// roleRank returns the position of a role in roleOrder.
func roleRank(r costants.Role) int {
	for i, rr := range roleOrder {
		if r == rr {
			return i
		}
	}
	return len(roleOrder)
}

// roleShort maps a role to its display shorthand.
var roleShort = map[costants.Role]string{
	costants.Goalkeeper: "GK",
	costants.Defender:   "DF",
	costants.Midfielder: "MD",
	costants.Forward:    "FW",
}

// SquadByRole returns a copy of the coach's players ordered by role
// (Goalkeeper, Defender, Midfielder, Forward), keeping insertion order within
// each role. It does not mutate the coach.
func SquadByRole(c *model.Coach) []model.Player {
	out := make([]model.Player, len(c.Players))
	copy(out, c.Players)
	sort.SliceStable(out, func(i, j int) bool {
		return roleRank(out[i].Role) < roleRank(out[j].Role)
	})
	return out
}

// PurchasesByRole returns the purchases whose player has the given role,
// in the original recording order.
func PurchasesByRole(sess *model.Session, role costants.Role) []model.Purchase {
	out := []model.Purchase{}
	for _, p := range sess.Purchases {
		if p.PlayerRole == role {
			out = append(out, p)
		}
	}
	return out
}

// PurchasesByCostDesc returns a copy of the purchases ordered by cost,
// highest first (stable: equal costs keep the original order).
func PurchasesByCostDesc(purchases []model.Purchase) []model.Purchase {
	out := make([]model.Purchase, len(purchases))
	copy(out, purchases)
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Cost > out[j].Cost
	})
	return out
}

// RoleSlot is one role's bought/total slot count for a coach.
type RoleSlot struct {
	Role   costants.Role
	Bought int
	Total  int
}

// String renders slot as "Role(3/3)".
func (s RoleSlot) String() string {
	return fmt.Sprintf("%s(%d/%d)", roleShort[s.Role], s.Bought, s.Total)
}

// RoleSlots returns each role's bought/total for the coach, in roleOrder.
func RoleSlots(c *model.Coach) []RoleSlot {
	out := make([]RoleSlot, 0, len(roleOrder))
	for _, r := range roleOrder {
		bought := 0
		for _, p := range c.Players {
			if p.Role == r {
				bought++
			}
		}
		out = append(out, RoleSlot{Role: r, Bought: bought, Total: costants.SquadSlots[r]})
	}
	return out
}

// RoleSlotsSummary returns a one-line summary like "GK(3/3), DF(0/8), MD(0/8), FW(0/6)".
func RoleSlotsSummary(c *model.Coach) string {
	slots := RoleSlots(c)
	parts := make([]string, len(slots))
	for i, s := range slots {
		parts[i] = s.String()
	}
	return strings.Join(parts, ", ")
}
