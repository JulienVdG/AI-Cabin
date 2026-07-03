package main

import (
	"fmt"
	"os"

	"github.com/JulienVdG/AI-Cabin/internal/config"

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

// profileInitCmd represents the profile init command
var profileInitCmd = &cobra.Command{
	Use:   "init [name]",
	Short: "Create a default profile",
	Long:  `Create a new profile with default values derived from the current environment.`,
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		profileName := "default"
		if len(args) > 0 {
			profileName = args[0]
		}

		// Check if profile already exists
		exists, err := config.ProfileExists(profileName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error checking profile existence: %v\n", err)
			os.Exit(1)
		}
		if exists {
			fmt.Fprintf(os.Stderr, "Profile %q already exists. Use --force to overwrite.\n", profileName)
			os.Exit(1)
		}

		profile, err := config.CreateDefaultProfile(profileName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating profile: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Created profile %q at %s\n", profile.Name, profile.Path())
		fmt.Println("Variables:")
		for k, v := range profile.Vars {
			fmt.Printf("  %s=%s\n", k, v)
		}
	},
}

func init() {
	profileCmd.AddCommand(profileListCmd)
	profileCmd.AddCommand(profileShowCmd)
	profileCmd.AddCommand(profileUseCmd)
	profileCmd.AddCommand(profileInitCmd)
	rootCmd.AddCommand(profileCmd)
}
