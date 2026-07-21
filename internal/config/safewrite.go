package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// filePerm is the default permission for config files written by this package.
// Centralized here so writers are the single source of truth for perms.
const filePerm os.FileMode = 0o644

// dirPerm is the default permission for directories created by this package.
const dirPerm os.FileMode = 0o755

// FileWriter writes a file by path with the given permission.
// The signature matches os.WriteFile so implementations can delegate directly.
// Injectable via ConfigService for write-error tests; the default implementation
// (AtomicFileWriter) writes atomically.
type FileWriter interface {
	WriteFile(path string, data []byte, perm os.FileMode) error
}

// AtomicFileWriter implements FileWriter using os.* with an atomic temp file +
// rename, so a crash mid-write never leaves a partially written config file.
//
// Atomicity is achieved by writing to a temp file in the SAME directory as the
// target (same directory == same filesystem, which is required for os.Rename to
// be atomic on POSIX; a temp file in /tmp could cross a filesystem boundary and
// fall back to a non-atomic copy). The sequence is:
//  1. MkdirAll the parent dir (so writing into a non-existent subdir works).
//  2. Create a temp file in that dir, write data, chmod, fsync.
//  3. os.Rename the temp file over the target (atomic on POSIX).
//  4. On any error, remove the temp file before returning (deferred).
//
// fsync before rename is what actually makes the write durable: without it the
// rename could reach disk before the file contents. Best-effort; dir fsync is
// intentionally omitted (low value for local user config, complicates tests).
type AtomicFileWriter struct{}

// WriteFile writes data to path atomically.
func (AtomicFileWriter) WriteFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, dirPerm); err != nil {
		return fmt.Errorf("failed to create directory %q: %w", dir, err)
	}

	// Temp file in the same dir (same filesystem → atomic rename). The prefix is
	// ".<basename>-tmp" so leftover temp files are visually associated with their
	// target and easy to spot/clean.
	prefix := "." + filepath.Base(path) + "-tmp*"
	tmp, err := os.CreateTemp(dir, prefix)
	if err != nil {
		return fmt.Errorf("failed to create temp file in %q: %w", dir, err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("failed to write temp file %q: %w", tmpName, err)
	}
	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("failed to chmod temp file %q: %w", tmpName, err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("failed to sync temp file %q: %w", tmpName, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("failed to close temp file %q: %w", tmpName, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("failed to rename %q to %q: %w", tmpName, path, err)
	}
	return nil
}
