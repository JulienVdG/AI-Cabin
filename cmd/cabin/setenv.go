package main

import (
	"fmt"
	"os"
	"sort"

	"github.com/JulienVdG/AI-Cabin/internal/config"

	"github.com/spf13/cobra"
)

// setenvCmd represents the setenv command
var setenvCmd = &cobra.Command{
	Use:   "setenv [profile]",
	Short: "Output environment variables for eval",
	Long: `Output environment variables in a format suitable for eval in the shell.

This command is designed to be used with eval in your shell configuration:
  eval "$(cabin setenv)"           # uses current profile
  eval "$(cabin setenv perso)"     # uses specific profile

Example usage in .bashrc or .envrc:
  eval "$(cabin setenv)"
`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		// Positional <profile> is a shorthand for --profile (backward compat
		// with `eval "$(cabin setenv perso)"`); wins over the flag if both set.
		profileName := profileFlag
		if len(args) > 0 {
			profileName = args[0]
		}

		vars, err := config.ResolveVars(profileName, cliVars)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		// Output in sorted order for consistency
		keys := make([]string, 0, len(vars))
		for k := range vars {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, k := range keys {
			v := vars[k]
			fmt.Printf("export %s=%q\n", k, v)
		}
	},
}

func init() {
	rootCmd.AddCommand(setenvCmd)
}
