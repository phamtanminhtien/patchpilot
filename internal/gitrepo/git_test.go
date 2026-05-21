package gitrepo

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestStatusReturnsPorcelain(t *testing.T) {
	root := initGitRepo(t)
	if err := os.WriteFile(filepath.Join(root, "new.txt"), []byte("new"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	client := NewClient()

	status, err := client.Status(context.Background(), root)
	if err != nil {
		t.Fatalf("Status returned error: %v", err)
	}
	if !strings.Contains(status.Porcelain, "?? new.txt") {
		t.Fatalf("expected untracked file in status, got %q", status.Porcelain)
	}
}

func TestRepositoryRootReturnsGitTopLevel(t *testing.T) {
	root := initGitRepo(t)
	nested := filepath.Join(root, "nested")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("mkdir nested: %v", err)
	}
	client := NewClient()

	repositoryRoot, err := client.RepositoryRoot(context.Background(), nested)
	if err != nil {
		t.Fatalf("RepositoryRoot returned error: %v", err)
	}
	expectedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatalf("EvalSymlinks returned error: %v", err)
	}
	if repositoryRoot != expectedRoot {
		t.Fatalf("expected %q, got %q", expectedRoot, repositoryRoot)
	}
}

func TestRepositoryRootRejectsNonGitDirectory(t *testing.T) {
	client := NewClient()

	_, err := client.RepositoryRoot(context.Background(), t.TempDir())
	if !errors.Is(err, ErrNotRepository) {
		t.Fatalf("expected ErrNotRepository, got %v", err)
	}
}

func TestDiffReturnsPathDiff(t *testing.T) {
	root := initGitRepo(t)
	path := filepath.Join(root, "tracked.txt")
	if err := os.WriteFile(path, []byte("before\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	run(t, root, "git", "add", "tracked.txt")
	run(t, root, "git", "-c", "user.email=test@example.com", "-c", "user.name=Test", "commit", "-m", "initial")
	if err := os.WriteFile(path, []byte("after\n"), 0o644); err != nil {
		t.Fatalf("modify file: %v", err)
	}
	client := NewClient()

	diff, err := client.Diff(context.Background(), root, "tracked.txt")
	if err != nil {
		t.Fatalf("Diff returned error: %v", err)
	}
	if !strings.Contains(diff.Diff, "-before") || !strings.Contains(diff.Diff, "+after") {
		t.Fatalf("expected file diff, got %q", diff.Diff)
	}
}

func TestDiffRejectsTraversalPath(t *testing.T) {
	root := initGitRepo(t)
	client := NewClient()

	_, err := client.Diff(context.Background(), root, "../outside.txt")
	if !errors.Is(err, ErrInvalidPath) {
		t.Fatalf("expected ErrInvalidPath, got %v", err)
	}
}

func initGitRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	run(t, root, "git", "init")
	return root
}

func run(t *testing.T, dir, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v failed: %v\n%s", name, args, err, output)
	}
}
