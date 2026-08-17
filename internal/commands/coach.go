package commands

import (
	"fmt"
	"strings"

	"lazy-fanta/internal/auction"
	"lazy-fanta/internal/costants"
	"lazy-fanta/internal/storage"

	"github.com/spf13/cobra"
)

var (
	coachName  string
	coachesNum int
)

var CoachCmd = &cobra.Command{
	Use: "coach",
}

var addCoachCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a coach to the auction",
	RunE: func(cmd *cobra.Command, args []string) error {
		if currentSession == nil {
			return fmt.Errorf("No active session.")
		}
		name := strings.TrimSpace(coachName)
		if name == "" {
			return fmt.Errorf("flag --name is required")
		}
		for _, c := range currentSession.Coaches {
			if strings.EqualFold(c.Name, name) {
				return fmt.Errorf("coach %q already exists", name)
			}
		}
		if err := auction.AddCoach(currentSession, name); err != nil {
			return err
		}
		if err := storage.SaveSession(currentSession); err != nil {
			return err
		}
		fmt.Printf("  Coach added: %s\n", name)
		return nil
	},
}

var addListCoachCmd = &cobra.Command{
	Use:     "addlist -n <N> <name...>",
	Short:   "Add multiple coaches at once",
	Example: "coach addlist -n 3 John Jane Bob",
	Args:    cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if currentSession == nil {
			return fmt.Errorf("No active session.")
		}
		n := coachesNum
		if n < 1 {
			return fmt.Errorf("flag -n is required and must be >= 1")
		}
		names := args
		if len(names) != n {
			return fmt.Errorf("expected %d names, got %d", n, len(names))
		}
		if err := auction.AddCoaches(currentSession, names); err != nil {
			return err
		}
		if err := storage.SaveSession(currentSession); err != nil {
			return err
		}
		fmt.Printf("  Coach added:")
		for _, c := range currentSession.Coaches[len(currentSession.Coaches)-len(names):] {
			fmt.Printf(" %s,", c.Name)
		}
		fmt.Printf("\n")
		return nil
	},
}

var editCoachCmd = &cobra.Command{
	Use:   "edit",
	Short: "Edit coach data",
	Args:  cobra.ExactArgs(1), // new name
	RunE: func(cmd *cobra.Command, args []string) error {
		if currentSession == nil {
			return fmt.Errorf("no active session")
		}
		oldName := strings.TrimSpace(coachName)
		newName := strings.TrimSpace(args[0])
		if oldName == "" {
			return fmt.Errorf("flag --name (old coach name) is required")
		}
		if err := auction.RenameCoach(currentSession, oldName, newName); err != nil {
			return err
		}
		if err := storage.SaveSession(currentSession); err != nil {
			return err
		}
		fmt.Printf("  Coach renamed: %s -> %s\n", oldName, newName)
		return nil
	},
}

var coachSquadCmd = &cobra.Command{
	Use:   "squad",
	Short: "Show a coach's squad and budget (active session)",
	Run: func(cmd *cobra.Command, args []string) {
		name := strings.TrimSpace(coachName)
		if name == "" {
			fmt.Println("  flag --name is required")
			return
		}
		if currentSession == nil {
			fmt.Println("  No active session.")
			return
		}
		for i := range currentSession.Coaches {
			c := &currentSession.Coaches[i]
			if strings.EqualFold(c.Name, name) {
				fmt.Printf("  Squad: %s — Budget %d M - Max Bid %d M - Spent %d M\n  Players (%d/%d) | %s\n",
					c.Name, c.Info.Budget, c.Info.MaxBid, c.Info.TotalSpent,
					len(c.Players), costants.SquadSize, auction.RoleSlotsSummary(c))
				for _, p := range auction.SquadByRole(c) {
					fmt.Printf("  - %s (%s) %d M.\n", p.Name, p.Role, p.Cost)
				}
				return
			}
		}
		fmt.Printf("  Coach %q not found in the active session.\n", name)
	},
}

func init() {
	addCoachCmd.Flags().StringVar(&coachName, "name", "", "Coach name")
	addListCoachCmd.Flags().IntVarP(&coachesNum, "num", "n", 0, "coaches number")
	editCoachCmd.Flags().StringVar(&coachName, "name", "", "Old coach name")
	coachSquadCmd.Flags().StringVar(&coachName, "name", "", "Coach name")
	CoachCmd.AddCommand(addCoachCmd, addListCoachCmd, coachSquadCmd, editCoachCmd)
}
