package embedded_test

import (
	"io/fs"
	"sort"
	"strings"
	"testing"

	"github.com/JulienVdG/AI-Cabin/internal/embedded"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// expectedBundles is the public contract: the bundle names referenced by
// cabin.ActiveBundles (base, git-agent, go, agent-pi, agent-opencode). A
// rename here must break a test rather than silently orphan a bundle.
var expectedBundles = []string{"agent-opencode", "agent-pi", "base", "git-agent", "go"}

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

	t.Run("EveryBundleHasMirrorDepsManifest", func(t *testing.T) {
		// Each bundle's deps.yaml is the manifest Materialize reads for the
		// deps facet; it must exist and declare the simple "mirror: deps/"
		// form (the v1 default bundles all mirror their deps/ subtree).
		// Asserting the exact content catches a manifest rewritten to
		// entries: by mistake. setup.yaml is optional (a bundle may have no
		// setup facet — e.g. git-agent, go); deps.yaml is required for every
		// bundle (the build facet always contributes).
		for _, b := range expectedBundles {
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
