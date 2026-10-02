// Package writestrategy provides the FileCreator strategy: how a destination
// file is opened for writing. Each implementation carries an overwrite policy
// (truncate, backup-on-diff, ...), selected by the caller at the call site
// so the copy/template engine stays policy-agnostic and a new policy is a new
// type, not a flag threaded through the engine.
package writestrategy

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"time"
)

// FilePerm is the mode used to create destination files:
// a plain 0666 so the umask reduces it to 0644 (writable by the owner,
// idempotent on re-run — re-opening an existing file for write succeeds,
// unlike a 0444 source).
const FilePerm os.FileMode = 0o666

// DirPerm is the mode used to create destination directories:
// a plain 0777 so the umask reduces it to 0755. Symmetric with FilePerm
// (0666/umask for files, 0777/umask for dirs).
const DirPerm os.FileMode = 0o777

// FileCreator abstracts how a destination file is opened for writing.
// Each implementation carries an overwrite policy, selected by the caller
// (a facet, a command) at the call site so the copy/template engine
// stays policy-agnostic. TruncateCreator overwrites immediately; BackupCreator
// backs up the previous version on diff. Create receives an absolute
// destination path, resolved by the caller, so the implementations are stateless.
type FileCreator interface {
	Create(name string) (io.WriteCloser, error)
}

// ErrSkip is returned by a FileCreator to signal that a destination file
// was skipped by the policy (SkipCreator: the file already exists).
// It is non-fatal: the engine continues and the file is excluded
// from the written list, distinct from an I/O error which is collected
// for the aggregated error. On ErrSkip the returned writer is non-nil
// (so a caller that defers Close before checking the error is safe)
// but need not be used.
var ErrSkip = errors.New("destination file skipped by the write policy")

// TruncateCreator is the overwrite policy: existing content is discarded.
type TruncateCreator struct{}

// Create opens name for writing, truncating any existing content.
func (TruncateCreator) Create(name string) (io.WriteCloser, error) {
	return os.OpenFile(name, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, FilePerm)
}

// SkipCreator is the no-overwrite policy:
// Create skips an existing destination (ErrSkip) and writes only when it is absent.
type SkipCreator struct{}

// Create returns a no-op writer and ErrSkip when name already exists; otherwise
// opens name for writing (truncate), like TruncateCreator.
func (SkipCreator) Create(name string) (io.WriteCloser, error) {
	if _, err := os.Stat(name); err == nil {
		return skipWriter{}, ErrSkip
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("stat %q: %w", name, err)
	}
	return os.OpenFile(name, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, FilePerm)
}

// skipWriter is a no-op WriteCloser returned by SkipCreator on an existing target:
// Write discards (the file is not modified), Close is a no-op.
type skipWriter struct{}

func (skipWriter) Write(p []byte) (int, error) { return len(p), nil }
func (skipWriter) Close() error                { return nil }

// BackupSuffix is the stable marker prefix appended to the target name to form
// a backup path (<name>.cabin-bak.<timestamp>). Kept constant so backups stay
// findable by pattern (find, gitignore, a future greywall deny).
const BackupSuffix = ".cabin-bak"

// backupTimeLayout formats the backup timestamp in UTC as a compact,
// lexicographically sortable value (YYYYMMDD-HHMMSS), so backups of the same
// target read chronologically with find | sort.
const backupTimeLayout = "20060102-150405"

// backupPath returns a non-colliding backup path for name:
// the BackupSuffix marker followed by a UTC timestamp, read at commit time.
// A same-second collision would silently drop the older generation,
// so a short increment (-2, -3, ...) disambiguates instead.
func backupPath(name string) (string, error) {
	base := name + BackupSuffix + "." + time.Now().UTC().Format(backupTimeLayout)
	p := base
	for i := 1; ; i++ {
		if _, err := os.Stat(p); errors.Is(err, fs.ErrNotExist) {
			return p, nil
		} else if err != nil {
			return "", fmt.Errorf("stat backup candidate %q: %w", p, err)
		}
		p = fmt.Sprintf("%s-%d", base, i)
	}
}

// BackupCreator is the copy-if-different policy:
// Close backs up the previous version before writing the new content;
// an identical target is a no-op.
type BackupCreator struct{}

// Create returns a backupWriter that buffers writes and commits at Close.
func (BackupCreator) Create(name string) (io.WriteCloser, error) {
	return &backupWriter{name: name, buf: new(bytes.Buffer)}, nil
}

// backupWriter buffers writes; Close commits them (see BackupCreator).
type backupWriter struct {
	name   string
	buf    *bytes.Buffer
	closed bool
}

func (w *backupWriter) Write(p []byte) (int, error) {
	if w.closed {
		return 0, errors.New("write on closed backup writer")
	}
	return w.buf.Write(p)
}

// Close commits the buffered content with copy-if-different + backup semantics.
func (w *backupWriter) Close() error {
	if w.closed {
		return nil
	}
	w.closed = true
	newContent := w.buf.Bytes()

	oldContent, err := os.ReadFile(w.name)
	switch {
	case err == nil:
		if bytes.Equal(oldContent, newContent) {
			return nil
		}
		bak, err := backupPath(w.name)
		if err != nil {
			return err
		}
		if err := os.Rename(w.name, bak); err != nil {
			return fmt.Errorf("backup %q: %w", bak, err)
		}
	case errors.Is(err, fs.ErrNotExist):
		// No existing file: nothing to back up, just write.
	default:
		return fmt.Errorf("read existing %q: %w", w.name, err)
	}

	if err := os.WriteFile(w.name, newContent, FilePerm); err != nil {
		return fmt.Errorf("write %q: %w", w.name, err)
	}
	return nil
}
