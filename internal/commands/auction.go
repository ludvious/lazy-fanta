package commands

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"lazy-fanta/internal/auction"
	"lazy-fanta/internal/model"
	"lazy-fanta/internal/storage"

	"github.com/spf13/cobra"
)

// global var
var currentSession *model.Session

var (
	watchInterval  time.Duration
	watchCoachName string
	watchTmux      bool
)

type fileSignature struct {
	modTime time.Time
	size    int64
}

func reloadSession(path string) error {
	sess, err := storage.LoadSession(path)
	if err != nil {
		return err
	}
	currentSession = sess
	return nil
}

func selectWatchSession(selector string) (*model.Session, error) {
	if selector == "" {
		if currentSession == nil {
			return nil, fmt.Errorf("no active session: load one with 'auction load <file>'")
		}
		return currentSession, nil
	}
	resolved, err := storage.ResolveSession(selector)
	if err != nil {
		return nil, err
	}
	path := resolved.Filepath
	if path == "" {
		path = selector
	}
	if err := reloadSession(path); err != nil {
		return nil, err
	}
	return currentSession, nil
}

func sessionFileSignature(path string) (fileSignature, error) {
	info, err := os.Stat(path)
	if err != nil {
		return fileSignature{}, err
	}
	return fileSignature{modTime: info.ModTime(), size: info.Size()}, nil
}

func printWatchSnapshot(sess *model.Session, coachName string) error {
	fmt.Print("\033[2J\033[H")
	if coachName == "" {
		auction.PrintAllSquads(sess)
		return nil
	}
	return auction.PrintSquad(sess, coachName)
}

func tmuxSessionName(sess *model.Session) string {
	return "lazy-fanta-" + sess.ID
}

func tmuxWatchArgs(executable, selector, coachName string, interval time.Duration) []string {
	args := []string{executable, "auction", "watch", selector, "--name", coachName}
	if interval != 2*time.Second {
		args = append(args, "--interval", interval.String())
	}
	return args
}

func launchTmuxWatch(sess *model.Session, selector string, interval time.Duration) error {
	if interval <= 0 {
		return fmt.Errorf("watch interval must be greater than zero")
	}
	if len(sess.Coaches) == 0 {
		return fmt.Errorf("cannot launch tmux watch: session has no coaches")
	}
	tmux, err := exec.LookPath("tmux")
	if err != nil {
		return fmt.Errorf("tmux is unavailable: %w", err)
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	if selector == "" {
		selector = sess.Filepath
	}
	name := tmuxSessionName(sess)

	hasSession := exec.Command(tmux, "has-session", "-t", name)
	hasSession.Stdout = io.Discard
	hasSession.Stderr = io.Discard
	if err := hasSession.Run(); err == nil {
		attach := exec.Command(tmux, "attach-session", "-t", name)
		attach.Stdin = os.Stdin
		attach.Stdout = os.Stdout
		attach.Stderr = os.Stderr
		return attach.Run()
	}

	runTmux := func(args ...string) error {
		command := exec.Command(tmux, args...)
		command.Stdin = os.Stdin
		command.Stdout = os.Stdout
		command.Stderr = os.Stderr
		return command.Run()
	}

	first := sess.Coaches[0].Name
	if err := runTmux(append([]string{
		"new-session", "-d", "-s", name, "-n", "watch", "--",
	}, tmuxWatchArgs(executable, selector, first, interval)...)...); err != nil {
		return fmt.Errorf("tmux new-session failed: %w", err)
	}
	for _, coach := range sess.Coaches[1:] {
		if err := runTmux(append([]string{
			"split-window", "-t", name + ":0", "--",
		}, tmuxWatchArgs(executable, selector, coach.Name, interval)...)...); err != nil {
			return fmt.Errorf("tmux split-window for %q failed: %w", coach.Name, err)
		}
	}
	if len(sess.Coaches) > 1 {
		if err := runTmux("select-layout", "-t", name+":0", "tiled"); err != nil {
			return fmt.Errorf("tmux select-layout failed: %w", err)
		}
	}
	return runTmux("attach-session", "-t", name)
}

func watchSession(ctx context.Context, path string, interval time.Duration, coachName string) error {
	if interval <= 0 {
		return fmt.Errorf("watch interval must be greater than zero")
	}
	if err := printWatchSnapshot(currentSession, coachName); err != nil {
		return err
	}
	last, err := sessionFileSignature(path)
	haveSignature := err == nil
	if err != nil {
		fmt.Fprintf(os.Stderr, "watch: cannot stat %s: %v\n", path, err)
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			next, err := sessionFileSignature(path)
			if err != nil {
				fmt.Fprintf(os.Stderr, "watch: cannot stat %s: %v\n", path, err)
				continue
			}
			if haveSignature && next == last {
				continue
			}
			if err := reloadSession(path); err != nil {
				fmt.Fprintf(os.Stderr, "watch: reload failed: %v\n", err)
				continue
			}
			last = next
			haveSignature = true
			if err := printWatchSnapshot(currentSession, coachName); err != nil {
				fmt.Fprintf(os.Stderr, "watch: render failed: %v\n", err)
			}
		}
	}
}

var SessionCmd = &cobra.Command{
	Use:   "auction",
	Short: "Manage auction sessions",
}

var newSessionCmd = &cobra.Command{
	Use:   "new [auctionName]",
	Short: "Create a new auction session",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		sess := auction.NewAuction(args[0])
		if err := storage.SaveSession(sess); err != nil {
			fmt.Fprintf(os.Stderr, "Error saving: %v\n", err)
			return
		}
		currentSession = sess
		fmt.Printf("  New session created: %s (%s)\n", sess.AuctionName, sess.ID)
	},
}

var loadSessionCmd = &cobra.Command{
	Use:   "load [path|ID|auctionName]",
	Short: "Load an auction session by file path, session ID or auction name",
	Args:  cobra.MaximumNArgs(1),
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		sessions, err := storage.ListSessions()
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}
		var out []string
		for _, s := range sessions {
			out = append(out, s.ID, s.AuctionName)
		}
		return out, cobra.ShellCompDirectiveNoFileComp
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			fmt.Println("  Choose an auction: (or 'auction list')")
			auction.PrintSessionList()
			return nil
		}
		resolved, err := storage.ResolveSession(args[0])
		if err != nil {
			return err
		}
		path := resolved.Filepath
		if path == "" {
			path = args[0]
		}
		if err := reloadSession(path); err != nil {
			return err
		}
		fmt.Printf("  Auction session loaded: %s (%s)\n", currentSession.AuctionName, currentSession.ID)
		return nil
	},
}

var updateSessionCmd = &cobra.Command{
	Use:   "update",
	Short: "Reload the active auction session from disk",
	RunE: func(cmd *cobra.Command, args []string) error {
		if currentSession == nil {
			return fmt.Errorf("no active session")
		}
		path := currentSession.Filepath
		if err := reloadSession(path); err != nil {
			return err
		}
		fmt.Printf("  Auction session updated: %s (%s)\n", currentSession.AuctionName, currentSession.ID)
		return nil
	},
}

var watchSessionCmd = &cobra.Command{
	Use:   "watch [path|ID|auctionName]",
	Short: "Watch an auction session for changes",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		selector := ""
		if len(args) == 1 {
			selector = args[0]
		}
		sess, err := selectWatchSession(selector)
		if err != nil {
			return err
		}
		if watchTmux {
			if watchCoachName != "" {
				return fmt.Errorf("--tmux cannot be combined with --name")
			}
			return launchTmuxWatch(sess, selector, watchInterval)
		}
		ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		return watchSession(ctx, sess.Filepath, watchInterval, watchCoachName)
	},
}

var listSessionCmd = &cobra.Command{
	Use:   "list",
	Short: "List stored auction sessions",
	Run: func(cmd *cobra.Command, args []string) {
		auction.PrintSessionList()
	},
}

var saveSessionCmd = &cobra.Command{
	Use:   "save",
	Short: "Save the current auction session to JSON file",
	Run: func(cmd *cobra.Command, args []string) {
		if currentSession == nil {
			fmt.Println("  No active session.")
			return
		}
		if err := storage.SaveSession(currentSession); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return
		}
	},
}

var infoSessionCmd = &cobra.Command{
	Use:   "info",
	Short: "Show current auction session status",
	Run: func(cmd *cobra.Command, args []string) {
		if currentSession == nil {
			fmt.Println("  No active auction session.")
			return
		}
		auction.PrintInfo(currentSession)
	},
}

var clearSessionCmd = &cobra.Command{
	Use:   "clear",
	Short: "Delete the current session file",
	Run: func(cmd *cobra.Command, args []string) {
		if currentSession == nil {
			fmt.Println("  No active auction session.")
			return
		}
		if err := storage.ClearSession(currentSession); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return
		}
		currentSession = nil
	},
}

var endSessionCmd = &cobra.Command{
	Use:   "end",
	Short: "End the active session and export data (CSV + JSON)",
	Run: func(cmd *cobra.Command, args []string) {
		if currentSession == nil {
			fmt.Println("  No active auction session.")
			return
		}
		currentSession.Status = "Ended"
		if err := storage.SaveSession(currentSession); err != nil {
			fmt.Fprintf(os.Stderr, "Error saving: %v\n", err)
			return
		}
		if err := storage.ExportCSV(currentSession); err != nil {
			fmt.Fprintf(os.Stderr, "Error exporting: %v\n", err)
			return
		}
		fmt.Printf("  Auction %s ended. JSON saved at %s\n", currentSession.AuctionName, currentSession.Filepath)
	},
}

// ResetFlags clears flag values between interactive REPL iterations.
func ResetFlags() {
	coachName = ""
	byCoachName, playerName, playerRole = "", "", ""
	cost = 0
	coachesNum = 0
	purchaseID = 0
	listRole = ""
	listCost = false
	watchInterval = 2 * time.Second
	watchCoachName = ""
	watchTmux = false
}

// Register all subcommands.
func init() {
	watchSessionCmd.Flags().DurationVar(&watchInterval, "interval", 2*time.Second, "Polling interval")
	watchSessionCmd.Flags().StringVar(&watchCoachName, "name", "", "Show only this coach's squad")
	watchSessionCmd.Flags().BoolVar(&watchTmux, "tmux", false, "Open one watch window per coach in tmux")
	SessionCmd.AddCommand(newSessionCmd, loadSessionCmd, updateSessionCmd, watchSessionCmd, listSessionCmd, saveSessionCmd, infoSessionCmd, clearSessionCmd, endSessionCmd)
}
