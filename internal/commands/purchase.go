package commands

import (
	"fmt"
	"os"
	"strings"

	"lazy-fanta/internal/auction"
	"lazy-fanta/internal/costants"
	"lazy-fanta/internal/parsing"
	"lazy-fanta/internal/storage"

	"github.com/spf13/cobra"
)

var (
	byCoachName string
	playerName  string
	playerRole  string
	cost        int
	purchaseID  int
	listRole    string
	listCost    bool
)

var PurchaseCmd = &cobra.Command{
	Use:   "purchase",
	Short: "Register a player purchase",
}

var addPurchaseCmd = &cobra.Command{
	Use:     "new",
	Short:   "Register a player purchase for a coach",
	Example: "purchase new --by John --player Messi --role fw --cost 120",
	RunE: func(cmd *cobra.Command, args []string) error {
		if currentSession == nil {
			return fmt.Errorf("no active session: create one with 'auction new <name>' or 'auction load <file>'")
		}
		coach := strings.TrimSpace(byCoachName)
		player := strings.TrimSpace(playerName)
		if coach == "" || player == "" || cost < 1 {
			return fmt.Errorf("required flags: --by, --player and --cost >= 1")
		}
		role, err := parsing.ParseRole(strings.TrimSpace(playerRole))
		if err != nil {
			return err
		}
		if err := auction.AddPurchase(currentSession, coach, player, role, cost); err != nil {
			return err
		}
		if err := storage.SaveSession(currentSession); err != nil {
			return err
		}
		fmt.Printf("Purchase recorded: %s (%s) to %s for %d M.\n", player, role, coach, cost)
		return nil
	},
}

var editPurchaseCmd = &cobra.Command{
	Use:     "edit",
	Short:   "Edit a purchased coach player's role and cost (re-creates the purchase)",
	Example: "purchase edit --by John --player Messi --role fw --cost 150",
	RunE: func(cmd *cobra.Command, args []string) error {
		if currentSession == nil {
			return fmt.Errorf("no active session")
		}
		coach := strings.TrimSpace(byCoachName)
		player := strings.TrimSpace(playerName)
		if coach == "" || player == "" || cost < 1 {
			return fmt.Errorf("required flags: --by, --player and --cost >= 1")
		}
		role, err := parsing.ParseRole(strings.TrimSpace(playerRole))
		if err != nil {
			return err
		}
		if err := auction.EditPurchase(currentSession, coach, player, role, cost); err != nil {
			return err
		}
		if err := storage.SaveSession(currentSession); err != nil {
			return err
		}
		fmt.Printf("Purchase updated: %s (%s) for %s at %d M.\n", player, role, coach, cost)
		return nil
	},
}

var removePurchaseCmd = &cobra.Command{
	Use:   "remove",
	Short: "Remove a purchased player by purchase ID",
	RunE: func(cmd *cobra.Command, args []string) error {
		if currentSession == nil {
			return fmt.Errorf("no active session")
		}
		if purchaseID < 1 {
			return fmt.Errorf("flag --id is required and must be >= 1")
		}
		if err := auction.RemovePurchase(currentSession, purchaseID); err != nil {
			return err
		}
		if err := storage.SaveSession(currentSession); err != nil {
			return err
		}
		fmt.Printf("Purchase (Id: %d)  removed.\n", purchaseID)
		return nil
	},
}

var listPurchaseCmd = &cobra.Command{
	Use:   "list",
	Short: "Show all purchased players, optionally filtered by role and cost",
	Run: func(cmd *cobra.Command, args []string) {
		if currentSession == nil {
			fmt.Println("No active session.")
			return
		}
		if len(currentSession.Purchases) == 0 {
			fmt.Println("No purchases recorded.")
			return
		}
		purchases := currentSession.Purchases
		if listRole != "" {
			role, err := parsing.ParseRole(strings.TrimSpace(listRole))
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return
			}
			purchases = auction.PurchasesByRole(currentSession, role)
		}
		if listCost {
			purchases = auction.PurchasesByCostDesc(purchases)
		}
		fmt.Println("Purchases:")
		for _, p := range purchases {
			fmt.Printf("- %s (%s) bought by %s for %d M. [%s] [%d]\n",
				p.PlayerName, p.PlayerRole, p.CoachName, p.Cost,
				p.Timestamp.Format(costants.SessionTimeFormat), p.ID)
		}
		if len(purchases) == 0 {
			fmt.Println("  (no purchases match the filters)")
		}
	},
}

var findPurchaseCmd = &cobra.Command{
	Use:   "find",
	Short: "Find a purchased player by name",
	RunE: func(cmd *cobra.Command, args []string) error {
		if currentSession == nil {
			return fmt.Errorf("no active session")
		}
		name := strings.TrimSpace(playerName)
		if name == "" {
			return fmt.Errorf("flag --player is required")
		}
		found := false
		for _, p := range currentSession.Purchases {
			if strings.EqualFold(p.PlayerName, name) {
				fmt.Printf("- %s (%s) bought by %s for %d M. [%s] [%d]\n",
					p.PlayerName, p.PlayerRole, p.CoachName, p.Cost,
					p.Timestamp.Format(costants.SessionTimeFormat), p.ID)
				found = true
			}
		}
		if !found {
			fmt.Printf("Player %q not found among purchases.\n", name)
		}
		return nil
	},
}

func init() {
	addPurchaseCmd.Flags().StringVarP(&byCoachName, "by", "b", "", "Coach name")
	addPurchaseCmd.Flags().StringVarP(&playerName, "player", "p", "", "Player name")
	addPurchaseCmd.Flags().StringVarP(&playerRole, "role", "r", "", "Role (gk/df/md/fw)")
	addPurchaseCmd.Flags().IntVarP(&cost, "cost", "c", 0, "Cost in credits")

	editPurchaseCmd.Flags().StringVarP(&byCoachName, "by", "b", "", "Coach name")
	editPurchaseCmd.Flags().StringVarP(&playerName, "player", "p", "", "Player name")
	editPurchaseCmd.Flags().StringVarP(&playerRole, "role", "r", "", "Role (gk/df/md/fw)")
	editPurchaseCmd.Flags().IntVarP(&cost, "cost", "c", 0, "Cost in credits")

	removePurchaseCmd.Flags().IntVar(&purchaseID, "id", 0, "Purchase ID")

	listPurchaseCmd.Flags().StringVar(&listRole, "role", "", "Filter by role (gk/df/md/fw)")
	listPurchaseCmd.Flags().BoolVar(&listCost, "cost", false, "Order by cost (highest first)")

	findPurchaseCmd.Flags().StringVarP(&playerName, "player", "p", "", "Player name")

	PurchaseCmd.AddCommand(addPurchaseCmd, editPurchaseCmd, removePurchaseCmd, listPurchaseCmd, findPurchaseCmd)
}
