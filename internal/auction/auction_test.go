package auction

import (
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"lazy-fanta/internal/costants"
	"lazy-fanta/internal/model"
)

func TestPrintAllSquadsShowsMetadataStatsAndRoleGroups(t *testing.T) {
	sess := &model.Session{
		AuctionName: "Final",
		ID:          "final-0001",
		Status:      "Active",
		Coaches: []model.Coach{{
			Name: "John",
			Info: model.Info{Budget: 400, MaxBid: 376, TotalSpent: 100},
			Players: []model.Player{
				{Name: "Messi", Role: costants.Forward, Cost: 100},
				{Name: "Keeper", Role: costants.Goalkeeper, Cost: 1},
			},
		}},
	}

	oldStdout := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = writer
	PrintAllSquads(sess)
	_ = writer.Close()
	os.Stdout = oldStdout
	output, err := io.ReadAll(reader)
	_ = reader.Close()
	if err != nil {
		t.Fatal(err)
	}
	text := string(output)
	for _, want := range []string{
		"Final", "final-0001", "Coach", "Budget", "Max bid", "Spent", "Roster",
		"John", "400", "376", "100", "2/25",
		"Slots:", "GK(1/3)", "DF(0/8)", "MD(0/8)", "FW(1/6)",
		"Goalkeeper:", "Keeper (1 M)",
		"Forward:", "Messi (100 M)",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("output missing %q:\n%s", want, text)
		}
	}

	roles := []string{"Goalkeeper:", "Defender:", "Midfielder:", "Forward:"}
	last := -1
	for _, role := range roles {
		index := strings.Index(text, role)
		if index < 0 {
			t.Errorf("output missing role label %q:\n%s", role, text)
			continue
		}
		if index < last {
			t.Errorf("role %q appears out of order:\n%s", role, text)
		}
		last = index
	}
}

func TestPrintSquadShowsOnlySelectedCoach(t *testing.T) {
	sess := &model.Session{
		AuctionName: "Final",
		ID:          "final-0001",
		Status:      "Active",
		Coaches: []model.Coach{
			{
				Name:    "John",
				Info:    model.Info{Budget: 400, MaxBid: 376, TotalSpent: 100},
				Players: []model.Player{{Name: "Messi", Role: costants.Forward, Cost: 100}},
			},
			{
				Name:    "Jane",
				Players: []model.Player{{Name: "Ronaldo", Role: costants.Forward, Cost: 90}},
			},
		},
	}

	oldStdout := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = writer
	if err := PrintSquad(sess, "john"); err != nil {
		t.Fatal(err)
	}
	_ = writer.Close()
	os.Stdout = oldStdout
	output, err := io.ReadAll(reader)
	_ = reader.Close()
	if err != nil {
		t.Fatal(err)
	}
	text := string(output)
	for _, want := range []string{"Final", "final-0001", "John", "Slots:", "FW(1/6)", "Messi (100 M)"} {
		if !strings.Contains(text, want) {
			t.Errorf("output missing %q:\n%s", want, text)
		}
	}
	for _, unwanted := range []string{"Jane", "Ronaldo"} {
		if strings.Contains(text, unwanted) {
			t.Errorf("output unexpectedly contains %q:\n%s", unwanted, text)
		}
	}
}

func TestPrintSquadRejectsUnknownCoach(t *testing.T) {
	sess := &model.Session{Coaches: []model.Coach{{Name: "John"}}}
	if err := PrintSquad(sess, "missing"); err == nil {
		t.Fatal("expected unknown coach error")
	}
}

func TestNewPurchaseUpdatesBudgetAndMaxBid(t *testing.T) {
	sess := NewAuction("test")
	if err := AddCoach(sess, "John"); err != nil {
		t.Fatal(err)
	}
	stats := &sess.Coaches[0].Info
	wantInitial := costants.DefaultBudget - (costants.SquadSize - 1) // e.g. 500 - 24
	if stats.MaxBid != wantInitial {
		t.Fatalf("initial max bid = %d, want %d", stats.MaxBid, wantInitial)
	}
	if err := AddPurchase(sess, "John", "Messi", costants.Forward, wantInitial); err != nil {
		t.Fatal(err)
	}
	wantBudget := costants.DefaultBudget - wantInitial
	if stats.Budget != wantBudget || stats.TotalSpent != wantInitial {
		t.Fatalf("budget = %d, spent = %d; want %d, %d", stats.Budget, stats.TotalSpent, wantBudget, wantInitial)
	}
	if stats.MaxBid != 1 { // e.g. 24 budget, 1 bought, 24 left: 24 - (24-1)
		t.Fatalf("max bid after purchase = %d, want 1", stats.MaxBid)
	}
	if len(sess.Purchases) != 1 || sess.Purchases[0].Cost != wantInitial {
		t.Fatalf("purchase not recorded: %+v", sess.Purchases)
	}

	if err := AddPurchase(sess, "John", "Ronaldo", costants.Forward, 2); err == nil {
		t.Fatal("expected error: cost 2 exceeds max bid 1")
	}
	if err := AddPurchase(sess, "Paul", "X", costants.Forward, 1); err == nil {
		t.Fatal("expected error: coach not found")
	}
	if err := AddPurchase(sess, "John", "Y", costants.Forward, 0); err == nil {
		t.Fatal("expected error: cost below 1")
	}
}

func TestAddPurchaseRoleSlots(t *testing.T) {
	sess := NewAuction("test")
	if err := AddCoach(sess, "John"); err != nil {
		t.Fatal(err)
	}
	// 3 GKs allowed, 4th must be rejected
	for i := 1; i <= 3; i++ {
		if err := AddPurchase(sess, "John", fmt.Sprintf("GK%d", i), costants.Goalkeeper, 1); err != nil {
			t.Fatalf("GK %d: %v", i, err)
		}
	}
	if err := AddPurchase(sess, "John", "GK4", costants.Goalkeeper, 1); err == nil {
		t.Fatal("expected error: 4th goalkeeper has no slot")
	}
}

func TestAddCoaches(t *testing.T) {
	sess := NewAuction("test")
	if err := AddCoach(sess, "Alex"); err != nil {
		t.Fatal(err)
	}
	if err := AddCoaches(sess, []string{"John", "Lucas"}); err != nil {
		t.Fatal(err)
	}
}

func TestAddCoachesRejects(t *testing.T) {
	sess := NewAuction("test")
	sess.Coaches = append(sess.Coaches, model.Coach{Name: "John"}) // add a first coach for test rejects
	for _, names := range [][]string{
		{"John"},      // already exists
		{"A", "a"},    // case-insensitive duplicate
		{"  ", "Bob"}, // empty name
		{"A", "John"}, // duplicate after first name in same batch
	} {
		if err := AddCoaches(sess, names); err == nil {
			t.Fatalf("expected error for %v", names)
		}
		if len(sess.Coaches) != 1 {
			t.Fatalf("roster mutated on error for %v", names)
		}
	}
}

func TestRenameCoach(t *testing.T) {
	sess := NewAuction("test")
	coaches := []model.Coach{{Name: "John"}, {Name: "Jake"}}
	sess.Coaches = append(sess.Coaches, coaches...)
	if err := RenameCoach(sess, "JOHN", "Giovanni"); err != nil {
		t.Fatal(err)
	}
	if sess.Coaches[0].Name != "Giovanni" {
		t.Fatalf("name = %q, want Giovanni", sess.Coaches[0].Name)
	}
	if err := RenameCoach(sess, "Giovanni", "Jake"); err == nil {
		t.Fatal("expected error: name already taken")
	}
	if err := RenameCoach(sess, "Nope", "X"); err == nil {
		t.Fatal("expected error: coach not found")
	}
}

func TestQueryUtils(t *testing.T) {
	coach := &model.Coach{Players: []model.Player{
		{Name: "A", Role: costants.Forward, Cost: 10},
		{Name: "B", Role: costants.Goalkeeper, Cost: 1},
		{Name: "C", Role: costants.Defender, Cost: 5},
		{Name: "D", Role: costants.Goalkeeper, Cost: 2},
	}}
	squad := SquadByRole(coach)
	want := []string{"B", "D", "C", "A"} // GK, GK, DF, FW; B before D stable
	for i, p := range squad {
		if p.Name != want[i] {
			t.Fatalf("SquadByRole[%d] = %s, want %s", i, p.Name, want[i])
		}
	}
	if coach.Players[0].Name != "A" {
		t.Fatal("SquadByRole mutated the coach's roster")
	}

	sess := &model.Session{Purchases: []model.Purchase{
		{PlayerName: "X", PlayerRole: costants.Forward, Cost: 30},
		{PlayerName: "Y", PlayerRole: costants.Midfielder, Cost: 20},
		{PlayerName: "Z", PlayerRole: costants.Forward, Cost: 50},
		{PlayerName: "W", PlayerRole: costants.Goalkeeper, Cost: 40},
	}}

	fw := PurchasesByRole(sess, costants.Forward)
	if len(fw) != 2 || fw[0].PlayerName != "X" || fw[1].PlayerName != "Z" {
		t.Fatalf("PurchasesByRole(fw) = %+v, want [X Z]", fw)
	}

	desc := PurchasesByCostDesc(sess.Purchases)
	wantCosts := []int{50, 40, 30, 20}
	for i, p := range desc {
		if p.Cost != wantCosts[i] {
			t.Fatalf("PurchasesByCostDesc[%d].Cost = %d, want %d", i, p.Cost, wantCosts[i])
		}
	}
}

func TestRoleSlots(t *testing.T) {
	coach := &model.Coach{Players: []model.Player{
		{Name: "A", Role: costants.Goalkeeper},
		{Name: "B", Role: costants.Goalkeeper},
		{Name: "C", Role: costants.Goalkeeper},
		{Name: "D", Role: costants.Forward},
	}}
	slots := RoleSlots(coach)
	if len(slots) != 4 {
		t.Fatalf("RoleSlots = %d slots, want 4", len(slots))
	}
	if slots[0].Bought != 3 || slots[0].Total != 3 {
		t.Fatalf("GK slot = %d/%d, want 3/3", slots[0].Bought, slots[0].Total)
	}
	if slots[1].Bought != 0 || slots[1].Total != 8 {
		t.Fatalf("DF slot = %d/%d, want 0/8", slots[1].Bought, slots[1].Total)
	}
	if got := RoleSlotsSummary(coach); got != "GK(3/3), DF(0/8), MD(0/8), FW(1/6)" {
		t.Fatalf("RoleSlotsSummary = %q", got)
	}
}

func TestRenameCoachUpdatesPurchases(t *testing.T) {
	sess := NewAuction("test")
	if err := AddCoach(sess, "John"); err != nil {
		t.Fatal(err)
	}
	if err := AddPurchase(sess, "John", "Messi", costants.Forward, 10); err != nil {
		t.Fatal(err)
	}
	if err := RenameCoach(sess, "John", "Giovanni"); err != nil {
		t.Fatal(err)
	}
	if sess.Purchases[0].CoachName != "Giovanni" {
		t.Fatalf("purchase coach = %q, want Giovanni", sess.Purchases[0].CoachName)
	}
}

func TestRemoveAndEditPurchase(t *testing.T) {
	sess := NewAuction("test")
	if err := AddCoach(sess, "John"); err != nil {
		t.Fatal(err)
	}
	if err := AddPurchase(sess, "John", "Messi", costants.Forward, 10); err != nil {
		t.Fatal(err)
	}
	id := sess.Purchases[0].ID
	if err := RemovePurchase(sess, id); err != nil {
		t.Fatal(err)
	}
	if len(sess.Purchases) != 0 || len(sess.Coaches[0].Players) != 0 {
		t.Fatalf("purchase not fully removed: %+v", sess)
	}
	if sess.Coaches[0].Info.Budget != costants.DefaultBudget {
		t.Fatalf("budget not refunded: %d", sess.Coaches[0].Info.Budget)
	}

	if err := AddPurchase(sess, "John", "Messi", costants.Forward, 10); err != nil {
		t.Fatal(err)
	}
	if err := EditPurchase(sess, "John", "Messi", costants.Defender, 20); err != nil {
		t.Fatal(err)
	}
	if len(sess.Purchases) != 1 || sess.Purchases[0].Cost != 20 || sess.Purchases[0].PlayerRole != costants.Defender {
		t.Fatalf("edit did not apply: %+v", sess.Purchases)
	}
	if sess.Coaches[0].Info.Budget != costants.DefaultBudget-20 {
		t.Fatalf("budget after edit = %d, want %d", sess.Coaches[0].Info.Budget, costants.DefaultBudget-20)
	}
}
