package formats

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

var errTestWrite = errors.New("write failed")

func TestWriteFileAtomicReplacesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "song.ttml")
	if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}

	err := WriteFileAtomic(path, func(w io.Writer) error {
		_, err := io.WriteString(w, "new")

		return err
	})
	if err != nil {
		t.Fatalf("WriteFileAtomic() error: %v", err)
	}
	if got, _ := os.ReadFile(path); string(got) != "new" {
		t.Fatalf("file holds %q, want %q", got, "new")
	}
	if info, _ := os.Stat(path); info.Mode().Perm() != 0o600 {
		t.Errorf("permissions = %v, want the old file's 0600", info.Mode().Perm())
	}
	assertOnlyFile(t, dir, "song.ttml")
}

func TestWriteFileAtomicKeepsOldFileOnFailure(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "song.ttml")
	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := WriteFileAtomic(path, func(w io.Writer) error {
		_, _ = io.WriteString(w, "half")

		return errTestWrite
	})
	if !errors.Is(err, errTestWrite) {
		t.Fatalf("WriteFileAtomic() error = %v, want %v", err, errTestWrite)
	}
	if got, _ := os.ReadFile(path); string(got) != "old" {
		t.Fatalf("file holds %q after a failed write, want %q", got, "old")
	}
	assertOnlyFile(t, dir, "song.ttml")
}

func TestWriteFileAtomicNewFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "new.ttml")

	if err := WriteFileAtomic(path, func(w io.Writer) error { _, err := io.WriteString(w, "x"); return err }); err != nil {
		t.Fatalf("WriteFileAtomic() error: %v", err)
	}
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0o644 {
		t.Fatalf("new file: %v, permissions %v; want 0644", err, info.Mode().Perm())
	}
}

// assertOnlyFile checks that no temporary file was left behind.
func assertOnlyFile(t *testing.T, dir, name string) {
	t.Helper()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != name {
		names := make([]string, len(entries))
		for i, e := range entries {
			names[i] = e.Name()
		}
		t.Fatalf("directory holds %v, want only %s", names, name)
	}
}
