package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/JulienVdG/AI-Cabin/internal/cabin"
	"github.com/JulienVdG/AI-Cabin/internal/config"
	"github.com/JulienVdG/AI-Cabin/internal/embedded"
	"github.com/JulienVdG/AI-Cabin/internal/fragments"

	"github.com/spf13/cobra"
)

// internalCmd groups internal commands invoked by Taskfile targets (not by
// users directly). Hidden from the default help; still runnable for debugging.
var internalCmd = &cobra.Command{
	Use:   "internal",
	Short: "Internal commands invoked by Taskfile targets",
	Long: `Internal commands invoked by Taskfile targets via $AI_CABIN_CMD self-delegation
(they are hidden from the default help). Run them from a cabin directory: one
containing a Taskfile.yml with an ai-cabin: header. They resolve the cabin
from the current working directory (no <cabin> arg).`,
	Hidden: true,
}

// depsManifest is the manifest name for the deps facet, materialized into
// <cabin>/.deps/ for the Docker build context.
const depsManifest = "deps.yaml"

// setupManifest is the manifest name for the setup facet, materialized into
// $AI_CABIN_HOME (persistent agent configs: pi/opencode settings, models,
// greywall profiles). Copy strategy is BackupCreator (copy-if-different).
const setupManifest = "setup.yaml"

// fragmentsSubdir is the cabin-local override layer subdir. A cabin opts into
// fragment overrides by creating <cabin>/fragments/; absent means no
// cabin-local layer (the dev layer is optional). Kept under a subdir (not the
// cabin root) so cabin files (Taskfile, Dockerfile, compose) cannot accidentally
// shadow an embedded fragment path, and the override layout mirrors the
// embedded root/fragments/ tree.
const fragmentsSubdir = "fragments"

// resolveCabinFragments resolves the cabin from CWD and builds the fragment
// fallback chain.
// Returns the active bundles, resolved vars, merged FS, and cabin path. The
// caller handles error printing (printValidateError formats ErrNoHeader
// specifically and falls back to a generic message otherwise).
func resolveCabinFragments() ([]cabin.FeatureRef, config.Vars, fs.FS, string, error) {
	// Resolve the cabin from CWD to derive active bundles.
	header, cabinPath, err := cabin.Header(".")
	if err != nil {
		return nil, nil, nil, cabinPath, err
	}
	bundles := cabin.ActiveBundles(header)

	// Resolve vars (profile + env + --var) for template rendering.
	vars, err := config.ResolveVars(profileFlag, cliVars)
	if err != nil {
		return nil, nil, nil, cabinPath, err
	}

	// Cabin-local override layer: <cabin>/fragments/ if it exists.
	cabinLocal := ""
	fragmentsDir := filepath.Join(cabinPath, fragmentsSubdir)
	if info, err := os.Stat(fragmentsDir); err == nil && info.IsDir() {
		cabinLocal = fragmentsDir
	}

	// Build the fallback chain (conf dirs > cabin-local > embedded).
	embedFS, err := embedded.Fragments()
	if err != nil {
		return nil, nil, nil, cabinPath, err
	}
	merged, err := fragments.BuildLayers(vars.FragmentsDirs(), cabinLocal, embedFS)
	if err != nil {
		return nil, nil, nil, cabinPath, err
	}
	return bundles, vars, merged, cabinPath, nil
}

// internalDepsCmd materializes <cabin>/.deps/ from the active bundles declared
// in the cabin's Taskfile header (agents:/features:), resolved through the
// fallback chain (AI_CABIN_FRAGMENTS_DIRS > <cabin>/fragments/ > embedded).
//
// Invoked from the Taskfile `deps` target via $AI_CABIN_CMD self-delegation:
// the task runs in the cabin dir, so this command resolves the cabin
// from CWD (no <cabin> arg). Host-specific binaries the CLI cannot ship
// (greywall bin, greyproxy-ca.crt) are still copied by the Taskfile target
// around this call.
var internalDepsCmd = &cobra.Command{
	Use:   "deps",
	Short: "Materialize <cabin>/.deps/ from active bundles",
	Run: func(cmd *cobra.Command, args []string) {
		bundles, vars, merged, cabinPath, err := resolveCabinFragments()
		if err != nil {
			printValidateError(os.Stderr, err, cabinPath)
			os.Exit(1)
		}

		// Materialize every active bundle's deps facet. Materialize collects
		// all errors (no fail-fast) so the user sees every issue in one run;
		// we aggregate per-bundle and continue across bundles the same way.
		depsDir := filepath.Join(cabinPath, ".deps")
		mat, err := fragments.NewMaterializer(merged, depsManifest, depsDir, vars.AsMap(), fragments.TruncateCreator{})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		var written []string
		var aggErr error
		for _, b := range bundles {
			w, err := mat.Materialize(b.Name, b.Attrs)
			written = append(written, w...)
			if err != nil {
				aggErr = errors.Join(aggErr, fmt.Errorf("bundle %q: %w", b.Name, err))
			}
		}

		fmt.Printf("Materialized %d fragments into %s/.deps/ for %d bundle(s)\n",
			len(written), cabinPath, len(bundles))
		for _, w := range written {
			fmt.Printf("  %s\n", w)
		}

		if aggErr != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", aggErr)
			os.Exit(1)
		}
	},
}

// internalSetupCmd materializes agent configs into $AI_CABIN_HOME from the
// active bundles' setup facet, resolved through the fallback chain. Uses
// BackupCreator (copy-if-different + backup) since the destination is
// persistent (and may be a shared global config when AI_CABIN_HOME=$HOME).
var internalSetupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Materialize agent configs into $AI_CABIN_HOME from active bundles",
	Run: func(cmd *cobra.Command, args []string) {
		bundles, vars, merged, cabinPath, err := resolveCabinFragments()
		if err != nil {
			printValidateError(os.Stderr, err, cabinPath)
			os.Exit(1)
		}

		destBase := vars[config.HomeVar]
		mat, err := fragments.NewMaterializer(merged, setupManifest, destBase, vars.AsMap(), fragments.BackupCreator{})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		var written []string
		var aggErr error
		for _, b := range bundles {
			w, err := mat.Materialize(b.Name, b.Attrs)
			written = append(written, w...)
			if err != nil {
				aggErr = errors.Join(aggErr, fmt.Errorf("bundle %q: %w", b.Name, err))
			}
		}

		fmt.Printf("Materialized %d fragments into %s for %d bundle(s)\n",
			len(written), destBase, len(bundles))
		for _, w := range written {
			fmt.Printf("  %s\n", w)
		}

		if aggErr != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", aggErr)
			os.Exit(1)
		}
	},
}

func init() {
	internalCmd.AddCommand(internalDepsCmd)
	internalCmd.AddCommand(internalSetupCmd)
	rootCmd.AddCommand(internalCmd)
}
