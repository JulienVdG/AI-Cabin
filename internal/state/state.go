// Package state materializes embedded runtime artifacts to XDG state
// ($XDG_STATE_HOME/ai-cabin). State artifacts are cross-cabin files (e.g. the
// shared lifecycle Taskfile) the CLI writes to disk before `task` parses a
// cabin Taskfile: an includes: entry resolves at parse time, before any task
// runs, so it cannot be produced by the deps task.
package state

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/JulienVdG/AI-Cabin/internal/config"
)

// EnsureArtifact materializes an embedded state artifact (srcFS/<name>) to XDG
// state (config.GetStateDir()/<name>) if it is missing or stale (its bytes
// differ from the embedded source). Idempotent: a file already up to date is a
// no-op (no write, no mtime churn). Returns the absolute destination path.
//
// The on-disk copy is state, not config: overwriting a user edit is
// acceptable (the file is regenerated from the embedded source on each CLI
// version that ships different content). srcFS is injected (typically
// embedded.State()) so the function is unit-testable via testing/fstest.MapFS
// without touching the real embed.FS.
func EnsureArtifact(srcFS fs.FS, name string) (string, error) {
	src, err := fs.ReadFile(srcFS, name)
	if err != nil {
		return "", fmt.Errorf("read embedded state artifact %q: %w", name, err)
	}

	stateDir, err := config.GetStateDir()
	if err != nil {
		return "", err
	}
	dest := filepath.Join(stateDir, name)

	// Up to date: identical bytes -> no-op (idempotent, no mtime churn).
	if existing, rerr := os.ReadFile(dest); rerr == nil && bytes.Equal(existing, src) {
		return dest, nil
	}

	// Missing or stale: (re)write. mkdir the parent for namespaced artifacts.
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return "", fmt.Errorf("create state dir for %q: %w", name, err)
	}
	if err := os.WriteFile(dest, src, 0o644); err != nil {
		return "", fmt.Errorf("write state artifact %q: %w", name, err)
	}
	return dest, nil
}
