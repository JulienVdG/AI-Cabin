package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/JulienVdG/AI-Cabin/internal/cabin"
	"github.com/JulienVdG/AI-Cabin/internal/config"
	"github.com/JulienVdG/AI-Cabin/internal/embedded"
	"github.com/JulienVdG/AI-Cabin/internal/fragments"
	"github.com/JulienVdG/AI-Cabin/internal/writestrategy"

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

// buildFragmentLayers builds the merged fragment fallback chain for a cabin:
// the cabin-local override layer (<cabin>/fragments when it exists) layered
// over the configured fragment dirs and the embedded fragments, resolved with
// the given vars. Returns the merged FS and the cabin-local layer path (empty
// when the cabin has none). Shared by the cabin runtime resolution and
// `cabin authoring`.
func buildFragmentLayers(cabinPath string, vars config.Vars) (fs.FS, string, error) {
	cabinLocal := ""
	if cabinPath != "" {
		p := filepath.Join(cabinPath, fragmentsSubdir)
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			cabinLocal = p
		}
	}
	embedFS, err := embedded.Fragments()
	if err != nil {
		return nil, cabinLocal, err
	}
	merged, err := fragments.BuildLayers(vars.FragmentsDirs(), vars.LayerFragmentDirs(), cabinLocal, embedFS)
	if err != nil {
		return nil, cabinLocal, err
	}
	return merged, cabinLocal, nil
}

// resolveCabinFragments resolves the cabin from CWD and builds the fragment
// fallback chain.
// Returns the active bundles, resolved vars, merged FS, and cabin path. The
// caller handles error printing (printValidateError formats ErrNoHeader
// specifically and falls back to a generic message otherwise).
// resolveCabinFragments resolves the active bundles for the cabin at cabinPath
// (header agents:/features:), the resolved vars, and the merged fallback chain.
// Hidden internal commands pass "." (CWD); the task runner passes the
// registered cabin's path so behavior is independent of the caller's CWD.
func resolveCabinFragments(cabinPath string) ([]cabin.FeatureRef, config.Vars, fs.FS, string, error) {
	header, resolvedPath, err := cabin.Header(cabinPath)
	if err != nil {
		return nil, nil, nil, resolvedPath, err
	}
	bundles := cabin.ActiveBundles(header)

	// Resolve vars (profile + env + --var) for template rendering.
	vars, err := config.ResolveVars(profileFlag, cliVars)
	if err != nil {
		return nil, nil, nil, resolvedPath, err
	}

	// Build the fallback chain (conf dirs > cabin-local > embedded).
	merged, _, err := buildFragmentLayers(resolvedPath, vars)
	if err != nil {
		return nil, nil, nil, resolvedPath, err
	}
	return bundles, vars, merged, resolvedPath, nil
}

// composeProjectName resolves the docker compose project name for a cabin: the
// active profile's name and the cabin's canonical name (ai-cabin.cabin header
// > basename), sanitized and joined so two profiles operating the same cabin
// get distinct projects while sharing the image build. A missing profile
// yields the canonical name alone. The returned resolvedPath is populated even
// on error so callers can surface which directory was inspected. Shared by
// runCabinTask (CLI path) and `cabin internal compose-project-name` (standalone
// `task` path).
func composeProjectName(cabinPath string) (name, resolvedPath string, err error) {
	canonical, resolvedPath, err := cabin.ValidateCabin(cabinPath, "")
	if err != nil {
		return "", resolvedPath, err
	}
	profileName := ""
	if p, err := config.GetActiveProfile(profileFlag); err == nil {
		profileName = p.Name
	}
	return cabin.ComposeProjectName(profileName, canonical), resolvedPath, nil
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
		bundles, vars, merged, cabinPath, err := resolveCabinFragments(".")
		if err != nil {
			printValidateError(os.Stderr, err, cabinPath)
			os.Exit(1)
		}

		// Materialize every active bundle's deps facet. Materialize collects
		// all errors (no fail-fast) so the user sees every issue in one run;
		// we aggregate per-bundle and continue across bundles the same way.
		depsDir := filepath.Join(cabinPath, ".deps")
		mat, err := fragments.NewMaterializer(merged, depsManifest, depsDir, vars.AsMap(), writestrategy.TruncateCreator{})
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
		bundles, vars, merged, cabinPath, err := resolveCabinFragments(".")
		if err != nil {
			printValidateError(os.Stderr, err, cabinPath)
			os.Exit(1)
		}

		destBase := vars[config.HomeVar]
		mat, err := fragments.NewMaterializer(merged, setupManifest, destBase, vars.AsMap(), writestrategy.BackupCreator{})
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

// internalGreywallProfileCmd resolves the greywall profile list from the
// cabin's active bundles (header agents:/features:), through the fallback
// chain. It prints the comma-joined list to stdout (e.g.
// "workspace,pi,go,forward-mariadb-3306") so the Taskfile can capture it via
// `env: sh:` on the standalone path. The CLI path (cabin task/cabin up)
// sets GREYWALL_PROFILE on the process directly, short-circuiting the sh: (no
// subprocess). This is the hidden command the wrapper's $GREYWALL_PROFILE
// resolves to when not preset.
var internalGreywallProfileCmd = &cobra.Command{
	Use:   "greywall-profile",
	Short: "Resolve the greywall profile list from the cabin's active bundles",
	Run: func(cmd *cobra.Command, args []string) {
		bundles, vars, merged, cabinPath, err := resolveCabinFragments(".")
		if err != nil {
			printValidateError(os.Stderr, err, cabinPath)
			os.Exit(1)
		}

		profiles, err := fragments.ResolveGreywallProfiles(merged, bundles, vars.AsMap())
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(strings.Join(profiles, ","))
	},
}

// internalComposeProjectNameCmd resolves the docker compose project name for
// the cabin in the current directory (active profile + cabin canonical name).
// It prints the name to stdout so the Taskfile can capture it via `env: sh:` on
// the standalone path. The CLI path (cabin task/cabin up) sets
// COMPOSE_PROJECT_NAME on the process directly, short-circuiting the sh: (no
// subprocess). This is the hidden command the wrapper's $COMPOSE_PROJECT_NAME
// resolves to when not preset — same convergence pattern as greywall-profile.
var internalComposeProjectNameCmd = &cobra.Command{
	Use:   "compose-project-name",
	Short: "Resolve the docker compose project name from the active profile and cabin",
	Run: func(cmd *cobra.Command, args []string) {
		project, resolvedPath, err := composeProjectName(".")
		if err != nil {
			printValidateError(os.Stderr, err, resolvedPath)
			os.Exit(1)
		}
		fmt.Println(project)
	},
}

func init() {
	internalCmd.AddCommand(internalDepsCmd)
	internalCmd.AddCommand(internalSetupCmd)
	internalCmd.AddCommand(internalGreywallProfileCmd)
	internalCmd.AddCommand(internalComposeProjectNameCmd)
	rootCmd.AddCommand(internalCmd)
}
