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
// AI_CABIN_PROFILE env and config.yaml); --var KEY=VAL overrides a single
// var (repeatable, highest precedence). On `cabin task`
// (SetInterspersed(false)) pass them BEFORE positional args:
// `cabin --profile x task pi-go pi`.
var (
	profileFlag string
	cliVars     []string
)

func init() {
	rootCmd.PersistentFlags().StringVar(&profileFlag, "profile", "", "profile to use (overrides AI_CABIN_PROFILE and config.yaml)")
	rootCmd.PersistentFlags().StringArrayVar(&cliVars, "var", nil, "var override KEY=VAL (repeatable; highest precedence)")
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.go.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
