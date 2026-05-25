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

func TestReadWithOptionsReturnsLineRange(t *testing.T) {
	root := t.TempDir()
	content := "one\ntwo\nthree\nfour\n"
	if err := os.WriteFile(filepath.Join(root, "note.txt"), []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	service := NewService()

	tests := []struct {
		name string
		opts ReadOptions
		want string
	}{
		{name: "full file", opts: ReadOptions{}, want: content},
		{name: "bounded", opts: ReadOptions{StartLine: 2, EndLine: 3}, want: "two\nthree\n"},
		{name: "open ended", opts: ReadOptions{StartLine: 3}, want: "three\nfour\n"},
		{name: "past eof", opts: ReadOptions{StartLine: 9}, want: ""},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			file, err := service.ReadWithOptions(root, "note.txt", testCase.opts)
			if err != nil {
				t.Fatalf("ReadWithOptions returned error: %v", err)
			}
			if file.Content != testCase.want {
				t.Fatalf("expected %q, got %q", testCase.want, file.Content)
			}
		})
	}
}

func TestReadWithOptionsRejectsInvalidLineRange(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "note.txt"), []byte("one\ntwo\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	service := NewService()

	for _, opts := range []ReadOptions{
		{StartLine: -1},
		{EndLine: -1},
		{StartLine: 3, EndLine: 2},
	} {
		_, err := service.ReadWithOptions(root, "note.txt", opts)
		if !errors.Is(err, ErrInvalidLineRange) {
			t.Fatalf("expected ErrInvalidLineRange for %+v, got %v", opts, err)
		}
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

func TestWriteUpdatesExistingTextFile(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "note.txt")
	if err := os.WriteFile(path, []byte("before"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	service := NewService()

	file, err := service.Write(root, "note.txt", "after")
	if err != nil {
		t.Fatalf("Write returned error: %v", err)
	}
	if file.Path != "note.txt" || file.Content != "after" {
		t.Fatalf("unexpected file: %+v", file)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(got) != "after" {
		t.Fatalf("expected written content, got %q", got)
	}
}

func TestWriteRejectsMissingFile(t *testing.T) {
	root := t.TempDir()
	service := NewService()

	_, err := service.Write(root, "missing.txt", "after")
	if err == nil {
		t.Fatal("expected missing file error")
	}
}

func TestWriteRejectsUnsafePathsAndContent(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("secret"), 0o644); err != nil {
		t.Fatalf("write outside file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "note.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "binary.txt"), []byte{0, 1, 2}, 0o644); err != nil {
		t.Fatalf("write binary file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".env"), []byte("TOKEN=value"), 0o644); err != nil {
		t.Fatalf("write env file: %v", err)
	}
	if err := os.Symlink(filepath.Join(root, "note.txt"), filepath.Join(root, "link.txt")); err != nil {
		t.Fatalf("create symlink: %v", err)
	}
	mustMkdirAll(t, filepath.Join(root, "nested"))
	if err := os.Symlink(filepath.Join(root, "note.txt"), filepath.Join(root, "nested", "link.txt")); err != nil {
		t.Fatalf("create nested symlink: %v", err)
	}
	service := NewService()

	tests := []struct {
		name    string
		path    string
		content string
		want    error
	}{
		{name: "traversal", path: "../secret.txt", content: "after", want: ErrOutsideRoot},
		{name: "secret", path: ".env", content: "after", want: ErrSecretPath},
		{name: "symlink", path: "link.txt", content: "after", want: ErrSymlinkPath},
		{name: "nested symlink", path: "nested/link.txt", content: "after", want: ErrSymlinkPath},
		{name: "existing binary", path: "binary.txt", content: "after", want: ErrNotTextFile},
		{name: "binary content", path: "note.txt", content: "after\x00", want: ErrNotTextFile},
		{name: "large content", path: "note.txt", content: string(make([]byte, MaxReadableFileSize+1)), want: ErrFileTooLarge},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := service.Write(root, testCase.path, testCase.content)
			if !errors.Is(err, testCase.want) {
				t.Fatalf("expected %v, got %v", testCase.want, err)
			}
		})
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

func TestSearchWithOptionsScopesResults(t *testing.T) {
	root := t.TempDir()
	mustMkdirAll(t, filepath.Join(root, "src"))
	mustMkdirAll(t, filepath.Join(root, "docs"))
	if err := os.WriteFile(filepath.Join(root, "src", "note.txt"), []byte("needle in src\n"), 0o644); err != nil {
		t.Fatalf("write src file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "note.txt"), []byte("needle in docs\n"), 0o644); err != nil {
		t.Fatalf("write docs file: %v", err)
	}
	service := NewService()

	results, err := service.SearchWithOptions(root, "needle", SearchOptions{Path: "src"})
	if err != nil {
		t.Fatalf("SearchWithOptions returned error: %v", err)
	}
	if len(results) != 1 || results[0].Path != "src/note.txt" {
		t.Fatalf("expected only src result, got %+v", results)
	}

	results, err = service.SearchWithOptions(root, "needle", SearchOptions{Path: "docs/note.txt"})
	if err != nil {
		t.Fatalf("SearchWithOptions file scope returned error: %v", err)
	}
	if len(results) != 1 || results[0].Path != "docs/note.txt" {
		t.Fatalf("expected only docs file result, got %+v", results)
	}
}

func TestSearchWithOptionsRejectsUnsafeScope(t *testing.T) {
	root := t.TempDir()
	mustMkdirAll(t, filepath.Join(root, ".git"))
	if err := os.WriteFile(filepath.Join(root, ".git", "config"), []byte("needle"), 0o644); err != nil {
		t.Fatalf("write ignored file: %v", err)
	}
	service := NewService()

	_, err := service.SearchWithOptions(root, "needle", SearchOptions{Path: "../outside"})
	if !errors.Is(err, ErrOutsideRoot) {
		t.Fatalf("expected ErrOutsideRoot, got %v", err)
	}
	_, err = service.SearchWithOptions(root, "needle", SearchOptions{Path: ".git"})
	if !errors.Is(err, ErrIgnoredPath) {
		t.Fatalf("expected ErrIgnoredPath, got %v", err)
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
