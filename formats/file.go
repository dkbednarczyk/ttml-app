package formats

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// WriteFileAtomic writes path through a temporary file in the same directory,
// then renames it into place, so a failed write never leaves a partial file.
// An existing file keeps its permissions.
func WriteFileAtomic(path string, write func(io.Writer) error) (err error) {
	mode := fs.FileMode(0o644)
	if info, statErr := os.Stat(path); statErr == nil {
		mode = info.Mode().Perm()
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary file: %w", err)
	}
	closed := false
	defer func() {
		if err == nil {
			return
		}
		// Best effort: the write already failed, and that is the error to report.
		if !closed {
			_ = tmp.Close()
		}
		_ = os.Remove(tmp.Name())
	}()

	if err := write(tmp); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("sync temporary file: %w", err)
	}
	if err := tmp.Chmod(mode); err != nil {
		return fmt.Errorf("set file permissions: %w", err)
	}
	closed = true
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temporary file: %w", err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("replace %s: %w", filepath.Base(path), err)
	}

	return nil
}
