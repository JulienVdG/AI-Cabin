package config

import (
	"fmt"
	"path/filepath"
	"strings"
)

// RelPath computes the sub-path of cwd relative to workdir, for the
// CABIN_REL_PATH container env var (path shadowing: the host CWD sub-path the
// agent launches into inside the greywall sandbox, via the two-step cd). Returns
// "" when cwd is workdir itself (the agent launches at the workdir root).
// Returns an error when cwd is outside workdir: the host-side validation that
// prevents the container-side cd from landing in the wrong directory
// (fail-fast, no silent fallback to the root — a fallback would make the
// agent run in the wrong dir while the user believes it is in the sub-path).
//
// Symlinks in both paths are resolved (EvalSymlinks) so a CWD reached through
// a symlink of the workdir resolves to the same sub-path as the canonical
// form. This makes transparent mode (CONTAINER_WORKDIR=AI_CABIN_WORKDIR) and
// remap mode (=/workspace) produce the same relpath, since relpath is relative
// to the workdir root in both cases.
func RelPath(cwd, workdir string) (string, error) {
	cwdReal, err := filepath.EvalSymlinks(cwd)
	if err != nil {
		return "", fmt.Errorf("resolve cwd symlinks %q: %w", cwd, err)
	}
	workReal, err := filepath.EvalSymlinks(workdir)
	if err != nil {
		return "", fmt.Errorf("resolve workdir symlinks %q: %w", workdir, err)
	}

	rel, err := filepath.Rel(workReal, cwdReal)
	if err != nil {
		return "", fmt.Errorf("cwd %q is not relative to workdir %q: %w", cwdReal, workReal, err)
	}
	if rel == "." {
		return "", nil
	}
	// filepath.Rel produces a ".."-prefixed path when cwd is outside workdir
	// (sibling or ancestor), or an absolute path on volume mismatch. Reject
	// both rather than inject a path the container-side cd would resolve
	// outside the sandbox.
	if strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("cwd %q is outside workdir %q", cwdReal, workReal)
	}
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("cwd %q is on a different volume than workdir %q", cwdReal, workReal)
	}
	return rel, nil
}
