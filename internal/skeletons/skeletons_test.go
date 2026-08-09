package skeletons_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"testing/fstest"

	"github.com/JulienVdG/AI-Cabin/internal/render"
	"github.com/JulienVdG/AI-Cabin/internal/skeletons"
	"github.com/JulienVdG/AI-Cabin/internal/writestrategy"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// deskSkeleton is a representative Class 1 skeleton: a plain AGENTS.md, a
// rendered config.tmpl, and a nested skill file. Used across sub-cases so the
// tree shape (flat + nested + .tmpl) is asserted once and reused.
func deskSkeleton() fstest.MapFS {
	return fstest.MapFS{
		"minimal/AGENTS.md":                     {Data: []byte("# Agent rules\n")},
		"minimal/TODO.md":                       {Data: []byte("# Tasks\n")},
		"minimal/config.tmpl":                   {Data: []byte("desk={{.Vars.AI_CABIN_DESK}}\n")},
		"minimal/skills/retro/SKILL.md":         {Data: []byte("# Retro\n")},
		"minimal/skills/retro/rendered.md.tmpl": {Data: []byte("home={{.Vars.AI_CABIN_HOME}}\n")},
	}
}

// readDest reads a destination file relative to dest, failing the test if it
// does not exist (a missing target is a bug the assertion should catch).
func readDest(t *testing.T, dest, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dest, rel))
	require.NoError(t, err, "read %q", rel)
	return string(b)
}

// destExists reports whether a destination file exists (used for skip cases
// where absence is the expected state).
func destExists(t *testing.T, dest, rel string) bool {
	t.Helper()
	_, err := os.Stat(filepath.Join(dest, rel))
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	t.Fatalf("stat %q: %v", rel, err)
	return false
}

func TestApply(t *testing.T) {
	vars := map[string]string{
		"AI_CABIN_HOME": "/home/user",
		"AI_CABIN_DESK": "/home/user/desk",
	}

	t.Run("CopiesPlainAndRendersTmpl", func(t *testing.T) {
		// A first copy with SkipCreator: every file is absent so all land.
		// Plain files are copied as-is; .tmpl files are rendered with vars
		// (namespaced .Vars.X) and the .tmpl suffix is stripped from the dst.
		srcFS := deskSkeleton()
		dest := t.TempDir()

		written, err := skeletons.Apply(srcFS, "minimal", dest, vars, writestrategy.SkipCreator{})
		require.NoError(t, err)

		// Plain file copied as-is.
		assert.Equal(t, "# Agent rules\n", readDest(t, dest, "AGENTS.md"))
		// .tmpl rendered, suffix stripped, var substituted.
		assert.Equal(t, "desk=/home/user/desk\n", readDest(t, dest, "config"))
		// Nested dir created, plain skill copied.
		assert.Equal(t, "# Retro\n", readDest(t, dest, "skills/retro/SKILL.md"))
		// Nested .tmpl rendered, suffix stripped.
		assert.Equal(t, "home=/home/user\n", readDest(t, dest, "skills/retro/rendered.md"))

		// The .tmpl suffix is stripped from the written list too (the
		// destination name is the non-.tmpl form).
		sort.Strings(written)
		assert.Equal(t, []string{
			"AGENTS.md",
			"TODO.md",
			"config",
			"skills/retro/SKILL.md",
			"skills/retro/rendered.md",
		}, written)
	})

	t.Run("SkipCreatorNoOverwrite", func(t *testing.T) {
		// A second Apply with SkipCreator on an existing dest: existing files
		// are skipped (ErrSkip, non-fatal), their content unchanged. This is
		// the no-overwrite default of `cabin profile init` (without --force):
		// re-running does not clobber a desk the user edited.
		srcFS := deskSkeleton()
		dest := t.TempDir()

		// First copy lands everything.
		_, err := skeletons.Apply(srcFS, "minimal", dest, vars, writestrategy.SkipCreator{})
		require.NoError(t, err)

		// Edit a destination file (mimics a user edit) and change the source
		// so a diff would be visible if it were overwritten.
		require.NoError(t, os.WriteFile(filepath.Join(dest, "AGENTS.md"), []byte("# My edits\n"), 0o644))
		srcFS["minimal/AGENTS.md"].Data = []byte("# CHANGED\n")

		written, err := skeletons.Apply(srcFS, "minimal", dest, vars, writestrategy.SkipCreator{})
		require.NoError(t, err)

		// Every file already exists -> all skipped, written is empty.
		assert.Empty(t, written, "existing files are skipped")
		// The user edit survives.
		assert.Equal(t, "# My edits\n", readDest(t, dest, "AGENTS.md"))
	})

	t.Run("TruncateCreatorOverwrites", func(t *testing.T) {
		// TruncateCreator (--force) overwrites existing files: a re-run with
		// changed source updates every destination. The .cabin-bak backup is a
		// BackupCreator concern, not TruncateCreator's (skeletons use truncate
		// for --force, not backup).
		srcFS := deskSkeleton()
		dest := t.TempDir()

		_, err := skeletons.Apply(srcFS, "minimal", dest, vars, writestrategy.SkipCreator{})
		require.NoError(t, err)

		// Change source content and re-apply with TruncateCreator.
		srcFS["minimal/AGENTS.md"].Data = []byte("# CHANGED\n")
		srcFS["minimal/config.tmpl"].Data = []byte("desk={{.Vars.AI_CABIN_DESK}} v2\n")

		written, err := skeletons.Apply(srcFS, "minimal", dest, vars, writestrategy.TruncateCreator{})
		require.NoError(t, err)

		assert.Equal(t, "# CHANGED\n", readDest(t, dest, "AGENTS.md"))
		assert.Equal(t, "desk=/home/user/desk v2\n", readDest(t, dest, "config"))
		assert.Len(t, written, 5)
	})

	t.Run("CollectsRenderErrorsNoFailFast", func(t *testing.T) {
		// A .tmpl referencing an undefined var yields render.ErrUndefinedVar
		// (the output contains "<no value>"). The error is collected (not
		// fatal), the other files still copy. Mirrors internal/fragments: the
		// user sees every issue in one run.
		srcFS := fstest.MapFS{
			"sk/ok.txt":         {Data: []byte("plain\n")},
			"sk/broken.tmpl":    {Data: []byte("val={{.Vars.UNDEFINED_BUT_OK}}\n")},
			"sk/undefined.tmpl": {Data: []byte("{{.NOPE}}\n")}, // top-level .NOPE -> undefined
		}
		dest := t.TempDir()

		written, err := skeletons.Apply(srcFS, "sk", dest, vars, writestrategy.TruncateCreator{})

		// The plain file and the guarded .tmpl land; the undefined-var .tmpl
		// also writes (écriture-malgré-erreur) but is reported as an error.
		assert.Contains(t, written, "ok.txt")
		// broken.tmpl -> "broken" rendered (guarded {{.Vars.X}} on a missing
		// key renders "<no value>", which is an error sentinel, but the value
		// IS present in vars-style namespace so it renders the marker).
		require.Error(t, err)
		assert.ErrorIs(t, err, render.ErrUndefinedVar)
	})

	t.Run("EmptyTree", func(t *testing.T) {
		// An empty skeleton dir is a no-op: no error, empty written. WalkDir
		// on an empty dir yields only the root (a dir, skipped).
		srcFS := fstest.MapFS{"empty/.keep": {Data: nil}} // removed below
		delete(srcFS, "empty/.keep")
		// fstest.MapFS with just a dir entry: emulate by not adding any file.
		srcFS = fstest.MapFS{}
		dest := t.TempDir()

		written, err := skeletons.Apply(srcFS, ".", dest, vars, writestrategy.SkipCreator{})
		require.NoError(t, err)
		assert.Empty(t, written)
	})

	t.Run("RejectsNonWalkFS", func(t *testing.T) {
		// A plain fs.FS (no ReadDirFS+StatFS) is rejected up front rather than
		// failing deep in the walk. Mirrors internal/fragments.NewMaterializer.
		// bareFS implements only fs.FS (Open), so WalkDir cannot list dirs.
		srcFS := bareFS{}
		dest := t.TempDir()

		_, err := skeletons.Apply(srcFS, ".", dest, vars, writestrategy.SkipCreator{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "ReadDirFS")
	})
}

// bareFS implements only fs.FS.Open (no ReadDirFS, no StatFS), used to assert
// Apply rejects an FS that cannot walk.
type bareFS struct{}

func (bareFS) Open(string) (fs.File, error) { return nil, fs.ErrNotExist }
