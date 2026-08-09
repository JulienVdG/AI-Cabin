package main

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
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

		if len(profiles) == 0 {
			fmt.Println("No profiles found. Create one in ~/.config/ai-cabin/profiles/")
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
		fmt.Println("Variables:")
		for k, v := range profile.Vars {
			fmt.Printf("  %s=%s\n", k, v)
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

		// Verify profile exists
		_, err := config.LoadProfile(profileName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Profile %q not found: %v\n", profileName, err)
			os.Exit(1)
		}

		if err := config.SetCurrentProfile(profileName); err != nil {
			fmt.Fprintf(os.Stderr, "Error setting profile: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Active profile set to %q\n", profileName)
	},
}

// profileInitForce overwrites an existing profile (and re-copies the desk
// skeleton) when set by --force. Without it, init on an existing profile is a
// no-op (warn + exit 0, mirroring `cabin add`).
var profileInitForce bool

// profileInitSkeleton selects the desk skeleton to copy to AI_CABIN_DESK: a
// name (resolved from the embedded catalogue, e.g. "minimal") or a path to a
// skeleton directory on disk. Omitted -> "minimal" (the zero-config default).
var profileInitSkeleton string

// profileInitCmd creates a profile and copies the desk skeleton to the
// profile's AI_CABIN_DESK. The persisted var set is bounded (defaults ∪ --var
// ∪ existing-on-force); see config.InitProfile for the persistence rule.
var profileInitCmd = &cobra.Command{
	Use:   "init [name]",
	Short: "Create a profile and copy the desk skeleton",
	Long: `Create a new profile with default values derived from the environment, and copy the desk skeleton to AI_CABIN_DESK.

On an existing profile: no-op (warn + exit 0); use --force to overwrite both the profile and re-copy the desk skeleton. --skeleton accepts a name (embedded: minimal) or a path to a skeleton directory; omitted -> minimal. --var KEY=VAL adds/overrides vars (repeatable); it acts as the initial set of the profile CRUD, so a custom path is set with e.g. --var AI_CABIN_DESK=/custom/path.`,
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

		desk := profile.Vars[config.DeskVar]
		if desk == "" {
			fmt.Fprintf(os.Stderr, "Error: AI_CABIN_DESK is not set in the profile\n")
			os.Exit(1)
		}
		written, err := applyDeskSkeleton(profileInitSkeleton, desk, profile.Vars, profileInitForce)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Created profile %q at %s\n", profile.Name, profile.Path())
		fmt.Println("Variables:")
		for k, v := range profile.Vars {
			fmt.Printf("  %s=%s\n", k, v)
		}
		fmt.Printf("Copied desk skeleton (%d files) to %s\n", len(written), desk)
	},
}

// applyDeskSkeleton resolves the --skeleton flag (a name or a path) and copies
// the desk skeleton tree to dest (the resolved AI_CABIN_DESK). A name resolves
// to an embedded skeleton (desks/<name>); a path resolves to a directory on
// disk. Omitted -> "minimal" (the zero-config default of `cabin setup`). The
// write policy is SkipCreator (no-overwrite existing desk files) unless force
// selects TruncateCreator (--force re-copies over an existing desk).
func applyDeskSkeleton(skeleton, dest string, vars map[string]string, force bool) ([]string, error) {
	if skeleton == "" {
		skeleton = "minimal"
	}

	var srcFS fs.FS
	var srcRoot string
	if filepath.IsAbs(skeleton) || strings.ContainsRune(skeleton, '/') {
		// A path: resolve from disk. os.DirFS rooted at the skeleton dir.
		info, err := os.Stat(skeleton)
		if err != nil {
			return nil, fmt.Errorf("skeleton path %q: %w", skeleton, err)
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("skeleton path %q is not a directory", skeleton)
		}
		srcFS = os.DirFS(skeleton)
		srcRoot = "."
	} else {
		// A name: resolve from the embedded skeletons catalogue. fs.Sub does
		// not implement StatFS, so the name is validated via ReadDir (which
		// lists the available skeletons for a helpful error too).
		emb, err := embedded.Skeletons()
		if err != nil {
			return nil, err
		}
		entries, err := fs.ReadDir(emb, "desks")
		if err != nil {
			return nil, fmt.Errorf("read embedded skeletons catalogue: %w", err)
		}
		var available []string
		found := false
		for _, e := range entries {
			available = append(available, e.Name())
			if e.Name() == skeleton {
				found = true
			}
		}
		if !found {
			return nil, fmt.Errorf("skeleton %q not found in the embedded catalogue (available: %s)",
				skeleton, strings.Join(available, ", "))
		}
		srcFS = emb
		srcRoot = path.Join("desks", skeleton)
	}

	if err := os.MkdirAll(dest, 0o755); err != nil {
		return nil, fmt.Errorf("create desk dir %q: %w", dest, err)
	}

	creator := writestrategy.FileCreator(writestrategy.SkipCreator{})
	if force {
		creator = writestrategy.TruncateCreator{}
	}
	return skeletons.Apply(srcFS, srcRoot, dest, vars, creator)
}

func init() {
	profileInitCmd.Flags().BoolVarP(&profileInitForce, "force", "f", false,
		"overwrite an existing profile and re-copy the desk skeleton")
	profileInitCmd.Flags().StringVar(&profileInitSkeleton, "skeleton", "",
		"desk skeleton to copy to AI_CABIN_DESK (embedded name e.g. minimal, or a path)")
	profileCmd.AddCommand(profileListCmd)
	profileCmd.AddCommand(profileShowCmd)
	profileCmd.AddCommand(profileUseCmd)
	profileCmd.AddCommand(profileInitCmd)
	rootCmd.AddCommand(profileCmd)
}
