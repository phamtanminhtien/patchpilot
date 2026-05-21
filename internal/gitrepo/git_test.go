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

func TestStatusExpandsUntrackedDirectories(t *testing.T) {
	root := initGitRepo(t)
	nested := filepath.Join(root, "internal", "runner", "testdata")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("mkdir nested: %v", err)
	}
	if err := os.WriteFile(filepath.Join(nested, "event.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("write nested file: %v", err)
	}
	client := NewClient()

	status, err := client.Status(context.Background(), root)
	if err != nil {
		t.Fatalf("Status returned error: %v", err)
	}
	if !strings.Contains(status.Porcelain, "?? internal/runner/testdata/event.json") {
		t.Fatalf("expected nested untracked file in status, got %q", status.Porcelain)
	}
	if strings.Contains(status.Porcelain, "?? internal/\n") {
		t.Fatalf("expected untracked directory to be expanded, got %q", status.Porcelain)
	}
}

func TestStatusReturnsIgnoredPaths(t *testing.T) {
	root := initGitRepo(t)
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("ignored.txt\n"), 0o644); err != nil {
		t.Fatalf("write gitignore: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "ignored.txt"), []byte("ignored"), 0o644); err != nil {
		t.Fatalf("write ignored file: %v", err)
	}
	client := NewClient()

	status, err := client.Status(context.Background(), root)
	if err != nil {
		t.Fatalf("Status returned error: %v", err)
	}
	if !strings.Contains(status.Porcelain, "!! ignored.txt") {
		t.Fatalf("expected ignored file in status, got %q", status.Porcelain)
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
	commit(t, root, "initial")
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

func TestDiffReturnsStagedAndUnstagedChanges(t *testing.T) {
	root := initGitRepo(t)
	if err := os.WriteFile(filepath.Join(root, "tracked.txt"), []byte("before\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	run(t, root, "git", "add", "tracked.txt")
	commit(t, root, "initial")
	if err := os.WriteFile(filepath.Join(root, "tracked.txt"), []byte("after\n"), 0o644); err != nil {
		t.Fatalf("modify file: %v", err)
	}
	run(t, root, "git", "add", "tracked.txt")
	client := NewClient()

	diff, err := client.Diff(context.Background(), root, "")
	if err != nil {
		t.Fatalf("Diff returned error: %v", err)
	}
	if !strings.Contains(diff.Diff, "-before") || !strings.Contains(diff.Diff, "+after") {
		t.Fatalf("expected staged diff against HEAD, got %q", diff.Diff)
	}
}

func TestDiffReturnsUntrackedFileDiff(t *testing.T) {
	root := initGitRepo(t)
	if err := os.WriteFile(filepath.Join(root, "new.txt"), []byte("new\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	client := NewClient()

	diff, err := client.Diff(context.Background(), root, "new.txt")
	if err != nil {
		t.Fatalf("Diff returned error: %v", err)
	}
	if !strings.Contains(diff.Diff, "new.txt") || !strings.Contains(diff.Diff, "+new") {
		t.Fatalf("expected untracked file diff, got %q", diff.Diff)
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

func TestStageRejectsEmptyPathList(t *testing.T) {
	root := initGitRepo(t)
	client := NewClient()

	_, err := client.Stage(context.Background(), root, nil)
	if !errors.Is(err, ErrEmptyPathList) {
		t.Fatalf("expected ErrEmptyPathList, got %v", err)
	}
}

func TestStageRejectsTraversalPath(t *testing.T) {
	root := initGitRepo(t)
	client := NewClient()

	_, err := client.Stage(context.Background(), root, []string{"../outside.txt"})
	if !errors.Is(err, ErrInvalidPath) {
		t.Fatalf("expected ErrInvalidPath, got %v", err)
	}
}

func TestStageStagesOnlyExplicitPaths(t *testing.T) {
	root := initGitRepo(t)
	if err := os.WriteFile(filepath.Join(root, "first.txt"), []byte("first\n"), 0o644); err != nil {
		t.Fatalf("write first file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "second.txt"), []byte("second\n"), 0o644); err != nil {
		t.Fatalf("write second file: %v", err)
	}
	client := NewClient()

	status, err := client.Stage(context.Background(), root, []string{"first.txt"})
	if err != nil {
		t.Fatalf("Stage returned error: %v", err)
	}
	if !strings.Contains(status.Porcelain, "A  first.txt") {
		t.Fatalf("expected first file staged, got %q", status.Porcelain)
	}
	if !strings.Contains(status.Porcelain, "?? second.txt") {
		t.Fatalf("expected second file to stay untracked, got %q", status.Porcelain)
	}
}

func TestUnstageUnstagesOnlyExplicitPaths(t *testing.T) {
	root := initGitRepo(t)
	if err := os.WriteFile(filepath.Join(root, "first.txt"), []byte("first\n"), 0o644); err != nil {
		t.Fatalf("write first file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "second.txt"), []byte("second\n"), 0o644); err != nil {
		t.Fatalf("write second file: %v", err)
	}
	run(t, root, "git", "add", "first.txt", "second.txt")
	client := NewClient()

	status, err := client.Unstage(context.Background(), root, []string{"first.txt"})
	if err != nil {
		t.Fatalf("Unstage returned error: %v", err)
	}
	if !strings.Contains(status.Porcelain, "?? first.txt") {
		t.Fatalf("expected first file unstaged, got %q", status.Porcelain)
	}
	if !strings.Contains(status.Porcelain, "A  second.txt") {
		t.Fatalf("expected second file to stay staged, got %q", status.Porcelain)
	}
}

func TestDiscardDiscardsTrackedAndUntrackedPaths(t *testing.T) {
	root := initGitRepo(t)
	if err := os.WriteFile(filepath.Join(root, "tracked.txt"), []byte("before\n"), 0o644); err != nil {
		t.Fatalf("write tracked file: %v", err)
	}
	run(t, root, "git", "add", "tracked.txt")
	commit(t, root, "initial")
	if err := os.WriteFile(filepath.Join(root, "tracked.txt"), []byte("after\n"), 0o644); err != nil {
		t.Fatalf("modify tracked file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "new.txt"), []byte("new\n"), 0o644); err != nil {
		t.Fatalf("write untracked file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "keep.txt"), []byte("keep\n"), 0o644); err != nil {
		t.Fatalf("write kept file: %v", err)
	}
	client := NewClient()

	status, err := client.Discard(context.Background(), root, []string{"tracked.txt", "new.txt"})
	if err != nil {
		t.Fatalf("Discard returned error: %v", err)
	}
	content, err := os.ReadFile(filepath.Join(root, "tracked.txt"))
	if err != nil {
		t.Fatalf("read tracked file: %v", err)
	}
	if string(content) != "before\n" {
		t.Fatalf("expected tracked file restored, got %q", content)
	}
	if _, err := os.Stat(filepath.Join(root, "new.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected untracked file removed, got %v", err)
	}
	if !strings.Contains(status.Porcelain, "?? keep.txt") {
		t.Fatalf("expected unrelated file to remain untracked, got %q", status.Porcelain)
	}
}

func TestCommitRejectsEmptyMessageAndPaths(t *testing.T) {
	root := initGitRepo(t)
	client := NewClient()

	_, err := client.Commit(context.Background(), root, "", []string{"file.txt"})
	if !errors.Is(err, ErrEmptyCommitMessage) {
		t.Fatalf("expected ErrEmptyCommitMessage, got %v", err)
	}
	_, err = client.Commit(context.Background(), root, "message", nil)
	if !errors.Is(err, ErrEmptyPathList) {
		t.Fatalf("expected ErrEmptyPathList, got %v", err)
	}
}

func TestCommitCommitsOnlyExplicitPaths(t *testing.T) {
	root := initGitRepo(t)
	configureCommitter(t, root)
	if err := os.WriteFile(filepath.Join(root, "first.txt"), []byte("first\n"), 0o644); err != nil {
		t.Fatalf("write first file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "second.txt"), []byte("second\n"), 0o644); err != nil {
		t.Fatalf("write second file: %v", err)
	}
	client := NewClient()

	commitResult, err := client.Commit(context.Background(), root, "add first", []string{"first.txt"})
	if err != nil {
		t.Fatalf("Commit returned error: %v", err)
	}
	if commitResult.Hash == "" {
		t.Fatal("expected commit hash")
	}
	status, err := client.Status(context.Background(), root)
	if err != nil {
		t.Fatalf("Status returned error: %v", err)
	}
	if strings.Contains(status.Porcelain, "first.txt") {
		t.Fatalf("expected first file committed, got %q", status.Porcelain)
	}
	if !strings.Contains(status.Porcelain, "?? second.txt") {
		t.Fatalf("expected second file to stay untracked, got %q", status.Porcelain)
	}
}

func TestCommitDoesNotCommitUnrelatedStagedPaths(t *testing.T) {
	root := initGitRepo(t)
	configureCommitter(t, root)
	if err := os.WriteFile(filepath.Join(root, "first.txt"), []byte("first\n"), 0o644); err != nil {
		t.Fatalf("write first file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "second.txt"), []byte("second\n"), 0o644); err != nil {
		t.Fatalf("write second file: %v", err)
	}
	run(t, root, "git", "add", "second.txt")
	client := NewClient()

	if _, err := client.Commit(context.Background(), root, "add first", []string{"first.txt"}); err != nil {
		t.Fatalf("Commit returned error: %v", err)
	}
	status, err := client.Status(context.Background(), root)
	if err != nil {
		t.Fatalf("Status returned error: %v", err)
	}
	if strings.Contains(status.Porcelain, "first.txt") {
		t.Fatalf("expected first file committed, got %q", status.Porcelain)
	}
	if !strings.Contains(status.Porcelain, "A  second.txt") {
		t.Fatalf("expected second file to stay staged, got %q", status.Porcelain)
	}
}

func TestMethodsRejectNonRepositoryAndNestedRoot(t *testing.T) {
	client := NewClient()
	_, err := client.Status(context.Background(), t.TempDir())
	if !errors.Is(err, ErrNotRepository) {
		t.Fatalf("expected ErrNotRepository, got %v", err)
	}

	root := initGitRepo(t)
	nested := filepath.Join(root, "nested")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("mkdir nested: %v", err)
	}
	_, err = client.Status(context.Background(), nested)
	if !errors.Is(err, ErrNotRepository) {
		t.Fatalf("expected ErrNotRepository for nested root, got %v", err)
	}
}

func initGitRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	run(t, root, "git", "init")
	return root
}

func commit(t *testing.T, root, message string) {
	t.Helper()
	configureCommitter(t, root)
	run(t, root, "git", "commit", "-m", message)
}

func configureCommitter(t *testing.T, root string) {
	t.Helper()
	run(t, root, "git", "config", "user.email", "test@example.com")
	run(t, root, "git", "config", "user.name", "Test")
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
