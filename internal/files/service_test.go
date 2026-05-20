package files

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestReadRejectsTraversal(t *testing.T) {
	root := t.TempDir()
	service := NewService()

	_, err := service.Read(root, "../secret.txt")
	if !errors.Is(err, ErrOutsideRoot) {
		t.Fatalf("expected ErrOutsideRoot, got %v", err)
	}
}

func TestReadRejectsSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("secret"), 0o644); err != nil {
		t.Fatalf("write outside file: %v", err)
	}
	if err := os.Symlink(filepath.Join(outside, "secret.txt"), filepath.Join(root, "link.txt")); err != nil {
		t.Fatalf("create symlink: %v", err)
	}
	service := NewService()

	_, err := service.Read(root, "link.txt")
	if !errors.Is(err, ErrOutsideRoot) {
		t.Fatalf("expected ErrOutsideRoot, got %v", err)
	}
}

func TestReadReturnsTextFile(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "note.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	service := NewService()

	file, err := service.Read(root, "note.txt")
	if err != nil {
		t.Fatalf("Read returned error: %v", err)
	}
	if file.Path != "note.txt" || file.Content != "hello" {
		t.Fatalf("unexpected file: %+v", file)
	}
}

func TestListReturnsEntries(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "note.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	service := NewService()

	entries, err := service.List(root, ".")
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(entries) != 1 || entries[0].Path != "note.txt" {
		t.Fatalf("unexpected entries: %+v", entries)
	}
}
