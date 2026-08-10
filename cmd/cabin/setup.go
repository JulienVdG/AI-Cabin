package main

import (
	"fmt"
	"os"

	"github.com/JulienVdG/AI-Cabin/internal/config"

	"github.com/spf13/cobra"
)

// setupCmd is the zero-config env bootstrap (Class 1): it prepares the host-side
// environment so a cabin can run. It materializes the lifecycle Taskfile to
// XDG state (so `task` resolves the cabin's includes: at parse time), creates
// the default dirs (~/Documents/desk, ~/projects), and creates a default
// profile pointing at them with the minimal desk skeleton copied to
// AI_CABIN_DESK.
//
// It is the Go successor to bootstrap-cabin.sh (a profile XDG instead of
// .envrc). Agent-config materialization stays lazy, triggered on the first
// `cabin task`. The --var global flag reaches it (e.g. a custom desk path),
// but it carries no --skeleton/--force: zero-config means the default profile
// and the minimal desk; customization happens via `cabin profile init`.
var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Zero-config environment bootstrap",
	Long: `Prepare the host-side environment so a cabin can run.

Creates a default profile (default dirs + git identity derived from the host),
copies the minimal desk skeleton to AI_CABIN_DESK, creates the default workdir
(~/projects), and materializes the shared lifecycle Taskfile to XDG state so
task resolves a cabin's includes at parse time.

Idempotent: re-running with an existing default profile is a no-op (the profile
and desk are not overwritten). Use ` + "`cabin profile init --force`" + ` to overwrite.
Agent-config materialization stays lazy, triggered on the first cabin task.`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		// 1. Default profile (or no-op if it exists): the --var global flag
		// reaches InitProfile, so a custom desk path works the same as
		// `cabin profile init --var AI_CABIN_DESK=...`.
		profile, err := config.InitProfile("default", cliVars, false)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		created := profile.Name == "default" && profile.Vars[config.DeskVar] != ""

		// 2. Desk skeleton (minimal, no-overwrite): the profile's AI_CABIN_DESK
		// is the resolved value (--var > env > defaults), so a --var override
		// on setup lands the desk where the user asked.
		desk := profile.Vars[config.DeskVar]
		if desk == "" {
			fmt.Fprintf(os.Stderr, "Error: AI_CABIN_DESK is not set in the profile\n")
			os.Exit(1)
		}
		written, err := applyDeskSkeleton("minimal", desk, profile.Vars, false)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		// 3. Workdir (default ~/projects): created from the resolved profile
		// var, so a --var AI_CABIN_WORKDIR override is honored.
		workdir := profile.Vars[config.WorkdirVar]
		if err := os.MkdirAll(workdir, 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "Error: create workdir %q: %v\n", workdir, err)
			os.Exit(1)
		}

		// 4. Lifecycle Taskfile to XDG state (idempotent content-compare) so
		// `task` resolves a cabin's includes: at parse time before any task
		// runs. Reused from runtime.go (runCabinTask + completion).
		lifecyclePath, err := ensureLifecycleArtifact()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: materialize lifecycle taskfile: %v\n", err)
			os.Exit(1)
		}

		_ = created // (kept for a future "already set up" vs "fresh" message)
		fmt.Printf("AI-Cabin environment ready.\n")
		fmt.Printf("  Profile:    %s at %s\n", profile.Name, profile.Path())
		fmt.Printf("  Desk:       %s (%d skeleton files)\n", desk, len(written))
		fmt.Printf("  Workdir:    %s\n", workdir)
		fmt.Printf("  Lifecycle:  %s\n", lifecyclePath)
		fmt.Println("\nNext: register a cabin with `cabin cabin add <path>` and run `cabin task <cabin> <task>`.")
	},
}

func init() {
	rootCmd.AddCommand(setupCmd)
}
