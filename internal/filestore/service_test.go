package filestore

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

func TestReadRejectsIgnoredPath(t *testing.T) {
	root := t.TempDir()
	mustMkdirAll(t, filepath.Join(root, ".git"))
	if err := os.WriteFile(filepath.Join(root, ".git", "config"), []byte("secret"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	service := NewService()

	_, err := service.Read(root, ".git/config")
	if !errors.Is(err, ErrIgnoredPath) {
		t.Fatalf("expected ErrIgnoredPath, got %v", err)
	}
}

func TestReadRejectsLargeFile(t *testing.T) {
	root := t.TempDir()
	content := make([]byte, MaxReadableFileSize+1)
	if err := os.WriteFile(filepath.Join(root, "large.txt"), content, 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	service := NewService()

	_, err := service.Read(root, "large.txt")
	if !errors.Is(err, ErrFileTooLarge) {
		t.Fatalf("expected ErrFileTooLarge, got %v", err)
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
	if entries[0].ModifiedAt.IsZero() {
		t.Fatalf("expected modified time, got %+v", entries[0])
	}
}

func TestListSkipsIgnoredDirsSymlinksAndLargeFiles(t *testing.T) {
	root := t.TempDir()
	mustMkdirAll(t, filepath.Join(root, ".git"))
	mustMkdirAll(t, filepath.Join(root, "node_modules"))
	mustMkdirAll(t, filepath.Join(root, "build"))
	if err := os.WriteFile(filepath.Join(root, "small.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatalf("write small file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "large.txt"), make([]byte, MaxReadableFileSize+1), 0o644); err != nil {
		t.Fatalf("write large file: %v", err)
	}
	if err := os.Symlink(filepath.Join(root, "small.txt"), filepath.Join(root, "link.txt")); err != nil {
		t.Fatalf("create symlink: %v", err)
	}
	service := NewService()

	entries, err := service.List(root, ".")
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(entries) != 1 || entries[0].Path != "small.txt" {
		t.Fatalf("unexpected entries: %+v", entries)
	}
}

func TestSearchFindsFilenameAndContent(t *testing.T) {
	root := t.TempDir()
	mustMkdirAll(t, filepath.Join(root, "src"))
	if err := os.WriteFile(filepath.Join(root, "src", "note.txt"), []byte("first\nhello world\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	service := NewService()

	results, err := service.Search(root, "hello")
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(results) != 1 || results[0].Path != "src/note.txt" || results[0].Kind != "content" || results[0].Line != 2 {
		t.Fatalf("unexpected results: %+v", results)
	}
}

func TestSearchSkipsIgnoredDirsSymlinksAndLargeFiles(t *testing.T) {
	root := t.TempDir()
	mustMkdirAll(t, filepath.Join(root, ".git"))
	mustMkdirAll(t, filepath.Join(root, "node_modules"))
	mustMkdirAll(t, filepath.Join(root, "build"))
	if err := os.WriteFile(filepath.Join(root, ".git", "secret.txt"), []byte("needle"), 0o644); err != nil {
		t.Fatalf("write ignored file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "node_modules", "package.txt"), []byte("needle"), 0o644); err != nil {
		t.Fatalf("write ignored file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "build", "bundle.txt"), []byte("needle"), 0o644); err != nil {
		t.Fatalf("write ignored file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "large.txt"), make([]byte, MaxReadableFileSize+1), 0o644); err != nil {
		t.Fatalf("write large file: %v", err)
	}
	if err := os.Symlink(filepath.Join(root, ".git", "secret.txt"), filepath.Join(root, "link.txt")); err != nil {
		t.Fatalf("create symlink: %v", err)
	}
	service := NewService()

	results, err := service.Search(root, "needle")
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected no results, got %+v", results)
	}
}

func TestIndexReturnsRecursiveFileMetadata(t *testing.T) {
	root := t.TempDir()
	mustMkdirAll(t, filepath.Join(root, "src"))
	if err := os.WriteFile(filepath.Join(root, "src", "note.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("readme"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	service := NewService()

	entries, err := service.Index(root)
	if err != nil {
		t.Fatalf("Index returned error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 indexed files, got %+v", entries)
	}
	if entries[0].Path != "README.md" || entries[0].Size != 6 || entries[0].ModifiedAt.IsZero() {
		t.Fatalf("unexpected first entry: %+v", entries[0])
	}
	if entries[1].Path != "src/note.txt" || entries[1].Size != 5 || entries[1].ModifiedAt.IsZero() {
		t.Fatalf("unexpected second entry: %+v", entries[1])
	}
}

func TestIndexSkipsIgnoredDirsSymlinksAndLargeFiles(t *testing.T) {
	root := t.TempDir()
	mustMkdirAll(t, filepath.Join(root, ".git"))
	mustMkdirAll(t, filepath.Join(root, "node_modules"))
	if err := os.WriteFile(filepath.Join(root, ".git", "config"), []byte("secret"), 0o644); err != nil {
		t.Fatalf("write ignored file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "node_modules", "package.txt"), []byte("ignored"), 0o644); err != nil {
		t.Fatalf("write ignored file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "large.txt"), make([]byte, MaxReadableFileSize+1), 0o644); err != nil {
		t.Fatalf("write large file: %v", err)
	}
	if err := os.Symlink(filepath.Join(root, ".git", "config"), filepath.Join(root, "link.txt")); err != nil {
		t.Fatalf("create symlink: %v", err)
	}
	service := NewService()

	entries, err := service.Index(root)
	if err != nil {
		t.Fatalf("Index returned error: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected no indexed files, got %+v", entries)
	}
}

func mustMkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}
