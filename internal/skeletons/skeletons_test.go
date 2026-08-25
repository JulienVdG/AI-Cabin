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

// deskSkeleton is a representative Class 1 desk skeleton using mirror mode:
// a mandatory skeleton.yaml manifest declares mirror: content, and the content/
// subtree holds a plain AGENTS.md, a rendered config.tmpl, and a nested skill
// file. Used across desk sub-cases so the tree shape (manifest + mirror +
// nested + .tmpl) is asserted once and reused.
func deskSkeleton() fstest.MapFS {
	return fstest.MapFS{
		"minimal/skeleton.yaml":                         {Data: []byte("mirror: content\n")},
		"minimal/content/AGENTS.md":                     {Data: []byte("# Agent rules\n")},
		"minimal/content/TODO.md":                       {Data: []byte("# Tasks\n")},
		"minimal/content/config.tmpl":                   {Data: []byte("desk={{.Vars.AI_CABIN_DESK}}\n")},
		"minimal/content/skills/retro/SKILL.md":         {Data: []byte("# Retro\n")},
		"minimal/content/skills/retro/rendered.md.tmpl": {Data: []byte("home={{.Vars.AI_CABIN_HOME}}\n")},
	}
}

// projectSkeleton is a Class 1 project skeleton using entries mode: the
// manifest maps src (under content/) to dst, with a templated destination
// name (cmd/{{.project}}/main.go) and per-instance attrs ({{.module}}). This
// exercises the fragments Materializer entries path via the skeletons facade.
func projectSkeleton() fstest.MapFS {
	return fstest.MapFS{
		"go_makefile/skeleton.yaml":               {Data: []byte("entries:\n  - src: content/go.mod.tmpl\n    dst: go.mod\n  - src: content/cmd/project/main.go\n    dst: cmd/{{.project}}/main.go\n")},
		"go_makefile/content/go.mod.tmpl":         {Data: []byte("module {{.module}}\n\ngo 1.26\n")},
		"go_makefile/content/cmd/project/main.go": {Data: []byte("package main\n\nfunc main() {}\n")},
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

	t.Run("MirrorCopiesPlainAndRendersTmpl", func(t *testing.T) {
		// A desk skeleton with mirror: content. Plain files are copied as-is;
		// .tmpl files are rendered with vars (namespaced .Vars.X) and the
		// .tmpl suffix is stripped from the dst. The manifest itself is not
		// copied (it lives outside content/).
		srcFS := deskSkeleton()
		dest := t.TempDir()

		written, err := skeletons.Apply(srcFS, "minimal", dest, vars, nil, writestrategy.SkipCreator{})
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
		// destination name is the non-.tmpl form). The manifest is not copied.
		sort.Strings(written)
		assert.Equal(t, []string{
			"AGENTS.md",
			"TODO.md",
			"config",
			"skills/retro/SKILL.md",
			"skills/retro/rendered.md",
		}, written)
		// The manifest itself is not in the destination.
		assert.False(t, destExists(t, dest, "skeleton.yaml"))
	})

	t.Run("SkipCreatorNoOverwrite", func(t *testing.T) {
		// A second Apply with SkipCreator on an existing dest: existing files
		// are skipped (ErrSkip, non-fatal), their content unchanged. This is
		// the no-overwrite default of `cabin profile init` (without --force):
		// re-running does not clobber a desk the user edited.
		srcFS := deskSkeleton()
		dest := t.TempDir()

		// First copy lands everything.
		_, err := skeletons.Apply(srcFS, "minimal", dest, vars, nil, writestrategy.SkipCreator{})
		require.NoError(t, err)

		// Edit a destination file (mimics a user edit) and change the source
		// so a diff would be visible if it were overwritten.
		require.NoError(t, os.WriteFile(filepath.Join(dest, "AGENTS.md"), []byte("# My edits\n"), 0o644))
		srcFS["minimal/content/AGENTS.md"].Data = []byte("# CHANGED\n")

		written, err := skeletons.Apply(srcFS, "minimal", dest, vars, nil, writestrategy.SkipCreator{})
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

		_, err := skeletons.Apply(srcFS, "minimal", dest, vars, nil, writestrategy.SkipCreator{})
		require.NoError(t, err)

		// Change source content and re-apply with TruncateCreator.
		srcFS["minimal/content/AGENTS.md"].Data = []byte("# CHANGED\n")
		srcFS["minimal/content/config.tmpl"].Data = []byte("desk={{.Vars.AI_CABIN_DESK}} v2\n")

		written, err := skeletons.Apply(srcFS, "minimal", dest, vars, nil, writestrategy.TruncateCreator{})
		require.NoError(t, err)

		assert.Equal(t, "# CHANGED\n", readDest(t, dest, "AGENTS.md"))
		assert.Equal(t, "desk=/home/user/desk v2\n", readDest(t, dest, "config"))
		assert.Len(t, written, 5)
	})

	t.Run("EntriesTemplatedDstAndAttrs", func(t *testing.T) {
		// A project skeleton with entries: exercises templated destination
		// names (cmd/{{.project}}/main.go) and per-instance attrs
		// ({{.module}}), distinct from profile vars ({{.Vars.X}}). The
		// positional <name> populates the "project" attr; "module" comes from
		// the caller. The manifest itself is not copied (it is not an entry).
		srcFS := projectSkeleton()
		dest := t.TempDir()
		attrs := map[string]any{
			"project": "mysvc",
			"module":  "github.com/me/mysvc",
		}

		written, err := skeletons.Apply(srcFS, "go_makefile", dest, vars, attrs, writestrategy.SkipCreator{})
		require.NoError(t, err)

		// go.mod.tmpl rendered with the module attr.
		assert.Equal(t, "module github.com/me/mysvc\n\ngo 1.26\n", readDest(t, dest, "go.mod"))
		// cmd/{{.project}}/main.go: the destination name was templated.
		assert.Equal(t, "package main\n\nfunc main() {}\n", readDest(t, dest, "cmd/mysvc/main.go"))
		// The literal source dir cmd/project/ was NOT created (the entry's dst
		// is cmd/mysvc/, so project/ never lands).
		assert.False(t, destExists(t, dest, "cmd/project/main.go"))
		// The manifest is not copied (not an entry).
		assert.False(t, destExists(t, dest, "skeleton.yaml"))

		sort.Strings(written)
		assert.Equal(t, []string{"cmd/mysvc/main.go", "go.mod"}, written)
	})

	t.Run("EntriesUndefinedAttrErrors", func(t *testing.T) {
		// A referenced-but-unset attr renders <no value> (ErrUndefinedVar),
		// surfaced so the user learns which --attr to pass. The output is
		// still written (écriture-malgré-erreur) for inspection.
		srcFS := projectSkeleton()
		dest := t.TempDir()
		// "module" omitted -> {{.module}} renders <no value>.
		attrs := map[string]any{"project": "mysvc"}

		written, err := skeletons.Apply(srcFS, "go_makefile", dest, vars, attrs, writestrategy.TruncateCreator{})

		// The main.go (no undefined attr in its name/content) lands; go.mod
		// writes with <no value> and is reported as an error.
		assert.Contains(t, written, "cmd/mysvc/main.go")
		require.Error(t, err)
		assert.ErrorIs(t, err, render.ErrUndefinedVar)
		assert.Contains(t, readDest(t, dest, "go.mod"), "<no value>")
	})

	t.Run("MirrorCollectsRenderErrorsNoFailFast", func(t *testing.T) {
		// A .tmpl referencing an undefined var yields render.ErrUndefinedVar
		// (the output contains "<no value>"). The error is collected (not
		// fatal), the other files still copy. Mirrors internal/fragments: the
		// user sees every issue in one run.
		srcFS := fstest.MapFS{
			"sk/skeleton.yaml":          {Data: []byte("mirror: content\n")},
			"sk/content/ok.txt":         {Data: []byte("plain\n")},
			"sk/content/broken.tmpl":    {Data: []byte("val={{.Vars.UNDEFINED_BUT_OK}}\n")},
			"sk/content/undefined.tmpl": {Data: []byte("{{.NOPE}}\n")}, // top-level .NOPE -> undefined
		}
		dest := t.TempDir()

		written, err := skeletons.Apply(srcFS, "sk", dest, vars, nil, writestrategy.TruncateCreator{})

		// The plain file and the guarded .tmpl land; the undefined-var .tmpl
		// also writes (écriture-malgré-erreur) but is reported as an error.
		assert.Contains(t, written, "ok.txt")
		require.Error(t, err)
		assert.ErrorIs(t, err, render.ErrUndefinedVar)
	})

	t.Run("RejectsMissingManifest", func(t *testing.T) {
		// A directory without skeleton.yaml is not a skeleton: Apply rejects
		// it up front (before delegating to the Materializer) so the user gets
		// a clear "no manifest" error rather than a cryptic fragments error.
		srcFS := fstest.MapFS{
			"sk/AGENTS.md": {Data: []byte("# No manifest\n")},
		}
		dest := t.TempDir()

		_, err := skeletons.Apply(srcFS, "sk", dest, vars, nil, writestrategy.SkipCreator{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "skeleton.yaml")
	})

	t.Run("RejectsNonWalkFS", func(t *testing.T) {
		// A plain fs.FS (no ReadDirFS+StatFS) is rejected by NewMaterializer
		// rather than failing deep in the walk. The manifest check uses Open
		// (works on any fs.FS), so the rejection happens at NewMaterializer.
		// bareFS implements only fs.FS (Open) and returns ErrNotExist, so the
		// manifest check fails first with the "no manifest" error.
		srcFS := bareFS{}
		dest := t.TempDir()

		_, err := skeletons.Apply(srcFS, ".", dest, vars, nil, writestrategy.SkipCreator{})
		require.Error(t, err)
	})
}

// bareFS implements only fs.FS.Open (no ReadDirFS, no StatFS), used to assert
// Apply rejects an FS that cannot walk.
type bareFS struct{}

func (bareFS) Open(string) (fs.File, error) { return nil, fs.ErrNotExist }

// TestBuildLayersCatalogue covers the layer contribution to the skeleton
// catalogue: a <root>/skeletons dir is resolved by name over the embedded
// catalogue, and a layer root without a skeletons/ subdir is tolerated (a
// layer may be fragments-only).
func TestBuildLayersCatalogue(t *testing.T) {
	// On-disk layer with a skeletons/ subdir shipping a custom project skeleton.
	layerRoot := t.TempDir()
	skelDir := filepath.Join(layerRoot, "skeletons")
	require.NoError(t, os.MkdirAll(filepath.Join(skelDir, "corp_go"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(skelDir, "corp_go", "skeleton.yaml"),
		[]byte("entries:\n  - src: main.go\n    dst: main.go\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(skelDir, "corp_go", "main.go"),
		[]byte("package main\n"), 0o644))
	emb := fstest.MapFS{
		"desks/minimal/skeleton.yaml":     {Data: []byte("mirror: content\n")},
		"desks/minimal/content/AGENTS.md": {Data: []byte("# Agent rules\n")},
		"corp_go/skeleton.yaml":           {Data: []byte("mirror: content\n")},
		"corp_go/content/README.md":       {Data: []byte("embedded\n")},
	}

	t.Run("LayerSkeletonsResolvedOverEmbed", func(t *testing.T) {
		merged, err := skeletons.BuildLayers(nil, []string{filepath.Join(layerRoot, "skeletons")}, emb)
		require.NoError(t, err)

		dest := t.TempDir()
		written, err := skeletons.Apply(merged, "corp_go", dest, nil, nil, writestrategy.SkipCreator{})
		require.NoError(t, err)
		assert.Equal(t, []string{"main.go"}, written)
		// The layer version shadows the embedded corps_go of the same name.
		assert.Equal(t, "package main\n", readDest(t, dest, "main.go"))
		// Embedded minimal desk still reachable.
		_, err = merged.Open("desks/minimal/skeleton.yaml")
		require.NoError(t, err)
	})

	t.Run("LayerWithoutSkeletonsTolerated", func(t *testing.T) {
		// A layer root with no skeletons/ subdir must not break resolution: the
		// catalogue still resolves the embedded minimal desk.
		merged, err := skeletons.BuildLayers(nil, []string{layerRoot}, emb)
		require.NoError(t, err)

		_, err = merged.Open("desks/minimal/skeleton.yaml")
		require.NoError(t, err)
	})
}
