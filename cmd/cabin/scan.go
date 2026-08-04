package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/JulienVdG/AI-Cabin/internal/cabin"
	"github.com/JulienVdG/AI-Cabin/internal/config"

	"github.com/spf13/cobra"
)

// cabinScanCmd recursively discovers cabins under <path> and registers them.
// A directory is a cabin when it contains a Taskfile.yml with an ai-cabin:
// header (cabin.ValidateCabin). Non-cabin directories are skipped silently;
// discovered cabins are registered via the same add flow as `cabin cabin add`
// (idempotent on same path, strict on a conflicting path unless --force).
//
// Use case: onboard a developer by scanning ~/projects and registering every
// cabin in one command.
var cabinScanCmd = &cobra.Command{
	Use:   "scan <path>",
	Short: "Recursively discover and register cabins under a path",
	Long: `Walk <path> recursively and register every directory that is a valid cabin
(contains a Taskfile.yml with an "ai-cabin:" block).

Non-cabin directories are skipped silently. A discovered cabin already
registered with the same path is skipped (idempotent). A discovered cabin
whose name maps to a different path errors out unless --force is given
(same rules as 'cabin cabin add').`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		root := args[0]
		scanCabins(root, os.Stdout, os.Stderr)
	},
}

func init() {
	cabinScanCmd.Flags().BoolVarP(&forceAdd, "force", "f", false,
		"overwrite an existing cabin registered with a different path")
	cabinCmd.AddCommand(cabinScanCmd)
}

// scanCabins walks root and registers every valid cabin directory found. It
// reports progress to stdout (one line per discovered/skipped cabin) and
// exits non-zero if any registration failed (a conflicting path without
// --force). A walk error (unreadable subdirectory) is reported to stderr and
// aborts the scan; per-directory validation errors are skipped silently
// (non-cabin directories are expected and common).
func scanCabins(root string, stdout, stderr io.Writer) {
	expanded, err := expandScanRoot(root)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	var failed bool
	err = filepath.WalkDir(expanded, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			fmt.Fprintf(stderr, "Warning: skipping %q: %v\n", path, walkErr)
			return nil
		}
		if !d.IsDir() {
			return nil
		}

		name, normalized, verr := cabin.ValidateCabin(path, "")
		if verr != nil {
			// Not a cabin (no Taskfile or no ai-cabin: header): skip silently.
			// A non-directory Taskfile path or unreadable file is also a skip
			// here — WalkDir descends into every dir, so most dirs are not cabins.
			return nil
		}

		registered, rerr := registerScanned(name, normalized)
		switch {
		case rerr == nil:
			fmt.Fprintf(stdout, "Discovered %q -> %s%s\n", name, normalized, registered)
		default:
			fmt.Fprintf(stderr, "Error: register %q: %v\n", name, rerr)
			failed = true
		}
		return nil
	})
	if err != nil {
		fmt.Fprintf(stderr, "Error: scan %q: %v\n", expanded, err)
		os.Exit(1)
	}
	if failed {
		os.Exit(1)
	}
}

// registerScanned upserts a discovered cabin, mirroring `cabin cabin add` UX:
// same path -> idempotent skip (no warning, scan is bulk), different path ->
// error unless --force. Returns a suffix describing the outcome (e.g.
// " (updated, was /old)") for the progress line.
func registerScanned(name, normalized string) (string, error) {
	existing, err := config.GetCabin(name)
	switch {
	case err == nil && existing.Path == normalized:
		return " (already registered)", nil
	case err == nil && forceAdd:
		if rerr := config.AddCabin(name, normalized); rerr != nil {
			return "", rerr
		}
		return fmt.Sprintf(" (updated, was %s)", existing.Path), nil
	case err == nil:
		return "", fmt.Errorf("already exists at %s; use --force to overwrite with %s",
			existing.Path, normalized)
	}

	if err := config.AddCabin(name, normalized); err != nil {
		return "", err
	}
	return "", nil
}

// expandScanRoot normalizes the scan root (~ expansion + absolute path) so the
// reported paths and the walk are predictable. Symlink resolution is deferred
// to ValidateCabin per discovered cabin (the root itself need not be canonical).
func expandScanRoot(root string) (string, error) {
	expanded := config.ExpandHome(root)
	abs, err := filepath.Abs(expanded)
	if err != nil {
		return "", fmt.Errorf("resolve scan path: %w", err)
	}
	if info, err := os.Stat(abs); err != nil {
		return "", fmt.Errorf("scan path: %w", err)
	} else if !info.IsDir() {
		return "", fmt.Errorf("scan path %q is not a directory", abs)
	}
	return abs, nil
}
