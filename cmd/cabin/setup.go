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
		// Lifecycle first: the cabin is unusable without it, so fail fast (and
		// loudly) before touching the profile or desk. A read-only state dir is
		// worked around by redirecting the XDG vars to a writable path.
		if _, err := ensureLifecycleArtifact(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: materialize lifecycle taskfile: %v\n", err)
			fmt.Fprintln(os.Stderr, "The cabin is unusable without the lifecycle Taskfile.")
			fmt.Fprintln(os.Stderr, "If the state dir is read-only, redirect it to a writable path, e.g.:")
			fmt.Fprintln(os.Stderr, "  export XDG_STATE_HOME=/workspace/dev-state  XDG_CONFIG_HOME=/workspace/dev-config")
			os.Exit(1)
		}

		// Detect a fresh bootstrap vs a no-op re-run (setup has no --force).
		profExists, err := config.ProfileExists("default")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fresh := !profExists

		// Default profile: self-healing bootstrap. The forced merge
		// (defaults ∪ --var ∪ existing) refills missing structural vars (e.g.
		// a corrupted AI_CABIN_DESK) while preserving user-set keys. The --var
		// global flag sets a custom desk path the same way as
		// `cabin profile init --var AI_CABIN_DESK=...`.
		profile, err := config.InitProfile("default", cliVars, true)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		if fresh {
			fmt.Printf("Created profile %q at %s\n", profile.Name, profile.Path())
		} else {
			fmt.Printf("Ensured profile %q (user vars preserved)\n", profile.Name)
		}
		printVars(profile.Vars)

		// Desk skeleton (minimal, no-overwrite).
		desk := profile.Vars[config.DeskVar]
		if desk == "" {
			fmt.Fprintf(os.Stderr, "Error: AI_CABIN_DESK is not set in the profile\n")
			os.Exit(1)
		}
		resolvedVars, err := config.ResolveVars(profileFlag, cliVars)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		written, err := applyDeskSkeleton("minimal", desk, resolvedVars.AsMap(), false)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Copied desk skeleton (%d files) to %s\n", len(written), desk)

		// Workdir (default ~/projects): honors a --var AI_CABIN_WORKDIR override.
		workdir := profile.Vars[config.WorkdirVar]
		if err := os.MkdirAll(workdir, 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "Error: create workdir %q: %v\n", workdir, err)
			os.Exit(1)
		}
		fmt.Printf("Workdir: %s\n", workdir)

		// Activate default only once the environment is in place: a failed
		// bootstrap must never leave (or clobber) the current profile.
		if err := config.SetCurrentProfile("default"); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Active profile set to %q\n", "default")

		fmt.Println("AI-Cabin environment ready.")
		// Show the onboarding hint only when no cabin is registered yet.
		if cabins, err := config.ListCabins(); err == nil && len(cabins) == 0 {
			fmt.Println("Next: register a cabin with `cabin cabin add <path>` and run `cabin task <cabin> <task>`.")
		}
	},
}

func init() {
	rootCmd.AddCommand(setupCmd)
}
