package cabin

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/JulienVdG/AI-Cabin/internal/config"
)

// ValidateCabin validates that path points to a valid cabin directory and
// returns the cabin name and the normalized absolute path to register.
//
// A valid cabin is a directory containing a Taskfile.yml whose top-level
// "ai-cabin:" header exists. (could be empty {})
//
// Name derivation (first non-empty wins):
//  1. nameOverride (explicit "[name]" arg on the CLI)
//  2. ai-cabin.cabin from the Taskfile header
//  3. basename of the normalized path
//
// Path normalization: "~"/"~user" expansion, then made absolute, then
// symlinks resolved. The registry stores this canonical absolute path so
// "cd <path>" at run time never breaks on relative paths, unexpanded "~",
// or symlinks.
//
// Errors are strict: a non-directory path, a missing Taskfile, or a missing
// "ai-cabin:" header all fail registration. This catches typos and
// non-cabin-like paths at registration time rather than at run time.
//
// Convention (non-standard): on error, normalizedPath is still populated when
// known (i.e. once the path has been resolved, before the header check), so
// callers can surface it in UX without re-deriving it. name is always empty
// on error.
//
// See Header for callers that need the parsed ai-cabin header (agents:/
// features:) without re-reading the Taskfile.
func ValidateCabin(path, nameOverride string) (name, normalizedPath string, err error) {
	var name2 string
	_, name2, normalizedPath, err = validateCabin(path, nameOverride)
	if err != nil {
		return "", normalizedPath, err
	}
	return name2, normalizedPath, nil
}

// Header resolves and parses the ai-cabin header of the cabin at path. It is
// the variant of ValidateCabin for callers that consume the header (agents:/
// features: via ActiveBundles) and need the normalized path, without deriving
// the cabin name (e.g. cabin internal deps/setup). It reuses the same path
// normalization and validation as ValidateCabin (one read of the Taskfile).
//
// Returns the parsed header, the normalized absolute path, and an error on
// validation failure (same strict errors + ErrNoHeader sentinel as
// ValidateCabin; normalizedPath is populated when known, by the same
// convention).
func Header(path string) (header *AICabinHeader, normalizedPath string, err error) {
	header, _, normalizedPath, err = validateCabin(path, "")
	return header, normalizedPath, err
}

// validateCabin is the shared core of ValidateCabin and Header: it resolves
// and normalizes the path, reads the Taskfile, parses the ai-cabin header,
// and derives the name. It returns the parsed header (nil for an absent
// ai-cabin: block) alongside the name and normalized path, so each public
// wrapper selects what it needs without re-reading the Taskfile.
func validateCabin(path, nameOverride string) (header *AICabinHeader, name, normalizedPath string, err error) {
	// Expand "~"/"~user" before any FS operation (these are pure string ops).
	expanded := config.ExpandHome(path)

	// Make absolute relative to the process CWD. EvalSymlinks requires an
	// absolute path to behave predictably across platforms.
	absPath, err := filepath.Abs(expanded)
	if err != nil {
		return nil, "", "", fmt.Errorf("resolve absolute path: %w", err)
	}

	// Stat before EvalSymlinks: a missing path should error with a clear
	// "does not exist" rather than EvalSymlinks' loop-error.
	info, err := os.Stat(absPath)
	if err != nil {
		return nil, "", "", fmt.Errorf("cabin path: %w", err)
	}
	if !info.IsDir() {
		return nil, "", "", fmt.Errorf("cabin path %q is not a directory", absPath)
	}

	// Resolve symlinks to a canonical path. The directory itself may be a
	// symlink, and Taskfile.yml inside may sit behind a symlinked parent.
	resolved, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		return nil, "", "", fmt.Errorf("resolve symlinks: %w", err)
	}

	// Read and parse the Taskfile header.
	tfPath := filepath.Join(resolved, TaskfileName)
	data, err := os.ReadFile(tfPath)
	if err != nil {
		return nil, "", "", fmt.Errorf("read %s: %w", TaskfileName, err)
	}
	header, err = ParseHeader(data)
	if err != nil {
		return nil, "", "", fmt.Errorf("%s in %s: %w", TaskfileName, resolved, err)
	}
	if header == nil {
		return nil, "", resolved, ErrNoHeader
	}

	// Derive the name: explicit override > header.Cabin > basename.
	switch {
	case nameOverride != "":
		name = nameOverride
	case header.Cabin != "":
		name = header.Cabin
	default:
		name = filepath.Base(resolved)
	}

	return header, name, resolved, nil
}
