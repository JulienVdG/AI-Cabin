package fragments_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"testing/fstest"

	"github.com/JulienVdG/AI-Cabin/internal/cabin"
	"github.com/JulienVdG/AI-Cabin/internal/embedded"
	"github.com/JulienVdG/AI-Cabin/internal/fragments"
	"github.com/JulienVdG/AI-Cabin/internal/render"
	"github.com/JulienVdG/AI-Cabin/internal/writestrategy"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeLayer writes a map of relpath->content into dir (creating parents),
// to back an os.DirFS layer for BuildLayers tests.
func writeLayer(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for rel, content := range files {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		require.NoError(t, os.WriteFile(p, []byte(content), 0o644))
	}
}

// readDest reads a relpath under destBase, failing the test if missing.
func readDest(t *testing.T, destBase, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(destBase, filepath.FromSlash(rel)))
	require.NoError(t, err, "expected written file %s", rel)
	return string(b)
}

// truncateMat builds a Materializer with TruncateCreator (the deps-facet copy
// strategy: overwrite on every run). A thin helper so each sub-test stays
// focused on the behaviour under test, not on struct construction.
func truncateMat(t *testing.T, merged fs.FS, manifest, dest string, vars map[string]string) *fragments.Materializer {
	t.Helper()
	mat, err := fragments.NewMaterializer(merged, manifest, dest, vars, writestrategy.TruncateCreator{})
	require.NoError(t, err)
	return mat
}

func TestBuildLayers(t *testing.T) {
	t.Run("LayersOrderedFirstWins", func(t *testing.T) {
		// conf layer > cabin-local > embed; a file in the conf layer shadows
		// the others.
		confDir := t.TempDir()
		writeLayer(t, confDir, map[string]string{"base/shared.txt": "conf"})
		cabinDir := t.TempDir()
		writeLayer(t, cabinDir, map[string]string{"base/shared.txt": "cabin", "base/only-cabin.txt": "cabin"})
		embedFS := fstest.MapFS{"base/shared.txt": {Data: []byte("embed")}, "base/only-embed.txt": {Data: []byte("embed")}}

		merged, err := fragments.BuildLayers([]string{confDir}, cabinDir, embedFS)
		require.NoError(t, err)

		assert.Equal(t, "conf", readFS(t, merged, "base/shared.txt"))
		assert.Equal(t, "cabin", readFS(t, merged, "base/only-cabin.txt"))
		assert.Equal(t, "embed", readFS(t, merged, "base/only-embed.txt"))
	})

	t.Run("MultipleConfDirsFirstWins", func(t *testing.T) {
		// Two conf dirs in the list: the first wins for a shared file, the
		// second contributes its own files. (Comma-split and ~ expansion are
		// covered by config.SplitPathList tests; here only the union order.)
		dir1 := t.TempDir()
		dir2 := t.TempDir()
		writeLayer(t, dir1, map[string]string{"base/x.txt": "1"})
		writeLayer(t, dir2, map[string]string{"base/x.txt": "2", "base/y.txt": "2"})

		merged, err := fragments.BuildLayers([]string{dir1, dir2}, "", nil)
		require.NoError(t, err)

		assert.Equal(t, "1", readFS(t, merged, "base/x.txt")) // dir1 wins over dir2
		assert.Equal(t, "2", readFS(t, merged, "base/y.txt")) // dir2 only
	})

	t.Run("MissingConfDirIsStrictError", func(t *testing.T) {
		_, err := fragments.BuildLayers([]string{"/does/not/exist"}, "", nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "fragment dir")
	})

	t.Run("MissingCabinDirIsStrictError", func(t *testing.T) {
		_, err := fragments.BuildLayers(nil, "/does/not/exist", nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cabin-local")
	})

	t.Run("NoLayersConfiguredErrors", func(t *testing.T) {
		_, err := fragments.BuildLayers(nil, "", nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no fragment layers")
	})
}

// readFS is the fs.FS counterpart of readDest, for BuildLayers union tests.
func readFS(t *testing.T, fsys fs.FS, name string) string {
	t.Helper()
	b, err := fs.ReadFile(fsys, name)
	require.NoError(t, err)
	return string(b)
}

func TestMaterialize(t *testing.T) {
	t.Run("DepsMirror", func(t *testing.T) {
		// base/deps.yaml mirrors deps/; a .tmpl is rendered (suffix stripped from
		// the dst name), a plain file is copied as-is, subdirs are preserved.
		merged := fstest.MapFS{
			"base/deps.yaml":              {Data: []byte("mirror: deps/\n")},
			"base/deps/entrypoint.sh":     {Data: []byte("#!/bin/bash\nexec $@\n")},
			"base/deps/models.json.tmpl":  {Data: []byte(`{"id":"{{.Vars.SCW_PROJECT_ID}}"}`)},
			"base/deps/hooks/10-socat.sh": {Data: []byte("socat\n")},
		}
		vars := map[string]string{"SCW_PROJECT_ID": "proj-1"}
		dest := t.TempDir()

		written, err := truncateMat(t, merged, "deps.yaml", dest, vars).Materialize("base", nil)
		require.NoError(t, err)
		sort.Strings(written)
		assert.Equal(t, []string{"entrypoint.sh", "hooks/10-socat.sh", "models.json"}, written)

		assert.Equal(t, "#!/bin/bash\nexec $@\n", readDest(t, dest, "entrypoint.sh"))
		assert.Equal(t, `{"id":"proj-1"}`, readDest(t, dest, "models.json"))
		assert.Equal(t, "socat\n", readDest(t, dest, "hooks/10-socat.sh"))
	})

	t.Run("DepsEntriesTemplatedDst", func(t *testing.T) {
		// port-forward: explicit entries with {{.host}}-{{.port}} in dst. Two
		// instances (mariadb:3306, apache:8080) do not collide because the dst
		// encodes host-port.
		merged := fstest.MapFS{
			"port-forward/deps.yaml":              {Data: []byte("entries:\n  - src: deps/socat.sh.tmpl\n    dst: hooks/50-socat-{{.host}}-{{.port}}.sh\n  - src: deps/profile.json.tmpl\n    dst: profile/{{.host}}-{{.port}}.json\n")},
			"port-forward/deps/socat.sh.tmpl":     {Data: []byte("socat TCP-LISTEN:{{.port}},fork TCP:{{.host}}:{{.port}}\n")},
			"port-forward/deps/profile.json.tmpl": {Data: []byte(`{"forwardPorts":[{{.port}}]}`)},
		}
		dest := t.TempDir()

		// First instance: mariadb:3306.
		w1, err := truncateMat(t, merged, "deps.yaml", dest, nil).Materialize("port-forward",
			map[string]any{"host": "mariadb", "port": "3306"})
		require.NoError(t, err)
		sort.Strings(w1)
		assert.Equal(t, []string{"hooks/50-socat-mariadb-3306.sh", "profile/mariadb-3306.json"}, w1)

		// Second instance: apache:8080 — same bundle, different attrs, no collision.
		w2, err := truncateMat(t, merged, "deps.yaml", dest, nil).Materialize("port-forward",
			map[string]any{"host": "apache", "port": "8080"})
		require.NoError(t, err)
		sort.Strings(w2)
		assert.Equal(t, []string{"hooks/50-socat-apache-8080.sh", "profile/apache-8080.json"}, w2)

		assert.Equal(t, "socat TCP-LISTEN:3306,fork TCP:mariadb:3306\n", readDest(t, dest, "hooks/50-socat-mariadb-3306.sh"))
		assert.Equal(t, `{"forwardPorts":[3306]}`, readDest(t, dest, "profile/mariadb-3306.json"))
		assert.Equal(t, "socat TCP-LISTEN:8080,fork TCP:apache:8080\n", readDest(t, dest, "hooks/50-socat-apache-8080.sh"))
	})

	t.Run("PortForwardBundleFromEmbedded", func(t *testing.T) {
		// Materialize the real embedded port-forward bundle (not an inline
		// fixture): this validates the shipped fragments end-to-end through
		// BuildLayers + Materialize. The bundle has two facets: deps (the
		// 50-socat entrypoint script, build facet) and setup (the greywall
		// forward profile, seeded to .config/greywall/learned/ —
		// greywall profiles are a setup facet, not deps. Two instances
		// (mariadb:3306, apache:8080) do not collide: the dst encodes
		// host-port, and the profile carries a forward- prefix to avoid
		// collisions with agent profiles (pi.json, opencode.json).
		embedFS, err := embedded.Fragments()
		require.NoError(t, err)
		merged, err := fragments.BuildLayers(nil, "", embedFS)
		require.NoError(t, err)

		// --- deps facet (TruncateCreator): socat entrypoint script into .deps/.
		depsDest := t.TempDir()
		depsMat := truncateMat(t, merged, "deps.yaml", depsDest, nil)

		attrs := map[string]any{"host": "mariadb", "port": "3306"}
		w, err := depsMat.Materialize("port-forward", attrs)
		require.NoError(t, err)
		sort.Strings(w)
		assert.Equal(t, []string{"docker-entrypoint.d/50-socat-mariadb-3306.sh"}, w)
		assert.Contains(t, readDest(t, depsDest, "docker-entrypoint.d/50-socat-mariadb-3306.sh"),
			"socat TCP-LISTEN:3306,fork,reuseaddr TCP:mariadb:3306 &")

		// Second instance: same bundle, different attrs, no collision.
		attrs2 := map[string]any{"host": "apache", "port": "8080"}
		w2, err := depsMat.Materialize("port-forward", attrs2)
		require.NoError(t, err)
		sort.Strings(w2)
		assert.Equal(t, []string{"docker-entrypoint.d/50-socat-apache-8080.sh"}, w2)

		// --- setup facet (BackupCreator): greywall forward profile into
		// $AI_CABIN_HOME/.config/greywall/learned/.
		setupDest := t.TempDir()
		setupMat := backupMat(t, merged, "setup.yaml", setupDest, nil)

		ws, err := setupMat.Materialize("port-forward", attrs)
		require.NoError(t, err)
		sort.Strings(ws)
		assert.Equal(t, []string{".config/greywall/learned/forward-mariadb-3306.json"}, ws)
		assert.JSONEq(t, `{"network":{"forwardPorts":[3306]}}`,
			readDest(t, setupDest, ".config/greywall/learned/forward-mariadb-3306.json"))

		ws2, err := setupMat.Materialize("port-forward", attrs2)
		require.NoError(t, err)
		sort.Strings(ws2)
		assert.Equal(t, []string{".config/greywall/learned/forward-apache-8080.json"}, ws2)
	})

	t.Run("SetupEntries", func(t *testing.T) {
		// setup facet: entries map src -> dst under $AI_CABIN_HOME (destBase). Same
		// code path as deps; the facet is carried only by manifestName.
		merged := fstest.MapFS{
			"agent-pi/setup.yaml":             {Data: []byte("entries:\n  - src: setup/models.json.tmpl\n    dst: .pi/agent/models.json\n  - src: setup/settings.json\n    dst: .pi/agent/settings.json\n")},
			"agent-pi/setup/models.json.tmpl": {Data: []byte(`{"project":"{{.Vars.SCW_PROJECT_ID}}"}`)},
			"agent-pi/setup/settings.json":    {Data: []byte(`{"quiet":true}`)},
		}
		vars := map[string]string{"SCW_PROJECT_ID": "proj-9"}
		dest := t.TempDir()

		written, err := truncateMat(t, merged, "setup.yaml", dest, vars).Materialize("agent-pi", nil)
		require.NoError(t, err)
		sort.Strings(written)
		assert.Equal(t, []string{".pi/agent/models.json", ".pi/agent/settings.json"}, written)

		assert.Equal(t, `{"project":"proj-9"}`, readDest(t, dest, ".pi/agent/models.json"))
		assert.Equal(t, `{"quiet":true}`, readDest(t, dest, ".pi/agent/settings.json"))
	})

	t.Run("ManifestAbsentIsNoOp", func(t *testing.T) {
		// A bundle without a setup facet: setup.yaml absent = no-op, no error.
		merged := fstest.MapFS{
			"port-forward/deps.yaml":     {Data: []byte("mirror: deps/\n")},
			"port-forward/deps/socat.sh": {Data: []byte("socat\n")},
		}
		dest := t.TempDir()

		written, err := truncateMat(t, merged, "setup.yaml", dest, nil).Materialize("port-forward", nil)
		require.NoError(t, err)
		assert.Nil(t, written)
	})

	t.Run("BundleNotFound", func(t *testing.T) {
		// Bundle absent from all layers = strict error wrapping fs.ErrNotExist.
		// No sentinel: nothing branches on it; the message carries the bundle name.
		merged := fstest.MapFS{"base/deps.yaml": {Data: []byte("mirror: deps/\n")}}
		dest := t.TempDir()

		_, err := truncateMat(t, merged, "deps.yaml", dest, nil).Materialize("agent-opencode", nil)
		require.Error(t, err)
		assert.ErrorIs(t, err, fs.ErrNotExist)
		assert.Contains(t, err.Error(), "agent-opencode")
	})

	t.Run("FragmentDrift", func(t *testing.T) {
		// Manifest references a src that does not exist in merged: strict error
		// wrapping fs.ErrNotExist, message carries the bundle/manifest/src context.
		merged := fstest.MapFS{
			"base/deps.yaml": {Data: []byte("entries:\n  - src: deps/missing.sh\n    dst: missing.sh\n")},
		}
		dest := t.TempDir()

		_, err := truncateMat(t, merged, "deps.yaml", dest, nil).Materialize("base", nil)
		require.Error(t, err)
		assert.ErrorIs(t, err, fs.ErrNotExist)
		assert.Contains(t, err.Error(), "deps/missing.sh")
	})

	t.Run("UndefinedVarCollectedAndContinues", func(t *testing.T) {
		// A .tmpl references an undefined var: the broken content (with <no
		// value>) is written (écriture-malgré-erreur) and left on disk, the error is
		// collected, and materialization continues for the other fragments. The
		// broken file is NOT in the success list (written = ok.txt only) but is
		// readable on disk for the user to locate the missing var.
		merged := fstest.MapFS{
			"base/deps.yaml":        {Data: []byte("entries:\n  - src: deps/broken.tmpl\n    dst: broken.txt\n  - src: deps/ok.txt\n    dst: ok.txt\n")},
			"base/deps/broken.tmpl": {Data: []byte("v={{.Vars.MISSING}}")},
			"base/deps/ok.txt":      {Data: []byte("ok")},
		}
		dest := t.TempDir()

		written, err := truncateMat(t, merged, "deps.yaml", dest, nil).Materialize("base", nil)
		require.Error(t, err)
		assert.ErrorIs(t, err, render.ErrUndefinedVar)
		// Only the successful file is listed; the broken one is on disk (partial).
		sort.Strings(written)
		assert.Equal(t, []string{"ok.txt"}, written)
		assert.Equal(t, "v=<no value>", readDest(t, dest, "broken.txt"))
		assert.Equal(t, "ok", readDest(t, dest, "ok.txt"))
	})

	t.Run("UndefinedVarInDstSkipsWrite", func(t *testing.T) {
		// A dst with {{ referencing an undefined var: the file is not written (its
		// name cannot be resolved), the error is collected, materialization
		// continues.
		merged := fstest.MapFS{
			"base/deps.yaml":   {Data: []byte("entries:\n  - src: deps/a.tmpl\n    dst: p-{{.host}}.txt\n  - src: deps/b.txt\n    dst: b.txt\n")},
			"base/deps/a.tmpl": {Data: []byte("a")},
			"base/deps/b.txt":  {Data: []byte("b")},
		}
		dest := t.TempDir()

		written, err := truncateMat(t, merged, "deps.yaml", dest, nil).Materialize("base", map[string]any{})
		require.Error(t, err)
		assert.ErrorIs(t, err, render.ErrUndefinedVar)
		// Only the non-templated-dst file was written.
		assert.Equal(t, []string{"b.txt"}, written)
		_, statErr := os.Stat(filepath.Join(dest, "b.txt"))
		require.NoError(t, statErr)
	})

	t.Run("InvalidManifest", func(t *testing.T) {
		cases := []struct {
			name    string
			yaml    string
			wantErr string
		}{
			{"BothMirrorAndEntries", "mirror: deps/\nentries:\n  - src: a\n    dst: a\n", "both"},
			{"NeitherMode", "--- {}\n", "neither"},
			{"EmptySrc", "entries:\n  - src: \"\"\n    dst: a\n", "empty src"},
			{"EmptyDst", "entries:\n  - src: a\n    dst: \"\"\n", "empty dst"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				merged := fstest.MapFS{"base/deps.yaml": {Data: []byte(tc.yaml)}}
				_, err := truncateMat(t, merged, "deps.yaml", t.TempDir(), nil).Materialize("base", nil)
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
			})
		}
	})

	t.Run("UsesUmaskMode", func(t *testing.T) {
		// The destination mode is always writestrategy.FilePerm (0666 & ^umask): the source
		// mode is not preserved (embed.FS exposes every file as 0444, an
		// artifact; the executable bit is the Dockerfile's authority via
		// RUN chmod +x). So a 0755 wrapper and a 0644 plain file both land at
		// the same mode as os.Create (0666 & ^umask), whatever the runtime
		// umask is. The expected mode is derived from a sentinel file created
		// with os.Create in the same dest dir (same umask), instead of being
		// hardcoded to 0644 — a umask of 0002 (group-writable) would otherwise
		// flake the test.
		src := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(src, "base", "deps"), 0o755))
		wrapper := filepath.Join(src, "base", "deps", "greybash")
		require.NoError(t, os.WriteFile(wrapper, []byte("#!/bin/bash\n"), 0o755))
		plain := filepath.Join(src, "base", "deps", "entrypoint.sh")
		require.NoError(t, os.WriteFile(plain, []byte("#!/bin/bash\n"), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(src, "base", "deps.yaml"), []byte("mirror: deps/\n"), 0o644))

		merged, err := fragments.BuildLayers([]string{src}, "", nil)
		require.NoError(t, err)

		dest := t.TempDir()
		_, err = truncateMat(t, merged, "deps.yaml", dest, nil).Materialize("base", nil)
		require.NoError(t, err)

		// Sentinel: os.Create applies 0666 & ^umask, the same contract as
		// Materialize. Comparing to it makes the test umask-agnostic.
		sentinel, err := os.Create(filepath.Join(dest, ".sentinel"))
		require.NoError(t, err)
		require.NoError(t, sentinel.Close())
		sentinelInfo, err := os.Stat(sentinel.Name())
		require.NoError(t, err)
		want := sentinelInfo.Mode().Perm()

		info, err := os.Stat(filepath.Join(dest, "greybash"))
		require.NoError(t, err)
		assert.Equal(t, want, info.Mode().Perm(), "wrapper destination mode")
		info, err = os.Stat(filepath.Join(dest, "entrypoint.sh"))
		require.NoError(t, err)
		assert.Equal(t, want, info.Mode().Perm(), "plain file destination mode")
	})

	t.Run("DriftCollectedNotAborted", func(t *testing.T) {
		// Drift is collected (no fail-fast): both missing srcs are reported in
		// one run so the user fixes all manifest issues at once.
		merged := fstest.MapFS{
			"base/deps.yaml": {Data: []byte("entries:\n  - src: deps/missing.sh\n    dst: a.sh\n  - src: deps/also-missing.sh\n    dst: b.sh\n")},
		}
		_, err := truncateMat(t, merged, "deps.yaml", t.TempDir(), nil).Materialize("base", nil)
		require.Error(t, err)
		assert.ErrorIs(t, err, fs.ErrNotExist)
		// Drift is collected, so both missing srcs are reported.
		assert.Contains(t, err.Error(), "missing.sh")
		assert.Contains(t, err.Error(), "also-missing")
	})

	t.Run("SkipCreatorNoOverwrite", func(t *testing.T) {
		// The FileCreator.ErrSkip contract: a creator that declines to write
		// (SkipCreator skips existing files; a future InteractiveCreator may
		// decline per user choice) is non-fatal. The skipped file is neither in
		// the written list nor in the aggregated error, and materialization
		// continues for the rest. This is the no-overwrite default of the
		// skeletons facade (re-running `cabin profile init` without --force is a
		// silent no-op, not an error). deps/setup use
		// TruncateCreator/BackupCreator and never trip ErrSkip, so the contract
		// is exercised here via SkipCreator.
		merged := fstest.MapFS{
			"base/deps.yaml":  {Data: []byte("entries:\n  - src: deps/a.txt\n    dst: a.txt\n  - src: deps/b.txt\n    dst: b.txt\n")},
			"base/deps/a.txt": {Data: []byte("a")},
			"base/deps/b.txt": {Data: []byte("b")},
		}
		dest := t.TempDir()

		// First pass with TruncateCreator lands both files.
		mat := truncateMat(t, merged, "deps.yaml", dest, nil)
		written, err := mat.Materialize("base", nil)
		require.NoError(t, err)
		sort.Strings(written)
		assert.Equal(t, []string{"a.txt", "b.txt"}, written)

		// Second pass with SkipCreator: both files already exist -> both skipped.
		// ErrSkip is non-fatal: written is empty, err is nil (a no-overwrite
		// re-run is a silent no-op, matching the SkipCreator contract).
		skipMat, err := fragments.NewMaterializer(merged, "deps.yaml", dest, nil, writestrategy.SkipCreator{})
		require.NoError(t, err)
		written, err = skipMat.Materialize("base", nil)
		require.NoError(t, err, "ErrSkip is non-fatal")
		assert.Empty(t, written, "skipped files are excluded from written")
		// Original content survives (not clobbered).
		assert.Equal(t, "a", readDest(t, dest, "a.txt"))
		assert.Equal(t, "b", readDest(t, dest, "b.txt"))
	})

	t.Run("GreywallProfilesOnly", func(t *testing.T) {
		// A setup.yaml declaring only greywall_profiles (built-in references,
		// no entries) is the go bundle's setup.yaml shape: validate passes,
		// expand returns no entries, nothing is written, no error.
		t.Run("NoOpWrite", func(t *testing.T) {
			merged := fstest.MapFS{
				"go/setup.yaml": {Data: []byte("greywall_profiles: [go]\n")},
			}
			dest := t.TempDir()

			written, err := truncateMat(t, merged, "setup.yaml", dest, nil).Materialize("go", nil)
			require.NoError(t, err)
			assert.Nil(t, written, "greywall_profiles-only manifest writes nothing")
		})

		t.Run("EmptyManifestRejected", func(t *testing.T) {
			// A manifest declaring none of mirror/entries/greywall_profiles is
			// rejected as useless (validate extended for the new field).
			merged := fstest.MapFS{
				"go/setup.yaml": {Data: []byte("--- {}\n")},
			}
			dest := t.TempDir()

			_, err := truncateMat(t, merged, "setup.yaml", dest, nil).Materialize("go", nil)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "neither mirror, entries, nor greywall_profiles")
		})
	})
}

// backupMat builds a Materializer with BackupCreator (the setup-facet copy
// strategy: copy-if-different + backup on diff, no-op on identical).
func backupMat(t *testing.T, merged fs.FS, manifest, dest string, vars map[string]string) *fragments.Materializer {
	t.Helper()
	mat, err := fragments.NewMaterializer(merged, manifest, dest, vars, writestrategy.BackupCreator{})
	require.NoError(t, err)
	return mat
}

func TestBackupCreator(t *testing.T) {
	t.Run("AbsentTargetNoBackup", func(t *testing.T) {
		// First write to a non-existent target: no previous version to back up,
		// so no .cabin-bak is created. The target gets the new content.
		merged := fstest.MapFS{
			"base/setup.yaml":      {Data: []byte("entries:\n  - src: setup/conf.json\n    dst: conf.json\n")},
			"base/setup/conf.json": {Data: []byte(`{"v":1}`)},
		}
		dest := t.TempDir()

		_, err := backupMat(t, merged, "setup.yaml", dest, nil).Materialize("base", nil)
		require.NoError(t, err)

		assert.Equal(t, `{"v":1}`, readDest(t, dest, "conf.json"))
		_, statErr := os.Stat(filepath.Join(dest, "conf.json"+writestrategy.BackupSuffix))
		assert.True(t, os.IsNotExist(statErr), "no backup on first write")
	})

	t.Run("NoOpOnIdentical", func(t *testing.T) {
		// Second run with identical source: no-op (no .cabin-bak, target
		// unchanged). Idempotent on re-run — no backup churn.
		merged := fstest.MapFS{
			"base/setup.yaml":      {Data: []byte("entries:\n  - src: setup/conf.json\n    dst: conf.json\n")},
			"base/setup/conf.json": {Data: []byte(`{"v":1}`)},
		}
		dest := t.TempDir()
		mat := backupMat(t, merged, "setup.yaml", dest, nil)

		_, err := mat.Materialize("base", nil)
		require.NoError(t, err)

		// Second run: identical content → no-op, no backup.
		_, err = mat.Materialize("base", nil)
		require.NoError(t, err)

		assert.Equal(t, `{"v":1}`, readDest(t, dest, "conf.json"))
		_, statErr := os.Stat(filepath.Join(dest, "conf.json"+writestrategy.BackupSuffix))
		assert.True(t, os.IsNotExist(statErr), "no backup on identical re-run")
	})

	t.Run("BackupOnDiff", func(t *testing.T) {
		// Materialize once, change source content, materialize again: the
		// previous version is backed up (.cabin-bak), target has new content.
		merged := fstest.MapFS{
			"base/setup.yaml":      {Data: []byte("entries:\n  - src: setup/conf.json\n    dst: conf.json\n")},
			"base/setup/conf.json": {Data: []byte(`{"v":1}`)},
		}
		dest := t.TempDir()
		mat := backupMat(t, merged, "setup.yaml", dest, nil)

		_, err := mat.Materialize("base", nil)
		require.NoError(t, err)

		// Change source content and re-materialize.
		merged["base/setup/conf.json"].Data = []byte(`{"v":2}`)
		_, err = mat.Materialize("base", nil)
		require.NoError(t, err)

		// Target has new content.
		assert.Equal(t, `{"v":2}`, readDest(t, dest, "conf.json"))
		// Backup has old content.
		assert.Equal(t, `{"v":1}`, readDest(t, dest, "conf.json"+writestrategy.BackupSuffix))
	})

	t.Run("NoBackupWithTruncateCreator", func(t *testing.T) {
		// TruncateCreator (deps facet) never creates a backup, even on diff:
		// the deps destination is throwaway (.deps/), regenerated each build.
		merged := fstest.MapFS{
			"base/setup.yaml":      {Data: []byte("entries:\n  - src: setup/conf.json\n    dst: conf.json\n")},
			"base/setup/conf.json": {Data: []byte(`{"v":1}`)},
		}
		dest := t.TempDir()
		mat := truncateMat(t, merged, "setup.yaml", dest, nil)

		_, err := mat.Materialize("base", nil)
		require.NoError(t, err)

		merged["base/setup/conf.json"].Data = []byte(`{"v":2}`)
		_, err = mat.Materialize("base", nil)
		require.NoError(t, err)

		assert.Equal(t, `{"v":2}`, readDest(t, dest, "conf.json"))
		_, statErr := os.Stat(filepath.Join(dest, "conf.json"+writestrategy.BackupSuffix))
		assert.True(t, os.IsNotExist(statErr), "truncate never backs up")
	})
}

// TestResolveGreywallProfiles covers the derivation of the greywall profile
// list from active bundles: shipped profiles (detected by their learned/
// destination, name = stem of the rendered dst) plus built-in references
// (greywall_profiles: manifest field). The list is ordered (bundle order) and
// deduplicated by first occurrence.
func TestResolveGreywallProfiles(t *testing.T) {
	// A merged FS with all bundles' setup.yaml manifests. ResolveGreywallProfiles
	// reads only the manifests (not the shipped .json src files), so the src
	// files are omitted from the fixture.
	fullFS := fstest.MapFS{
		"base/setup.yaml":           {Data: []byte("entries:\n  - src: setup/greywall_profiles_workspace.json\n    dst: .config/greywall/learned/workspace.json\n")},
		"agent-pi/setup.yaml":       {Data: []byte("entries:\n  - src: setup/greywall_profiles_pi.json\n    dst: .config/greywall/learned/pi.json\n")},
		"agent-opencode/setup.yaml": {Data: []byte("entries:\n  - src: setup/greywall_profiles_opencode.json\n    dst: .config/greywall/learned/opencode.json\n")},
		"go/setup.yaml":             {Data: []byte("greywall_profiles: [go]\n")},
		"port-forward/setup.yaml":   {Data: []byte("entries:\n  - src: setup/forward.json.tmpl\n    dst: .config/greywall/learned/forward-{{.host}}-{{.port}}.json\n")},
	}

	t.Run("BaseOnly", func(t *testing.T) {
		bundles := []cabin.FeatureRef{{Name: "base"}}
		got, err := fragments.ResolveGreywallProfiles(fullFS, bundles, nil)
		require.NoError(t, err)
		assert.Equal(t, []string{"workspace"}, got)
	})

	t.Run("BasePlusPi", func(t *testing.T) {
		bundles := []cabin.FeatureRef{{Name: "base"}, {Name: "agent-pi"}}
		got, err := fragments.ResolveGreywallProfiles(fullFS, bundles, nil)
		require.NoError(t, err)
		assert.Equal(t, []string{"workspace", "pi"}, got)
	})

	t.Run("BasePlusPiPlusGoBuiltIn", func(t *testing.T) {
		// go references greywall's built-in go profile via greywall_profiles:
		// it ships no learned fragment, so the name comes from the manifest field.
		bundles := []cabin.FeatureRef{{Name: "base"}, {Name: "agent-pi"}, {Name: "go"}}
		got, err := fragments.ResolveGreywallProfiles(fullFS, bundles, nil)
		require.NoError(t, err)
		assert.Equal(t, []string{"workspace", "pi", "go"}, got)
	})

	t.Run("PortForwardTemplatedDst", func(t *testing.T) {
		// port-forward's dst is templated; the profile name is derived from
		// the rendered dst (forward-mariadb-3306), proving resolution uses the
		// rendered destination, not the source path.
		bundles := []cabin.FeatureRef{
			{Name: "base"},
			{Name: "agent-pi"},
			{Name: "port-forward", Attrs: map[string]any{"host": "mariadb", "port": "3306"}},
		}
		got, err := fragments.ResolveGreywallProfiles(fullFS, bundles, nil)
		require.NoError(t, err)
		assert.Equal(t, []string{"workspace", "pi", "forward-mariadb-3306"}, got)
	})

	t.Run("DedupPreservesFirstOccurrence", func(t *testing.T) {
		// Two bundles contributing the same profile (here base twice) yield
		// it once, in first-occurrence position. No special base-first handling:
		// base leads because ActiveBundles puts it first.
		bundles := []cabin.FeatureRef{{Name: "base"}, {Name: "agent-pi"}, {Name: "base"}}
		got, err := fragments.ResolveGreywallProfiles(fullFS, bundles, nil)
		require.NoError(t, err)
		assert.Equal(t, []string{"workspace", "pi"}, got)
	})

	t.Run("BundleWithoutSetupIsNoOp", func(t *testing.T) {
		// git-agent has no setup.yaml in the fixture: contributes nothing, no error.
		bundles := []cabin.FeatureRef{{Name: "base"}, {Name: "git-agent"}}
		got, err := fragments.ResolveGreywallProfiles(fullFS, bundles, nil)
		require.NoError(t, err)
		assert.Equal(t, []string{"workspace"}, got)
	})

	t.Run("BothAgentsUnion", func(t *testing.T) {
		// When both agents are active, the union includes both profiles in
		// bundle order (subagent pattern: opencode can carry pi).
		bundles := []cabin.FeatureRef{{Name: "base"}, {Name: "agent-pi"}, {Name: "agent-opencode"}}
		got, err := fragments.ResolveGreywallProfiles(fullFS, bundles, nil)
		require.NoError(t, err)
		assert.Equal(t, []string{"workspace", "pi", "opencode"}, got)
	})

	t.Run("FromEmbeddedPiGo", func(t *testing.T) {
		// The real embedded fragments resolved through the full fallback
		// chain, with bundles derived from a Taskfile header (matching the
		// real cabin internal greywall-profile path). pi-go's header is
		// agents:[pi] features:[git-agent, go] -> [workspace, pi, go], which
		// reproduces the profile list the pi wrapper consumes.
		embedFS, err := embedded.Fragments()
		require.NoError(t, err)
		merged, err := fragments.BuildLayers(nil, "", embedFS)
		require.NoError(t, err)

		header, err := cabin.ParseHeader([]byte(`ai-cabin:
  agents: [pi]
  features: [git-agent, go]
`))
		require.NoError(t, err)
		bundles := cabin.ActiveBundles(header)

		got, err := fragments.ResolveGreywallProfiles(merged, bundles, nil)
		require.NoError(t, err)
		assert.Equal(t, []string{"workspace", "pi", "go"}, got)
	})

	t.Run("FromEmbeddedWithPortForward", func(t *testing.T) {
		// A cabin declaring port-forward resolves the templated forward
		// profile name end-to-end through the embedded bundle — the gap
		// identified in Jalon 2 (forward-* not referenced by wrappers).
		embedFS, err := embedded.Fragments()
		require.NoError(t, err)
		merged, err := fragments.BuildLayers(nil, "", embedFS)
		require.NoError(t, err)

		header, err := cabin.ParseHeader([]byte(`ai-cabin:
  agents: [pi]
  features:
    - git-agent
    - go
    - port-forward: {port: 3306, host: mariadb}
`))
		require.NoError(t, err)
		bundles := cabin.ActiveBundles(header)

		got, err := fragments.ResolveGreywallProfiles(merged, bundles, nil)
		require.NoError(t, err)
		assert.Equal(t, []string{"workspace", "pi", "go", "forward-mariadb-3306"}, got)
	})
}
