package main

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"sort"
	"strings"

	"github.com/JulienVdG/AI-Cabin/internal/config"
	"github.com/JulienVdG/AI-Cabin/internal/embedded"
	"github.com/JulienVdG/AI-Cabin/internal/skeletons"
	"github.com/JulienVdG/AI-Cabin/internal/writestrategy"

	"github.com/spf13/cobra"
)

// profileCmd represents the profile command
var profileCmd = &cobra.Command{
	Use:   "profile",
	Short: "Manage AI-Cabin profiles",
	Long:  `Profiles define environment variables for different contexts (personal, work, etc.).`,
}

// profileListCmd represents the profile list command
var profileListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available profiles",
	Run: func(cmd *cobra.Command, args []string) {
		profiles, err := config.ListProfiles()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		profilesDir, err := config.GetProfilesDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Profiles: %s\n", profilesDir)

		if len(profiles) == 0 {
			fmt.Println("No profiles found.")
			return
		}

		current, err := config.GetCurrentProfile()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not get current profile: %v\n", err)
		}

		for _, p := range profiles {
			if p == current {
				fmt.Printf("* %s (current)\n", p)
			} else {
				fmt.Printf("  %s\n", p)
			}
		}
	},
}

// profileShowCmd represents the profile show command
var profileShowCmd = &cobra.Command{
	Use:   "show [name]",
	Short: "Show profile details",
	Long:  `Show details of a specific profile. If no name is provided, shows the current profile.`,
	Args:  cobra.MaximumNArgs(1),
	// <name> is completed from the available profiles.
	ValidArgsFunction: completeProfileNames,
	Run: func(cmd *cobra.Command, args []string) {
		var profileName string
		if len(args) > 0 {
			profileName = args[0]
		}

		profile, err := config.GetActiveProfile(profileName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Profile: %s\n", profile.Name)
		fmt.Printf("Path: %s\n", profile.Path())
		printVars(profile.Vars)

		// Warn about profile vars that the process env shadows (env wins over
		// profile in the resolved view), so silent precedence surprises are
		// visible before the view is set on a subprocess. stdout stays the
		// profile content; the warning goes to stderr.
		if overrides := config.EnvShadowed(profile.Vars); len(overrides) > 0 {
			keys := make([]string, 0, len(overrides))
			for k := range overrides {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			fmt.Fprintf(os.Stderr, "Warning: these profile variables are overridden by the environment (env wins):\n")
			for _, k := range keys {
				fmt.Fprintf(os.Stderr, "  %s (env=%s)\n", k, overrides[k])
			}
		}
	},
}

// profileUseCmd represents the profile use command
var profileUseCmd = &cobra.Command{
	Use:   "use <name>",
	Short: "Select active profile",
	Long:  `Select which profile to use as the default for AI-Cabin commands.`,
	Args:  cobra.ExactArgs(1),
	// <name> is completed from the available profiles.
	ValidArgsFunction: completeProfileNames,
	Run: func(cmd *cobra.Command, args []string) {
		profileName := args[0]

		// Verify profile exists (LoadProfile resolves an absolute Path() to
		// display where the selected profile lives).
		profile, err := config.LoadProfile(profileName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Profile %q not found: %v\n", profileName, err)
			os.Exit(1)
		}

		if err := config.SetCurrentProfile(profileName); err != nil {
			fmt.Fprintf(os.Stderr, "Error setting profile: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Profile: %s\n", profile.Name)
		fmt.Printf("Path: %s\n", profile.Path())
		fmt.Printf("Active profile set to %q\n", profileName)
	},
}

// profileSetCmd sets a single profile variable and persists it atomically. It
// targets the profile selected by --profile (default: the current profile),
// matching the other runtime commands. It is the CRUD continuation of `profile init --var` (the initial set).
var profileSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a variable on a profile",
	Long:  `Set a variable on the profile selected by --profile (default: the current profile) and persist it atomically. Any key is allowed; it is the runtime continuation of the --var CRUD (of which profile init --var is the initial set).`,
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		key, value := args[0], args[1]

		profile, err := config.SetProfileVar(profileFlag, key, value)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Profile: %s\n", profile.Name)
		fmt.Printf("Path: %s\n", profile.Path())
		fmt.Printf("Set %s=%s on profile %q\n", key, value, profile.Name)
	},
}

// profileInitForce overwrites an existing profile (and re-copies the desk
// skeleton) when set by --force. Without it, init on an existing profile is a
// no-op (warn + exit 0, mirroring `cabin add`).
var profileInitForce bool

// profileInitSkeleton selects the desk skeleton to copy to AI_CABIN_DESK: a
// name resolved from the skeleton catalogue (AI_CABIN_SKELETON_DIRS + the
// embedded `minimal`). Omitted -> "minimal" (the zero-config default).
var profileInitSkeleton string

// profileInitCmd creates a profile and copies the desk skeleton to the
// profile's AI_CABIN_DESK. The persisted var set is bounded (defaults ∪ --var
// ∪ existing-on-force); see config.InitProfile for the persistence rule.
var profileInitCmd = &cobra.Command{
	Use:   "init [name]",
	Short: "Create a profile and copy the desk skeleton",
	Long: `Create a new profile with default values derived from the environment, and copy the desk skeleton to AI_CABIN_DESK.

On an existing profile: no-op (warn + exit 0); use --force to overwrite both the profile and re-copy the desk skeleton. --skeleton accepts a name (resolved from AI_CABIN_SKELETON_DIRS + the embedded minimal); omitted -> minimal. --var KEY=VAL adds/overrides vars (repeatable); it acts as the initial set of the profile CRUD, so a custom path is set with e.g. --var AI_CABIN_DESK=/custom/path.`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := "default"
		if len(args) > 0 {
			name = args[0]
		}

		exists, err := config.ProfileExists(name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		if exists && !profileInitForce {
			fmt.Fprintf(os.Stderr, "Warning: profile %q already exists. Use --force to overwrite.\n", name)
			return
		}

		profile, err := config.InitProfile(name, cliVars, profileInitForce)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Profile: %s\n", profile.Name)
		fmt.Printf("Path: %s\n", profile.Path())
		if exists {
			fmt.Printf("Updated profile\n")
		} else {
			fmt.Printf("Created profile\n")
		}
		printVars(profile.Vars)

		desk := profile.Vars[config.DeskVar]
		if desk == "" {
			fmt.Fprintf(os.Stderr, "Error: AI_CABIN_DESK is not set in the profile\n")
			os.Exit(1)
		}
		// Resolve the runtime view (env included) for the skeleton catalogue:
		// AI_CABIN_SKELETON_DIRS is read from env/--var, not persisted to the
		// bounded profile, so profile.Vars alone would miss repo skeletons.
		resolvedVars, err := config.ResolveVars(profileFlag, cliVars)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		written, err := applyDeskSkeleton(profileInitSkeleton, desk, resolvedVars.AsMap(), profileInitForce)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Copied desk skeleton (%d files) to %s\n", len(written), desk)

		// Activate the profile only once the environment is in place: a failed
		// profile/desk must never become (or clobber) the current profile.
		if err := config.SetCurrentProfile(name); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Active profile set to %q\n", name)
	},
}

// applyDeskSkeleton resolves the desk skeleton by name and copies its tree to
// dest (the resolved AI_CABIN_DESK). The skeleton is resolved from the union of
// AI_CABIN_SKELETON_DIRS (a PATH-style list resolved from vars) and the
// embedded catalogue (desks/minimal); there is no path mode. Omitted ->
// "minimal" (the zero-config default of `cabin setup`). Desks take no attrs.
// The write policy is SkipCreator (no-overwrite existing desk files) unless
// force selects TruncateCreator (--force re-copies over an existing desk).
func applyDeskSkeleton(skeleton, dest string, vars map[string]string, force bool) ([]string, error) {
	if skeleton == "" {
		skeleton = "minimal"
	}

	dirs := config.Vars(vars).SkeletonDirs()
	emb, err := embedded.Skeletons()
	if err != nil {
		return nil, err
	}
	merged, err := skeletons.BuildLayers(dirs, emb)
	if err != nil {
		return nil, err
	}

	// Validate the name resolves to an existing skeleton dir before Apply
	// (which would surface a missing manifest error otherwise). Stat on the
	// union finds it in any layer; a helpful error lists the desks catalogue.
	skeletonRoot := path.Join("desks", skeleton)
	if _, err := fs.Stat(merged, skeletonRoot); err != nil {
		available := listSkeletons(merged, "desks")
		return nil, fmt.Errorf("desk skeleton %q not found (available: %s)",
			skeleton, strings.Join(available, ", "))
	}

	if err := os.MkdirAll(dest, 0o755); err != nil {
		return nil, fmt.Errorf("create desk dir %q: %w", dest, err)
	}

	creator := writestrategy.FileCreator(writestrategy.SkipCreator{})
	if force {
		creator = writestrategy.TruncateCreator{}
	}
	return skeletons.Apply(merged, skeletonRoot, dest, vars, nil, creator)
}

func init() {
	profileInitCmd.Flags().BoolVarP(&profileInitForce, "force", "f", false,
		"overwrite an existing profile and re-copy the desk skeleton")
	profileInitCmd.Flags().StringVar(&profileInitSkeleton, "skeleton", "",
		"desk skeleton to copy to AI_CABIN_DESK (embedded name e.g. minimal, or a path)")
	if err := profileInitCmd.RegisterFlagCompletionFunc("skeleton", completeDeskSkeletonNames); err != nil {
		panic(fmt.Sprintf("register --skeleton completion: %v", err))
	}
	profileCmd.AddCommand(profileListCmd)
	profileCmd.AddCommand(profileShowCmd)
	profileCmd.AddCommand(profileUseCmd)
	profileCmd.AddCommand(profileSetCmd)
	profileCmd.AddCommand(profileInitCmd)
	rootCmd.AddCommand(profileCmd)
}
