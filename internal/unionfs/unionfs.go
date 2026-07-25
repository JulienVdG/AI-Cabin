// Package unionfs allows multiple file systems to be read as a union.
//
// New returns a read-only fs.FS over ordered layers (highest priority first):
// when several layers provide the same path, the one listed earliest wins
// (first-wins), for both Open and ReadDir entries. ReadDir unions entries
// across layers with first-wins deduplication per entry name.
//
// The returned value implements fs.FS, fs.ReadDirFS and fs.StatFS, so the
// stdlib helpers (fs.WalkDir, fs.ReadDir, fs.Stat, fs.ReadFile) work over the
// union via type assertion. The concrete type is unexported: callers program
// to fs.FS.
//
// Resolution results are cached lazily: a ReadDir pre-populates the cache for
// its entries, so subsequent Opens are O(1). A negative lookup (no layer has
// the path) is memoized too, so a missing path is not re-walked on the next
// access. Layers are assumed read-only for the lifetime of the union: there
// is no cache invalidation.
//
// Example:
//
//	merged := unionfs.New(orgDir, cabinDir, embedFS)
//	err := fs.WalkDir(merged, "agent-pi", func(p string, d fs.DirEntry, err error) error {
//		// first-wins: a file in orgDir shadows the embedded one
//		...
//		return nil
//	})
package unionfs

import (
	"io/fs"
	"path"
)

// New returns a read-only union of the given layers, ordered highest priority
// first (first-wins for Open and ReadDir entries). The concrete type is
// unexported: callers program to fs.FS (and its optional ReadDirFS / StatFS).
func New(layers ...fs.FS) fs.FS {
	return &unionFS{
		layers: layers,
		cache:  make(map[string]int),
	}
}

// unionFS caches resolution results so a ReadDir pre-populates the cache and
// subsequent Opens are O(1). Layers are assumed read-only for the lifetime of
// the union: there is no cache invalidation.
type unionFS struct {
	layers []fs.FS
	cache  map[string]int // path -> owning layer index, or notFound; lazy
}

// notFound caches a negative resolution (no layer has the path), distinct
// from a cache miss (key absent = never looked up).
const notFound = -1

func (u *unionFS) Open(name string) (fs.File, error) {
	idx, err := u.resolve(name)
	if err != nil {
		return nil, err
	}
	return u.layers[idx].Open(name)
}

// Stat implements fs.StatFS, required by fs.WalkDir to walk the root.
func (u *unionFS) Stat(name string) (fs.FileInfo, error) {
	idx, err := u.resolve(name)
	if err != nil {
		return nil, err
	}
	return fs.Stat(u.layers[idx], name)
}

func (u *unionFS) ReadDir(name string) ([]fs.DirEntry, error) {
	var entries []fs.DirEntry
	seen := make(map[string]bool)
	var firstErr error
	dirOwner := -1
	for i, layer := range u.layers {
		list, err := fs.ReadDir(layer, name)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if dirOwner == -1 {
			dirOwner = i
		}
		for _, e := range list {
			if seen[e.Name()] {
				continue
			}
			seen[e.Name()] = true
			entries = append(entries, e)
			u.cache[path.Join(name, e.Name())] = i
		}
	}
	if dirOwner == -1 {
		return nil, firstErr
	}
	u.cache[name] = dirOwner
	return entries, nil
}

// resolve returns the owning layer index for name, using and populating the
// cache. A negative result is memoized as notFound so it is not re-walked.
func (u *unionFS) resolve(name string) (int, error) {
	if idx, ok := u.cache[name]; ok {
		if idx == notFound {
			return 0, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
		}
		return idx, nil
	}

	for i, layer := range u.layers {
		if _, err := fs.Stat(layer, name); err == nil {
			u.cache[name] = i
			return i, nil
		}
	}

	u.cache[name] = notFound
	return 0, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
}
