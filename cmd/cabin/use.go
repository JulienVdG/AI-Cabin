package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/JulienVdG/AI-Cabin/internal/config"

	"github.com/spf13/cobra"
)

// useCmd sets the current cabin for the active profile. It is thin sugar over
// `cabin profile set AI_CABIN_CURRENT_CABIN <cabin>`, adding what the generic
// var set cannot: registry validation (the cabin must exist) and completion of
// registered names. The current cabin lets cabin-scoped commands (up, task, ...)
// omit --cabin; the environment outranks it, so an exported
// AI_CABIN_CURRENT_CABIN takes precedence over the profile file.
var useCmd = &cobra.Command{
	Use:   "use <cabin>",
	Short: "Set the current cabin for the active profile",
	Long: `Set which cabin the cabin-scoped commands (up, down, build, shell,
greyshell, logs, restart, task) target by default, so they can omit --cabin.

The cabin must be registered first with 'cabin add'. The current cabin is a
profile variable (AI_CABIN_CURRENT_CABIN): it is stored per profile, hidden
behind --profile, and overridden by an exported AI_CABIN_CURRENT_CABIN.
This command is sugar for:
  cabin profile set AI_CABIN_CURRENT_CABIN <cabin>

The resolution order is: --cabin flag > AI_CABIN_CURRENT_CABIN env > the
current cabin of the active profile.
`,
	Args: cobra.ExactArgs(1),
	// <cabin> is completed from the registry (config.ListCabins).
	ValidArgsFunction: completeCabinNames,
	Run: func(cmd *cobra.Command, args []string) {
		cabinName := args[0]

		// The registry is the source of truth: the current cabin must name a
		// registered cabin (a per-profile var would accept anything, which
		// would make the first future `cabin up` fail with a confusing
		// not-registered error).
		if _, err := config.GetCabin(cabinName); err != nil {
			if errors.Is(err, config.ErrCabinNotFound) {
				fmt.Fprintf(os.Stderr,
					"Error: cabin %q is not registered.\n"+
						"Run 'cabin add <path> %s' to register it, or 'cabin list' to see known cabins.\n",
					cabinName, cabinName)
			} else {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			}
			os.Exit(1)
		}

		profile, err := config.SetProfileVar(profileFlag, config.CurrentCabinVar, cabinName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Profile: %s\n", profile.Name)
		fmt.Printf("Path: %s\n", profile.Path())
		fmt.Printf("Current cabin set to %q on profile %q\n", cabinName, profile.Name)
	},
}

func init() {
	rootCmd.AddCommand(useCmd)
}
