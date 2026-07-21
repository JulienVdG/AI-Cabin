package cabin_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/JulienVdG/AI-Cabin/internal/cabin"
)

// writeTaskfile writes a Taskfile.yml with the given ai-cabin content into dir.
func writeTaskfile(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, cabin.TaskfileName), []byte(content), 0o644); err != nil {
		t.Fatalf("write Taskfile: %v", err)
	}
}

func TestValidateCabin(t *testing.T) {
	// --- Happy paths (named sub-cases, distinct setup each) ---

	t.Run("header name used by default", func(t *testing.T) {
		dir := t.TempDir()
		writeTaskfile(t, dir, validTaskfile)

		name, path, err := cabin.ValidateCabin(dir, "")
		if err != nil {
			t.Fatalf("ValidateCabin error = %v", err)
		}
		if name != "blog" {
			t.Errorf("name = %q, want %q (from header)", name, "blog")
		}
		if path != dir {
			t.Errorf("path = %q, want %q (already absolute, no symlinks)", path, dir)
		}
	})

	t.Run("explicit name override takes precedence", func(t *testing.T) {
		dir := t.TempDir()
		writeTaskfile(t, dir, validTaskfile)

		name, _, err := cabin.ValidateCabin(dir, "custom-name")
		if err != nil {
			t.Fatalf("ValidateCabin error = %v", err)
		}
		if name != "custom-name" {
			t.Errorf("name = %q, want %q (override)", name, "custom-name")
		}
	})

	t.Run("basename fallback when cabin field absent", func(t *testing.T) {
		dir := t.TempDir()
		writeTaskfile(t, dir, `ai-cabin:
  agents: [pi]
`)
		name, _, err := cabin.ValidateCabin(dir, "")
		if err != nil {
			t.Fatalf("ValidateCabin error = %v", err)
		}
		if want := filepath.Base(dir); name != want {
			t.Errorf("name = %q, want %q (dir basename)", name, want)
		}
	})

	t.Run("empty ai-cabin map falls back to basename", func(t *testing.T) {
		// Header present as an empty map (`ai-cabin: {}`): still a valid cabin
		// — the block exists, the name just falls back to the basename.
		// (Note: `ai-cabin:` with nothing under it is YAML null, indistinguishable
		// from an absent block in yaml.v3; use `{}` to declare an empty header.)
		dir := t.TempDir()
		writeTaskfile(t, dir, `ai-cabin: {}
version: "3"
`)
		name, _, err := cabin.ValidateCabin(dir, "")
		if err != nil {
			t.Fatalf("ValidateCabin error = %v", err)
		}
		if want := filepath.Base(dir); name != want {
			t.Errorf("name = %q, want %q (dir basename, empty header)", name, want)
		}
	})

	t.Run("relative path resolved against CWD", func(t *testing.T) {
		dir := t.TempDir()
		writeTaskfile(t, dir, validTaskfile)
		parent := filepath.Dir(dir)
		base := filepath.Base(dir)

		prevWD, err := os.Getwd()
		if err != nil {
			t.Fatalf("getwd: %v", err)
		}
		defer func() { _ = os.Chdir(prevWD) }()
		if err := os.Chdir(parent); err != nil {
			t.Fatalf("chdir: %v", err)
		}

		name, path, err := cabin.ValidateCabin(base, "")
		if err != nil {
			t.Fatalf("ValidateCabin error = %v", err)
		}
		if name != "blog" {
			t.Errorf("name = %q, want %q", name, "blog")
		}
		if path != dir {
			t.Errorf("path = %q, want %q (absolute, normalized)", path, dir)
		}
	})

	t.Run("symlink resolved to real dir", func(t *testing.T) {
		realDir := t.TempDir()
		writeTaskfile(t, realDir, validTaskfile)
		linkDir := filepath.Join(t.TempDir(), "link")
		if err := os.Symlink(realDir, linkDir); err != nil {
			t.Skipf("symlink creation failed (platform restriction?): %v", err)
		}

		name, path, err := cabin.ValidateCabin(linkDir, "")
		if err != nil {
			t.Fatalf("ValidateCabin error = %v", err)
		}
		if name != "blog" {
			t.Errorf("name = %q, want %q", name, "blog")
		}
		if path != realDir {
			t.Errorf("path = %q, want %q (symlink resolved)", path, realDir)
		}
	})

	t.Run("tilde expanded via HOME", func(t *testing.T) {
		// Option F: $HOME takes precedence, so t.Setenv("HOME", ...) works
		// without needing osuser.Current() to read env.
		home := t.TempDir()
		t.Setenv("HOME", home)

		cabinDir := filepath.Join(home, "projects", "blog")
		if err := os.MkdirAll(cabinDir, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		writeTaskfile(t, cabinDir, validTaskfile)

		name, path, err := cabin.ValidateCabin("~/projects/blog", "")
		if err != nil {
			t.Fatalf("ValidateCabin error = %v", err)
		}
		if name != "blog" {
			t.Errorf("name = %q, want %q", name, "blog")
		}
		if path != cabinDir {
			t.Errorf("path = %q, want %q (~ expanded)", path, cabinDir)
		}
	})

	// --- Error cases (table-driven: same setup shape, vary input/expectation) ---

	errCases := []struct {
		name   string
		setup  func(t *testing.T) string // returns the path to validate
		needle string                    // substring expected in the error
	}{
		{
			name:  "path does not exist",
			setup: func(t *testing.T) string { return "/nonexistent/path/that/should/not/exist" },
			// OS-dependent wording: Linux gives "no such file or directory",
			// other platforms may say "does not exist".
			needle: "no such file",
		},
		{
			name: "path is a file not a directory",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				file := filepath.Join(dir, "notadir.txt")
				if err := os.WriteFile(file, []byte("hi"), 0o644); err != nil {
					t.Fatalf("write file: %v", err)
				}
				return file
			},
			needle: "not a directory",
		},
		{
			name: "directory has no Taskfile",
			setup: func(t *testing.T) string {
				return t.TempDir() // empty dir, no Taskfile.yml
			},
			needle: cabin.TaskfileName,
		},
	}

	for _, tc := range errCases {
		t.Run(tc.name, func(t *testing.T) {
			path := tc.setup(t)
			_, _, err := cabin.ValidateCabin(path, "")
			if err == nil {
				t.Fatal("ValidateCabin error = nil, want error")
			}
			if !strings.Contains(err.Error(), tc.needle) {
				t.Errorf("error = %v, want it to contain %q", err, tc.needle)
			}
		})
	}

	// Dedicated sentinel check: a Taskfile without the ai-cabin: block must
	// return ErrNoHeader (not a wrapped string match), so callers can branch
	// via errors.Is and surface richer UX (snippet, guidance). Also validates
	// the convention that normalizedPath is still populated on this error
	// (valid once the path is resolved, before the header check), so the UX
	// layer can show which directory was inspected.
	t.Run("missing ai-cabin block returns ErrNoHeader sentinel", func(t *testing.T) {
		dir := t.TempDir()
		writeTaskfile(t, dir, `version: "3"
tasks:
  pi:
    cmds: ["echo hi"]
`)
		name, resolved, err := cabin.ValidateCabin(dir, "")
		if err == nil {
			t.Fatal("ValidateCabin error = nil, want ErrNoHeader")
		}
		if !errors.Is(err, cabin.ErrNoHeader) {
			t.Errorf("error = %v, want errors.Is(err, ErrNoHeader)", err)
		}
		// name is always empty on error.
		if name != "" {
			t.Errorf("name = %q, want empty on error", name)
		}
		// normalizedPath is still populated so the UX can show which dir was
		// inspected (convention documented on ValidateCabin).
		if resolved != dir {
			t.Errorf("normalizedPath = %q, want %q (populated even on error)", resolved, dir)
		}
	})
}
