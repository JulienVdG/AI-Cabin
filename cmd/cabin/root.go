package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "cabin",
	Short: "AI-Cabin CLI: Manage your AI agent cabins",
	Long:  `You're the captain, AI is just another passenger on the boat 🚢`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := cmd.Help(); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	},
}

// Global persistent flags: --profile selects the profile file (overrides
// AI_CABIN_PROFILE env and config.yaml); --cabin selects the cabin target
// (overrides AI_CABIN_CURRENT_CABIN); --var KEY=VAL overrides a single
// var (repeatable, highest precedence). On `cabin task`
// (SetInterspersed(false)) pass them BEFORE positional args:
// `cabin --cabin pi-go task pi`.
var (
	profileFlag   string
	cabinFlag     string
	cliVars       []string
	noRelpathFlag bool
)

func init() {
	rootCmd.PersistentFlags().StringVar(&profileFlag, "profile", "", "profile to use (overrides AI_CABIN_PROFILE and config.yaml)")
	rootCmd.PersistentFlags().StringVar(&cabinFlag, "cabin", "", "cabin to use (overrides AI_CABIN_CURRENT_CABIN and the active profile's current cabin)")
	rootCmd.PersistentFlags().StringArrayVar(&cliVars, "var", nil, "var override KEY=VAL (repeatable; highest precedence)")
	rootCmd.PersistentFlags().BoolVar(&noRelpathFlag, "no-relpath", false, "skip path shadowing (launch the agent at the workdir root instead of the host CWD sub-path)")

	// Dynamic completion: --profile <TAB> suggests available profiles, and
	// --cabin <TAB> suggests registered cabins. Registered on root so every
	// subcommand (task, up, ...) inherits them. The completion subcommand
	// itself is Cobra's default (bash/zsh/fish script generation).
	if err := rootCmd.RegisterFlagCompletionFunc("profile", completeProfileNames); err != nil {
		// Non-fatal: completion is a UX nicety, not a core function. The binary
		// still works without it; surface the misconfiguration on stderr.
		fmt.Fprintf(os.Stderr, "Warning: could not register --profile completion: %v\n", err)
	}
	if err := rootCmd.RegisterFlagCompletionFunc("cabin", completeCabinNames); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not register --cabin completion: %v\n", err)
	}
	rootCmd.InitDefaultCompletionCmd()
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.go.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
