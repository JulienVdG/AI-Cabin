package unionfs_test

import (
	"io/fs"
	"path"
	"sort"
	"testing"
	"testing/fstest"

	mock_fs "github.com/JulienVdG/AI-Cabin/internal/mocks/io/fs"
	"github.com/JulienVdG/AI-Cabin/internal/unionfs"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildLayers returns three MapFS layers: high (highest priority), mid, low
// (embedded fallback). shared.txt is present in all three to exercise shadowing.
func buildLayers() (high, mid, low fs.FS) {
	high = fstest.MapFS{
		"shared.txt":    {Data: []byte("high")},
		"only-high.txt": {Data: []byte("high-only")},
		"dir/a.txt":     {Data: []byte("a-high")},
	}
	mid = fstest.MapFS{
		"shared.txt":         {Data: []byte("mid")},
		"only-mid.txt":       {Data: []byte("mid-only")},
		"dir/b.txt":          {Data: []byte("b-mid")},
		"only-mid-dir/x.txt": {Data: []byte("x-mid")},
	}
	low = fstest.MapFS{
		"shared.txt":   {Data: []byte("low")},
		"only-low.txt": {Data: []byte("low-only")},
	}
	return high, mid, low
}

func readAll(t *testing.T, fsys fs.FS, name string) string {
	t.Helper()
	b, err := fs.ReadFile(fsys, name)
	require.NoError(t, err)
	return string(b)
}

func TestUnionFS_Open(t *testing.T) {
	merged := unionfs.New(buildLayers())

	cases := []struct {
		name string
		path string
		want string
	}{
		{"SharedFileResolvesToHighestLayer", "shared.txt", "high"},
		{"FileOnlyInHigh", "only-high.txt", "high-only"},
		{"FileOnlyInMid", "only-mid.txt", "mid-only"},
		{"FileOnlyInLow", "only-low.txt", "low-only"},
		{"FileInSharedDir", "dir/a.txt", "a-high"},
		{"FileInMidOnlyDir", "dir/b.txt", "b-mid"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, readAll(t, merged, tc.path))
		})
	}

	t.Run("MissingReturnsErrNotExist", func(t *testing.T) {
		merged := unionfs.New(buildLayers())

		_, err := merged.Open("does-not-exist.txt")
		require.Error(t, err)
		assert.ErrorIs(t, err, fs.ErrNotExist)
	})
}

func TestUnionFS_ReadDir(t *testing.T) {
	merged := unionfs.New(buildLayers())

	t.Run("RootUnionDedup", func(t *testing.T) {
		entries, err := fs.ReadDir(merged, ".")
		require.NoError(t, err)

		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		sort.Strings(names)
		assert.Equal(t, []string{"dir", "only-high.txt", "only-low.txt", "only-mid-dir", "only-mid.txt", "shared.txt"}, names)
	})

	t.Run("SubdirMergedAcrossLayers", func(t *testing.T) {
		entries, err := fs.ReadDir(merged, "dir")
		require.NoError(t, err)

		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		sort.Strings(names)
		assert.Equal(t, []string{"a.txt", "b.txt"}, names)
	})

	t.Run("DirAbsentEverywhereErrors", func(t *testing.T) {
		_, err := fs.ReadDir(merged, "no-such-dir")
		require.Error(t, err)
		assert.ErrorIs(t, err, fs.ErrNotExist)
	})
}

func TestUnionFS_Stat(t *testing.T) {
	merged := unionfs.New(buildLayers())

	t.Run("Root", func(t *testing.T) {
		info, err := fs.Stat(merged, ".")
		require.NoError(t, err)
		assert.True(t, info.IsDir())
	})

	t.Run("FileResolvesToOwningLayer", func(t *testing.T) {
		info, err := fs.Stat(merged, "shared.txt")
		require.NoError(t, err)
		assert.Equal(t, "shared.txt", info.Name())
		assert.False(t, info.IsDir())
	})

	t.Run("ResolveErrorReturnsErrNotExist", func(t *testing.T) {
		mockLayer := mock_fs.NewStatFS(t)
		mockLayer.EXPECT().Stat("a.txt").Return(nil, fs.ErrInvalid)
		merged := unionfs.New(mockLayer)

		_, err := fs.Stat(merged, "a.txt")
		require.Error(t, err)
		assert.ErrorIs(t, err, fs.ErrNotExist)
	})
}

func TestUnionFS_WalkDir(t *testing.T) {
	merged := unionfs.New(buildLayers())

	var got []string
	err := fs.WalkDir(merged, ".", func(p string, _ fs.DirEntry, err error) error {
		require.NoError(t, err)
		got = append(got, p)
		return nil
	})
	require.NoError(t, err)

	sort.Strings(got)
	want := []string{
		".", "dir", "dir/a.txt", "dir/b.txt",
		"only-high.txt", "only-low.txt",
		"only-mid-dir", "only-mid-dir/x.txt",
		"only-mid.txt", "shared.txt",
	}
	assert.Equal(t, want, got)
}

func TestUnionFS_ReadFile(t *testing.T) {
	merged := unionfs.New(buildLayers())

	b, err := fs.ReadFile(merged, path.Join("dir", "b.txt"))
	require.NoError(t, err)
	assert.Equal(t, "b-mid", string(b))
}

// countStatFS counts fs.Stat calls into its layer, so cache memoization is
// observable: a cache hit performs zero layer Stats.
type countStatFS struct {
	fs.FS
	stats *int
}

func (c countStatFS) Stat(name string) (fs.FileInfo, error) {
	*c.stats++
	return fs.Stat(c.FS, name)
}

func (c countStatFS) ReadDir(name string) ([]fs.DirEntry, error) {
	return fs.ReadDir(c.FS, name)
}

func TestUnionFS_ResolveCaches(t *testing.T) {
	t.Run("NegativeLookupIsMemoized", func(t *testing.T) {
		var stats int
		merged := unionfs.New(countStatFS{
			FS:    fstest.MapFS{"a.txt": {Data: []byte("a")}},
			stats: &stats,
		})

		_, err := merged.Open("missing.txt")
		require.Error(t, err)
		require.Greater(t, stats, 0)

		_, err = merged.Open("missing.txt")
		require.Error(t, err)
		assert.Equal(t, stats, 1, "second lookup hits the negative cache (no re-walk)")
	})

	t.Run("PositiveLookupIsMemoized", func(t *testing.T) {
		var stats int
		merged := unionfs.New(countStatFS{
			FS:    fstest.MapFS{"a.txt": {Data: []byte("a")}},
			stats: &stats,
		})

		_, err := fs.ReadFile(merged, "a.txt")
		require.NoError(t, err)
		require.Greater(t, stats, 0)

		_, err = fs.ReadFile(merged, "a.txt")
		require.NoError(t, err)
		assert.Equal(t, stats, 1, "second lookup hits the positive cache (no re-walk)")
	})
}
