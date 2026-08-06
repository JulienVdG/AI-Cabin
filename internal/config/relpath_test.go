package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRelPath covers the path-shadowing host-side computation: the agent
// launches into a sub-directory inside the sandbox matching the host CWD
// sub-path. Cases use a real tmp tree (t.TempDir) so symlinks resolve as in
// production; RelPath is an os-level function (EvalSymlinks), not fstest.
func TestRelPath(t *testing.T) {
	tmp := t.TempDir()
	workdir := filepath.Join(tmp, "workdir")
	subdir := filepath.Join(workdir, "pkg", "helper")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatalf("mkdir subdir: %v", err)
	}
	sibling := filepath.Join(tmp, "sibling")
	if err := os.MkdirAll(sibling, 0o755); err != nil {
		t.Fatalf("mkdir sibling: %v", err)
	}

	// A symlink inside the workdir pointing to a subdir, so a CWD reached
	// through the symlink resolves to the same relpath as the canonical path.
	linkPath := filepath.Join(workdir, "link-to-pkg")
	if err := os.Symlink(subdir, linkPath); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	// A symlink TO the workdir itself, so workdir is reached through a link
	// (the CWD is a real subdir; the workdir arg is the symlink).
	workdirLink := filepath.Join(tmp, "workdir-link")
	if err := os.Symlink(workdir, workdirLink); err != nil {
		t.Fatalf("symlink workdir: %v", err)
	}

	cases := []struct {
		name     string
		cwd      string
		workdir  string
		wantRel  string
		wantErr  bool
		errMatch string
	}{
		{name: "Root", cwd: workdir, workdir: workdir, wantRel: ""},
		{name: "Subdir", cwd: subdir, workdir: workdir, wantRel: filepath.Join("pkg", "helper")},
		{name: "OutsideWorkdir", cwd: sibling, workdir: workdir, wantErr: true, errMatch: "outside workdir"},
		{name: "AncestorOfWorkdir", cwd: tmp, workdir: workdir, wantErr: true, errMatch: "outside workdir"},
		{name: "SymlinkedCwd", cwd: linkPath, workdir: workdir, wantRel: filepath.Join("pkg", "helper")},
		{name: "SymlinkedWorkdir", cwd: subdir, workdir: workdirLink, wantRel: filepath.Join("pkg", "helper")},
		{name: "MissingWorkdir", cwd: subdir, workdir: filepath.Join(tmp, "nope"), wantErr: true, errMatch: "resolve workdir symlinks"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := RelPath(tc.cwd, tc.workdir)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("RelPath(%q, %q) = %q, want error", tc.cwd, tc.workdir, got)
				}
				if tc.errMatch != "" && !strings.Contains(err.Error(), tc.errMatch) {
					t.Fatalf("RelPath error = %q, want substring %q", err.Error(), tc.errMatch)
				}
				return
			}
			if err != nil {
				t.Fatalf("RelPath(%q, %q) error = %v, want nil", tc.cwd, tc.workdir, err)
			}
			if got != tc.wantRel {
				t.Fatalf("RelPath(%q, %q) = %q, want %q", tc.cwd, tc.workdir, got, tc.wantRel)
			}
		})
	}
}
