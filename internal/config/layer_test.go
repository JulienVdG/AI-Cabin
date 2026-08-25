package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/JulienVdG/AI-Cabin/internal/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeLayerFile writes one file (layer.yaml or another) under a layer root.
func writeLayerFile(t *testing.T, root, name, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(root, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, name), []byte(content), 0o644))
}

// TestLayerVars covers the layer.yaml vars: contribution: first layer with a
// manifest wins (file-level, no merge), later layers ignored, absent manifest
// contributes nothing, and a malformed present manifest is an error.
func TestLayerVars(t *testing.T) {
	t.Run("NoLayersReturnsEmpty", func(t *testing.T) {
		out, err := config.LayerVars(nil)
		require.NoError(t, err)
		assert.Empty(t, out)
	})

	t.Run("FirstManifestWinsNoMerge", func(t *testing.T) {
		// Two layers both carry layer.yaml; only the first contributes (its
		// whole vars: block), the second is ignored — file-level first-wins.
		a := t.TempDir()
		b := t.TempDir()
		writeLayerFile(t, a, "layer.yaml", "vars:\n  A: from-a\n  SHARED: a\n")
		writeLayerFile(t, b, "layer.yaml", "vars:\n  B: from-b\n  SHARED: b\n")

		out, err := config.LayerVars([]string{a, b})
		require.NoError(t, err)
		assert.Equal(t, config.Vars{"A": "from-a", "SHARED": "a"}, out)
	})

	t.Run("AbsentManifestSkipped", func(t *testing.T) {
		// A layer root with no layer.yaml contributes nothing and is skipped;
		// a later layer with a manifest contributes. This mirrors a
		// fragments-only layer preceding one that ships a manifest.
		a := t.TempDir() // no layer.yaml
		b := t.TempDir()
		writeLayerFile(t, b, "layer.yaml", "vars:\n  DEFAULT_MODEL: Qwen3\n")

		out, err := config.LayerVars([]string{a, b})
		require.NoError(t, err)
		assert.Equal(t, config.Vars{"DEFAULT_MODEL": "Qwen3"}, out)
	})

	t.Run("EmptyVarsBlockStillCaptured", func(t *testing.T) {
		// The first layer with a manifest captures the contribution even when
		// its vars: block is empty (later manifests still ignored).
		a := t.TempDir()
		b := t.TempDir()
		writeLayerFile(t, a, "layer.yaml", "vars: {}\n")
		writeLayerFile(t, b, "layer.yaml", "vars:\n  B: ignored\n")

		out, err := config.LayerVars([]string{a, b})
		require.NoError(t, err)
		assert.Empty(t, out, "first manifest has no vars; later layers must not contribute")
	})

	t.Run("MalformedManifestIsError", func(t *testing.T) {
		a := t.TempDir()
		writeLayerFile(t, a, "layer.yaml", "vars: [not-a-map\n")

		_, err := config.LayerVars([]string{a})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "layer manifest")
	})
}
