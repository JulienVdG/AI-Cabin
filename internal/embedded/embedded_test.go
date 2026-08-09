package embedded_test

import (
	"io/fs"
	"path"
	"sort"
	"strings"
	"testing"

	"github.com/JulienVdG/AI-Cabin/internal/embedded"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// expectedBundles is the public contract: the bundle names referenced by
// cabin.ActiveBundles (base, git-agent, go, agent-pi, agent-opencode) plus
// port-forward (the declarative service-forwarding bundle, one instance per
// forwarded service). A rename
// here must break a test rather than silently orphan a bundle.
var expectedBundles = []string{"agent-opencode", "agent-pi", "base", "git-agent", "go", "port-forward"}

// mirrorBundles are the bundles that mirror their deps/ subtree (the v1 default
// layout). port-forward is excluded: it uses explicit entries: with templated
// dst (one bundle instance per forwarded service), so it legitimately
// does not declare mirror: deps/.
var mirrorBundles = []string{"agent-opencode", "agent-pi", "base", "git-agent", "go"}

// TestFragments verifies the embedded base layer exposes the expected bundles
// and that the structure is self-consistent, without hardcoding the full file
// list (which would require manual edits on every added fragment). It walks
// the FS so a deleted/renamed file is caught by the walk itself rather than by
// a stale expected-path list.
func TestFragments(t *testing.T) {
	fsys, err := embedded.Fragments()
	require.NoError(t, err)

	t.Run("BundleRootsMatchContract", func(t *testing.T) {
		entries, err := fs.ReadDir(fsys, ".")
		require.NoError(t, err)
		var names []string
		for _, e := range entries {
			assert.True(t, e.IsDir(), "root entry %q is not a directory", e.Name())
			names = append(names, e.Name())
		}
		sort.Strings(names)
		assert.Equal(t, expectedBundles, names)
	})

	t.Run("EveryBundleHasDepsManifest", func(t *testing.T) {
		// Each bundle's deps.yaml is the manifest Materialize reads for the
		// deps facet; it must exist for every bundle (the build facet always
		// contributes). setup.yaml is optional (a bundle may have no setup
		// facet — e.g. git-agent, go, port-forward).
		for _, b := range expectedBundles {
			t.Run(b, func(t *testing.T) {
				_, err := fs.ReadFile(fsys, b+"/deps.yaml")
				require.NoError(t, err, "bundle %q must have a deps.yaml manifest", b)
			})
		}
	})

	t.Run("MirrorBundlesDeclareMirrorDeps", func(t *testing.T) {
		// The v1 default bundles all mirror their deps/ subtree. Asserting the
		// exact content catches a manifest rewritten to entries: by mistake.
		// port-forward is excluded (it uses explicit entries: with templated dst,
		// one instance per forwarded service) and is covered by its own Materialize
		// test in internal/fragments.
		for _, b := range mirrorBundles {
			t.Run(b, func(t *testing.T) {
				data, err := fs.ReadFile(fsys, b+"/deps.yaml")
				require.NoError(t, err)
				assert.Equal(t, "mirror: deps/\n", string(data),
					"bundle %q deps.yaml must declare mirror: deps/", b)
			})
		}
	})

	t.Run("EveryFileIsReadableAndUnderBundleFacet", func(t *testing.T) {
		// Walking catches a deleted or renamed fragment without a stale path
		// list: any ReadFile error fails here. It also asserts the on-disk
		// shape (every file lives under <bundle>/{deps,setup}/, with the
		// {deps,setup}.yaml manifests at the bundle root — no stray files, no
		// leaked cabin/ subtree).
		var count int
		err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			count++
			bundle, rest, ok := splitFirst(p)
			require.True(t, ok, "file %q is not under a bundle dir", p)
			assert.Contains(t, expectedBundles, bundle, "file %q under unknown bundle %q", p, bundle)
			switch rest {
			case "deps.yaml", "setup.yaml":
				// Manifests sit at the bundle root.
			default:
				top, _, _ := strings.Cut(rest, "/")
				assert.Contains(t, []string{"deps", "setup"}, top,
					"file %q is not a manifest and not under deps/ or setup/", p)
			}
			_, rerr := fs.ReadFile(fsys, p)
			require.NoError(t, rerr, "fragment %q is not readable", p)
			return nil
		})
		require.NoError(t, err)
		assert.Greater(t, count, 0, "walked zero files — embed directive or root/ is broken")
	})
}

// splitFirst splits "a/b/c" into ("a", "b/c"). A bare "a" returns ok=false
// (no separator — the path is a bundle, not a file under one).
func splitFirst(p string) (first, rest string, ok bool) {
	// WalkDir yields "." for the root, skip it.
	if p == "." {
		return "", "", false
	}
	idx := -1
	for i := 0; i < len(p); i++ {
		if p[i] == '/' {
			idx = i
			break
		}
	}
	if idx == -1 {
		return p, "", false
	}
	return p[:idx], p[idx+1:], true
}

func TestState(t *testing.T) {
	// State() ships the shared lifecycle Taskfile the cabins include. The
	// docker-* task names are the contract `cabin up <cabin>` relies on:
	// dropping one breaks every cabin's lifecycle targets.
	fsys, err := embedded.State()
	require.NoError(t, err)

	data, err := fs.ReadFile(fsys, "Taskfile.lifecycle.yml")
	require.NoError(t, err)
	content := string(data)
	for _, task := range []string{
		"docker-up", "docker-down", "docker-build",
		"docker-shell", "docker-greyshell", "docker-logs", "docker-restart",
	} {
		assert.Contains(t, content, task, "lifecycle Taskfile missing the %q target", task)
	}
}

// expectedDesks is the public contract for the embedded desk skeletons. The
// `minimal` desk skeleton is the zero-config default of `cabin setup` (copied
// to AI_CABIN_DESK); a rename here breaks a test rather than silently
// orphaning a skeleton.
var expectedDesks = []string{"minimal"}

// minimalDeskFiles are the files the minimal desk skeleton must ship: the base
// agent rules, a minimal TODO, the retro process doc, and the retro-process
// skill. `cabin setup` copies this whole tree to AI_CABIN_DESK. The list is
// exhaustive: a stray file (.swp, .DS_Store, ...) breaks the
// MinimalDeskHasNoStrayFiles sub-case rather than being silently embedded.
var minimalDeskFiles = []string{
	"AGENTS.md",
	"TODO.md",
	"retro.md",
	"skills/retro-process/SKILL.md",
}

func TestSkeletons(t *testing.T) {
	// Skeletons() ships the embedded Class 1 scaffolding trees, typed by
	// concern. `desks/minimal/` is the contract `cabin setup` / `cabin profile
	// init` (default skeleton) relies on: dropping a file breaks the
	// zero-config onboarding.
	fsys, err := embedded.Skeletons()
	require.NoError(t, err)

	t.Run("DesksRootMatchesContract", func(t *testing.T) {
		entries, err := fs.ReadDir(fsys, "desks")
		require.NoError(t, err)
		var names []string
		for _, e := range entries {
			assert.True(t, e.IsDir(), "desks entry %q is not a directory", e.Name())
			names = append(names, e.Name())
		}
		sort.Strings(names)
		assert.Equal(t, expectedDesks, names)
	})

	t.Run("MinimalDeskShipsRequiredFiles", func(t *testing.T) {
		for _, f := range minimalDeskFiles {
			p := path.Join("desks", "minimal", f)
			_, err := fs.ReadFile(fsys, p)
			require.NoError(t, err, "minimal desk missing required file %q", p)
		}
	})

	t.Run("MinimalDeskHasNoStrayFiles", func(t *testing.T) {
		// Walk the minimal desk and assert every file is in minimalDeskFiles.
		// Catches a stray editor swap file (.swp) or OS file (.DS_Store) that
		// //go:embed all:root would silently ship — a clean tree is the contract.
		want := make(map[string]bool, len(minimalDeskFiles))
		for _, f := range minimalDeskFiles {
			want[f] = true
		}
		var stray []string
		err := fs.WalkDir(fsys, "desks/minimal", func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			rel := strings.TrimPrefix(p, "desks/minimal/")
			if !want[rel] {
				stray = append(stray, rel)
			}
			return nil
		})
		require.NoError(t, err)
		assert.Empty(t, stray, "minimal desk has stray files not in the contract: %v", stray)
	})
}
