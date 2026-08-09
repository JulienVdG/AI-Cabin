package writestrategy_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/JulienVdG/AI-Cabin/internal/writestrategy"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeAll is the minimal driver the strategies share: open via Create, write
// the bytes, close. The Materialize loop in internal/fragments does the same
// (render.Execute / io.Copy then Close); testing the strategies directly avoids
// re-asserting the materialize plumbing and isolates each policy's contract.
func writeAll(t *testing.T, c writestrategy.FileCreator, name, content string) {
	t.Helper()
	w, err := c.Create(name)
	require.NoError(t, err)
	defer w.Close()
	_, err = w.Write([]byte(content))
	require.NoError(t, err)
}

// readTarget reads the destination file, failing the test if it does not exist
// (a missing target is a bug the assertion should catch, not a silent skip).
func readTarget(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(name)
	require.NoError(t, err, "read %q", name)
	return string(b)
}

func TestTruncateCreator(t *testing.T) {
	t.Run("CreatesAbsentFile", func(t *testing.T) {
		// A non-existent target is created with the new content. The parent dir
		// is the caller's responsibility (Materialize mkdir's it); here it exists.
		dest := t.TempDir()
		name := filepath.Join(dest, "out.txt")

		writeAll(t, writestrategy.TruncateCreator{}, name, "hello")

		assert.Equal(t, "hello", readTarget(t, name))
	})

	t.Run("OverwritesExisting", func(t *testing.T) {
		// An existing target is truncated then written: no backup, no append,
		// no permission error (re-opening a 0644 file for write succeeds —
		// the umask-applied FilePerm keeps re-runs idempotent).
		dest := t.TempDir()
		name := filepath.Join(dest, "out.txt")
		require.NoError(t, os.WriteFile(name, []byte("old"), 0o644))

		writeAll(t, writestrategy.TruncateCreator{}, name, "new")

		assert.Equal(t, "new", readTarget(t, name))
		_, statErr := os.Stat(name + writestrategy.BackupSuffix)
		assert.True(t, os.IsNotExist(statErr), "truncate never backs up")
	})

	t.Run("CreatesParentDirsAndNestedFile", func(t *testing.T) {
		// TruncateCreator itself does not mkdir (the Materialize loop does); the
		// contract is "open for write at an existing path". Asserting Create
		// fails on a missing parent documents that boundary so a future caller
		// does not assume mkdir is baked in.
		dest := t.TempDir()
		name := filepath.Join(dest, "sub", "out.txt")

		_, err := writestrategy.TruncateCreator{}.Create(name)
		require.Error(t, err, "Create must not mkdir its parent")
	})
}

func TestBackupCreator(t *testing.T) {
	t.Run("AbsentTargetNoBackup", func(t *testing.T) {
		// First write to a non-existent target: no previous version to back up,
		// so no .cabin-bak is created. The target gets the new content.
		dest := t.TempDir()
		name := filepath.Join(dest, "conf.json")

		writeAll(t, writestrategy.BackupCreator{}, name, `{"v":1}`)

		assert.Equal(t, `{"v":1}`, readTarget(t, name))
		_, statErr := os.Stat(name + writestrategy.BackupSuffix)
		assert.True(t, os.IsNotExist(statErr), "no backup on first write")
	})

	t.Run("NoOpOnIdentical", func(t *testing.T) {
		// Second write with identical content: no-op (no .cabin-bak, target
		// unchanged). Idempotent on re-run — no backup churn. This is the
		// copy-if-different contract that makes re-running setup safe.
		dest := t.TempDir()
		name := filepath.Join(dest, "conf.json")
		c := writestrategy.BackupCreator{}

		writeAll(t, c, name, `{"v":1}`)
		writeAll(t, c, name, `{"v":1}`)

		assert.Equal(t, `{"v":1}`, readTarget(t, name))
		_, statErr := os.Stat(name + writestrategy.BackupSuffix)
		assert.True(t, os.IsNotExist(statErr), "no backup on identical re-run")
	})

	t.Run("BackupOnDiff", func(t *testing.T) {
		// Write once, change content, write again: the previous version is
		// backed up (.cabin-bak), the target has the new content. Single-slot
		// backup: a second diff overwrites the first backup (no history chain).
		dest := t.TempDir()
		name := filepath.Join(dest, "conf.json")
		c := writestrategy.BackupCreator{}

		writeAll(t, c, name, `{"v":1}`)
		writeAll(t, c, name, `{"v":2}`)

		assert.Equal(t, `{"v":2}`, readTarget(t, name))
		assert.Equal(t, `{"v":1}`, readTarget(t, name+writestrategy.BackupSuffix))
	})

	t.Run("DoubleCloseIsNoOp", func(t *testing.T) {
		// Close is idempotent: a second Close returns nil and does not re-commit
		// (which would clobber the backup slot or re-write identical content).
		// Guards against a caller that defers Close alongside an explicit Close.
		dest := t.TempDir()
		name := filepath.Join(dest, "conf.json")

		w, err := writestrategy.BackupCreator{}.Create(name)
		require.NoError(t, err)
		require.NoError(t, w.Close())
		assert.NoError(t, w.Close(), "second Close is a no-op")

		assert.Equal(t, "", readTarget(t, name), "empty buffer -> empty file")
	})
}

func TestBackupWriter_WriteAfterClose(t *testing.T) {
	// Writing to a backupWriter after Close errors: the writer is sealed at
	// Close (content committed), a later Write would otherwise append to a
	// buffer whose content is no longer read by Close — silently dropped data.
	dest := t.TempDir()
	name := filepath.Join(dest, "conf.json")

	w, err := writestrategy.BackupCreator{}.Create(name)
	require.NoError(t, err)
	require.NoError(t, w.Close())

	_, err = w.Write([]byte("late"))
	assert.Error(t, err, "Write after Close must error")
}
