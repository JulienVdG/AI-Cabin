package fragments

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
)

// FileCreator abstracts how a destination file is opened for writing.
// TruncateCreator overwrites immediately (deps facet); BackupCreator buffers
// and compares at Close, backing up the previous version on diff (setup facet).
// Create receives an absolute destination path, resolved by the caller, so both
// implementations are stateless.
type FileCreator interface {
	Create(name string) (io.WriteCloser, error)
}

// TruncateCreator opens the destination with O_CREATE|O_WRONLY|O_TRUNC.
type TruncateCreator struct{}

// Create opens name for writing, truncating any existing content.
func (TruncateCreator) Create(name string) (io.WriteCloser, error) {
	return os.OpenFile(name, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, filePerm)
}

// BackupSuffix is appended to the target name to form the single-slot backup
// path (<name>.cabin-bak).
const BackupSuffix = ".cabin-bak"

// BackupCreator returns a backupWriter that buffers writes, then at Close
// compares the buffered content against the existing target: identical is a
// no-op; different or absent backs up the previous version then writes the new
// content. Stateless: the destination path is passed to Create.
type BackupCreator struct{}

// Create returns a backupWriter that buffers writes and commits at Close.
func (BackupCreator) Create(name string) (io.WriteCloser, error) {
	return &backupWriter{name: name, buf: new(bytes.Buffer)}, nil
}

// backupWriter buffers writes and commits at Close with copy-if-different +
// backup semantics.
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

// Close commits the buffered content: read the existing target, compare, and
// either no-op (identical) or back up + write (different or absent).
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
		bak := w.name + BackupSuffix
		if err := os.Rename(w.name, bak); err != nil {
			return fmt.Errorf("backup %q: %w", bak, err)
		}
	case errors.Is(err, fs.ErrNotExist):
		// No existing file: nothing to back up, just write.
	default:
		return fmt.Errorf("read existing %q: %w", w.name, err)
	}

	if err := os.WriteFile(w.name, newContent, filePerm); err != nil {
		return fmt.Errorf("write %q: %w", w.name, err)
	}
	return nil
}
