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
// the default dirs (~/Documents/desk, ~/projects), and creates the profile
// named by --profile (default: default) pointing at them with the minimal desk
// skeleton copied to AI_CABIN_DESK.
//
// The --var global flag reaches it (e.g. a custom desk path), but it carries
// no --skeleton/--force: zero-config means the default profile and the minimal
// desk; customization happens via `cabin profile init`.
var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Zero-config environment bootstrap",
	Long: `Prepare the host-side environment so a cabin can run.

Creates a profile pointed by --profile (default: the default profile with
default dirs + git identity derived from the host) and activates it,
copies the minimal desk skeleton to AI_CABIN_DESK, creates the default workdir
(~/projects), and materializes the shared lifecycle Taskfile to XDG state so
task resolves a cabin's includes at parse time.

Idempotent: re-running with an existing target profile is a no-op (the profile
and desk are not overwritten). Use ` + "`cabin profile init --force`" + ` to overwrite.
Agent-config materialization stays lazy, triggered on the first cabin task.`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		// Lifecycle first: the cabin is unusable without it, so fail fast (and
		// loudly) before touching the profile or desk. A read-only state dir is
		// worked around by redirecting the XDG vars to a writable path.
		//
		// The profile to bootstrap: honor --profile when given, else the
		// zero-config default. setup never reads the current profile from the
		// config file — it always targets this explicit name (and activates it).
		profileName := profileFlag
		if profileName == "" {
			profileName = "default"
		}

		if _, err := ensureLifecycleArtifact(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: materialize lifecycle taskfile: %v\n", err)
			fmt.Fprintln(os.Stderr, "The cabin is unusable without the lifecycle Taskfile.")
			fmt.Fprintln(os.Stderr, "If the state dir is read-only, redirect it to a writable path, e.g.:")
			fmt.Fprintln(os.Stderr, "  export XDG_STATE_HOME=/workspace/dev-state  XDG_CONFIG_HOME=/workspace/dev-config")
			os.Exit(1)
		}

		// Detect a fresh bootstrap vs a no-op re-run (setup has no --force).
		profExists, err := config.ProfileExists(profileName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fresh := !profExists

		// Target profile: self-healing bootstrap. The forced merge
		// (defaults ∪ --var ∪ existing) refills missing structural vars (e.g.
		// a corrupted AI_CABIN_DESK) while preserving user-set keys. The --var
		// global flag sets a custom desk path the same way as
		// `cabin profile init --var AI_CABIN_DESK=...`.
		profile, err := config.InitProfile(profileName, cliVars, true)
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
		// Resolve against the bootstrapped profile (not the current one from
		// config.yaml), so the desk skeleton catalogue is independent of the
		// already-active profile when --profile is omitted.
		resolvedVars, err := config.ResolveVars(profileName, cliVars)
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

		// Activate the target profile only once the environment is in place: a failed
		// bootstrap must never leave (or clobber) the current profile.
		if err := config.SetCurrentProfile(profileName); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Active profile set to %q\n", profileName)

		fmt.Println("AI-Cabin environment ready.")
		// Show the onboarding hint only when no cabin is registered yet.
		if cabins, err := config.ListCabins(); err == nil && len(cabins) == 0 {
			fmt.Println("Next: scan for cabins with `cabin scan <path>`, then `cabin use <name>` and `cabin task <task>`.")
		}
	},
}

func init() {
	rootCmd.AddCommand(setupCmd)
}
