package auction

import (
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"lazy-fanta/internal/costants"
	"lazy-fanta/internal/model"
)

// NewAuctionID builds a short, typeable session ID from the auction name.
func NewAuctionID(auctionName string) string {
	slug := strings.ToLower(auctionName)
	slug = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			return r
		}
		return -1
	}, slug)
	if len(slug) > 12 {
		slug = slug[:12]
	}
	if slug == "" {
		slug = "asta"
	}
	return fmt.Sprintf("%s-%04x", slug, rand.Intn(0x10000))
}

// NewAuction creates a new auction session with the given name.
func NewAuction(auctionName string) *model.Session {
	id := NewAuctionID(auctionName)
	return &model.Session{
		Filepath:    "data/session/" + id + ".json",
		ID:          id,
		AuctionName: auctionName,
		UpdatedAt:   time.Now().Format(costants.SessionTimeFormat),
		Status:      "Active",
		Purchases:   []model.Purchase{},
		Coaches:     []model.Coach{},
	}
}

// NextPurchaseID returns the next purchase ID for a session.
func NextPurchaseID(purchases []model.Purchase) int {
	maxID := 0
	for _, p := range purchases {
		if p.ID > maxID {
			maxID = p.ID
		}
	}
	return maxID + 1
}

// AddPurchase validates and records a purchase and updates the coach's stats
// (budget, total spent, stored MaxBid). It does not persist: callers save.
// Gates: coach exists, free role slot (canBuy), cost >= 1, cost <= budget, cost <= maxBid.
// findCoach returns the index of the coach with the given name (case-insensitive), or -1.
func findCoach(sess *model.Session, name string) int {
	for i := range sess.Coaches {
		if strings.EqualFold(sess.Coaches[i].Name, name) {
			return i
		}
	}
	return -1
}

func AddPurchase(sess *model.Session, coachName, playerName string, role costants.Role, cost int) error {
	if sess == nil {
		return errors.New("no active session")
	}
	idx := findCoach(sess, coachName)
	if idx == -1 {
		return fmt.Errorf("coach %q not found in session", coachName)
	}
	c := &sess.Coaches[idx]
	if err := canBuy(c, role); err != nil {
		return err
	}
	if cost < 1 {
		return fmt.Errorf("cost must be at least 1 credit")
	}
	if cost > c.Info.Budget {
		return fmt.Errorf("cost %d exceeds budget %d", cost, c.Info.Budget)
	}
	if cost > c.Info.MaxBid {
		return fmt.Errorf("cost %d exceeds max bid %d (budget after reserving 1 credit per remaining slot)", cost, c.Info.MaxBid)
	}

	c.Players = append(c.Players, model.Player{Name: playerName, Role: role, Cost: cost})
	c.Info.Budget -= cost
	c.Info.TotalSpent += cost
	c.Info.MaxBid = maxBid(c.Info.Budget, len(c.Players))
	sess.Purchases = append(sess.Purchases, model.Purchase{
		ID:         NextPurchaseID(sess.Purchases),
		CoachName:  c.Name,
		PlayerName: playerName,
		PlayerRole: role,
		Cost:       cost,
		Timestamp:  time.Now(),
	})

	return nil
}

// RemovePurchase removes a purchase by ID, refunding the coach's budget and
// removing the player from the squad. It does not persist: callers save.
func RemovePurchase(sess *model.Session, purchaseID int) error {
	pidx := -1
	for i := range sess.Purchases {
		if sess.Purchases[i].ID == purchaseID {
			pidx = i
			break
		}
	}
	if pidx == -1 {
		return fmt.Errorf("purchase %d not found", purchaseID)
	}
	p := sess.Purchases[pidx]
	cidx := findCoach(sess, p.CoachName)
	if cidx == -1 {
		return fmt.Errorf("coach %q not found for purchase %d", p.CoachName, purchaseID)
	}
	c := &sess.Coaches[cidx]
	// drop the matching player from the squad
	for i := range c.Players {
		if c.Players[i].Name == p.PlayerName && c.Players[i].Role == p.PlayerRole && c.Players[i].Cost == p.Cost {
			c.Players = append(c.Players[:i], c.Players[i+1:]...)
			break
		}
	}
	c.Info.Budget += p.Cost
	c.Info.TotalSpent -= p.Cost
	c.Info.MaxBid = maxBid(c.Info.Budget, len(c.Players))
	sess.Purchases = append(sess.Purchases[:pidx], sess.Purchases[pidx+1:]...)
	return nil
}

// EditPurchase replaces a coach's purchase of a player with new role/cost
// values: it removes the old purchase (refunding the coach) and re-adds the
// player with the new values. If the re-add fails, the old purchase is restored.
func EditPurchase(sess *model.Session, coachName, playerName string, role costants.Role, cost int) error {
	idx := -1
	for i := range sess.Purchases {
		if strings.EqualFold(sess.Purchases[i].CoachName, coachName) && strings.EqualFold(sess.Purchases[i].PlayerName, playerName) {
			idx = i
			break
		}
	}
	if idx == -1 {
		return fmt.Errorf("no purchase found for player %q by coach %q", playerName, coachName)
	}
	oldID := sess.Purchases[idx].ID
	oldRole := sess.Purchases[idx].PlayerRole
	oldCost := sess.Purchases[idx].Cost
	if err := RemovePurchase(sess, oldID); err != nil {
		return err
	}
	if err := AddPurchase(sess, coachName, playerName, role, cost); err != nil {
		_ = AddPurchase(sess, coachName, playerName, oldRole, oldCost) // rollback
		return err
	}
	return nil
}

// AddCoach add new coach
func AddCoach(sess *model.Session, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("coach name must not be empty")
	}
	for _, c := range sess.Coaches {
		if strings.EqualFold(c.Name, name) {
			return fmt.Errorf("coach %q already exists", name)
		}
	}
	sess.Coaches = append(sess.Coaches, model.Coach{
		Name: strings.TrimSpace(name),
		Info: model.Info{
			InitialBudget: costants.DefaultBudget,
			Budget:        costants.DefaultBudget,
			MaxBid:        maxBid(costants.DefaultBudget, 0)},
		Players: []model.Player{},
	})

	return nil
}

// AddCoaches add new coaches by listing them names, auto-assigning team names team-N continuing
// after the highest existing one. All-or-nothing: validates the whole batch
// (no empty/duplicate names, none already in the roster) before appending.
func AddCoaches(sess *model.Session, names []string) error {
	// validate the whole batch first (all-or-nothing)
	trimmed := make([]string, len(names))
	seen := make(map[string]bool, len(names))
	for i, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			return fmt.Errorf("coach name must not be empty")
		}
		key := strings.ToLower(name)
		if seen[key] {
			return fmt.Errorf("duplicate coach name %q in batch", name)
		}
		for _, c := range sess.Coaches {
			if strings.EqualFold(c.Name, name) {
				return fmt.Errorf("coach %q already exists", name)
			}
		}
		seen[key] = true
		trimmed[i] = name
	}
	for _, name := range trimmed {
		sess.Coaches = append(sess.Coaches, model.Coach{
			Name: name,
			Info: model.Info{
				InitialBudget: costants.DefaultBudget,
				Budget:        costants.DefaultBudget,
				MaxBid:        maxBid(costants.DefaultBudget, 0)},
			Players: []model.Player{},
		})
	}

	return nil
}

// RenameCoach renames a coach (case-insensitive match), rejecting names already
// taken by another coach.
func RenameCoach(sess *model.Session, oldName, newName string) error {
	oldName = strings.TrimSpace(oldName)
	newName = strings.TrimSpace(newName)
	if oldName == "" || newName == "" {
		return errors.New("coach name must not be empty")
	}
	idx := findCoach(sess, oldName)
	if idx == -1 {
		return fmt.Errorf("coach %q not found", oldName)
	}
	for i := range sess.Coaches {
		if i != idx && strings.EqualFold(sess.Coaches[i].Name, newName) {
			return fmt.Errorf("coach %q already exists", newName)
		}
	}
	sess.Coaches[idx].Name = newName
	// keep recorded purchases consistent with the renamed coach
	for i := range sess.Purchases {
		if strings.EqualFold(sess.Purchases[i].CoachName, oldName) {
			sess.Purchases[i].CoachName = newName
		}
	}
	return nil
}
