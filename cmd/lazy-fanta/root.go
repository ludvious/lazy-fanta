package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"lazy-fanta/internal/auction"
	"lazy-fanta/internal/commands"

	"github.com/spf13/cobra"
)

var rootCmd *cobra.Command

func init() {
	rootCmd = &cobra.Command{
		Short:         "CLI fantacalcio auction",
		Long:          "list all commands",
		SilenceUsage:  true,
		SilenceErrors: true, // cobra does not print the error: the caller (main/REPL) prints it once
		Run: func(cmd *cobra.Command, args []string) {
			if len(os.Args) == 1 {
				runInteractive()
			} else {
				cmd.Help()
			}
		},
	}
	// Register wires all commands onto the root command
	rootCmd.AddCommand(commands.SessionCmd, commands.CoachCmd, commands.PurchaseCmd)
}

func main() {
	rootCmd.SetArgs(auction.JoinPlayerFlag(os.Args[1:]))
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runInteractive() {
	scanner := bufio.NewScanner(os.Stdin)

	auction.Greet()

	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if line == "quit" || line == "q" {
			return
		}

		parts := auction.JoinPlayerFlag(auction.SplitArgs(line))
		if len(parts) == 0 {
			continue
		}
		sub, _, err := rootCmd.Find(parts)
		if err != nil || sub == rootCmd {
			fmt.Printf("Unknown command: %s\n", parts[0])
			continue
		}

		rootCmd.SetArgs(parts)
		if err := rootCmd.Execute(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
		commands.ResetFlags()
		rootCmd.SetArgs(nil)
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Read error: %v\n", err)
	}
}
