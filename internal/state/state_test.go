package state_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/JulienVdG/AI-Cabin/internal/config"
	"github.com/JulienVdG/AI-Cabin/internal/state"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setStateDir points XDG_STATE_HOME at a temp dir, mkdirs the ai-cabin subdir,
// and returns the resolved state dir (GetStateDir). Tests write/read real
// files under it.
func setStateDir(t *testing.T) string {
	t.Helper()
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	d, err := config.GetStateDir()
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(d, 0o755))
	return d
}

func TestEnsureArtifact(t *testing.T) {
	const name = "Taskfile.lifecycle.yml"
	src := []byte("version: '3'\ntasks:\n  docker-up:\n    cmds:\n      - echo up\n")

	t.Run("MissingWritesFile", func(t *testing.T) {
		stateDir := setStateDir(t)
		srcFS := fstest.MapFS{name: {Data: src}}

		dest, err := state.EnsureArtifact(srcFS, name)
		require.NoError(t, err)

		assert.Equal(t, filepath.Join(stateDir, name), dest)
		got, err := os.ReadFile(dest)
		require.NoError(t, err)
		assert.Equal(t, src, got)
	})

	t.Run("IdenticalIsNoOp", func(t *testing.T) {
		// A file already matching the embedded source must not be rewritten
		// (idempotent re-run, no mtime churn).
		stateDir := setStateDir(t)
		dest := filepath.Join(stateDir, name)
		require.NoError(t, os.WriteFile(dest, src, 0o644))
		info, err := os.Stat(dest)
		require.NoError(t, err)
		mtime := info.ModTime()

		srcFS := fstest.MapFS{name: {Data: src}}
		got, err := state.EnsureArtifact(srcFS, name)
		require.NoError(t, err)
		assert.Equal(t, dest, got)

		info2, err := os.Stat(dest)
		require.NoError(t, err)
		assert.Equal(t, mtime, info2.ModTime(), "identical file must not be rewritten")
	})

	t.Run("StaleOverwrites", func(t *testing.T) {
		// A file whose bytes differ from the embedded source is regenerated.
		stateDir := setStateDir(t)
		dest := filepath.Join(stateDir, name)
		require.NoError(t, os.WriteFile(dest, []byte("STALE CONTENT"), 0o644))

		srcFS := fstest.MapFS{name: {Data: src}}
		got, err := state.EnsureArtifact(srcFS, name)
		require.NoError(t, err)
		assert.Equal(t, dest, got)

		gotBytes, err := os.ReadFile(dest)
		require.NoError(t, err)
		assert.Equal(t, src, gotBytes, "stale file must be overwritten with the embedded source")
	})

	t.Run("MissingSourceErrors", func(t *testing.T) {
		// srcFS without the named artifact: strict error wrapping fs.ErrNotExist.
		setStateDir(t)
		srcFS := fstest.MapFS{}

		_, err := state.EnsureArtifact(srcFS, name)
		require.Error(t, err)
		assert.ErrorIs(t, err, fs.ErrNotExist)
		assert.Contains(t, err.Error(), name)
	})

	t.Run("CreatesParentDir", func(t *testing.T) {
		// A namespaced name (future state artifacts may be namespaced): the
		// parent dir is created if missing.
		const subName = "sub/dir/artifact.txt"
		stateDir := setStateDir(t)
		srcFS := fstest.MapFS{subName: {Data: src}}

		dest, err := state.EnsureArtifact(srcFS, subName)
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(stateDir, subName), dest)
		got, err := os.ReadFile(dest)
		require.NoError(t, err)
		assert.Equal(t, src, got)
	})
}
