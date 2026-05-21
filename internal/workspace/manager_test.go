package workspace

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/phamtanminhtien/patchpilot/internal/database"
	"github.com/phamtanminhtien/patchpilot/internal/gitrepo"
)

func TestCreateRejectsRelativeRoot(t *testing.T) {
	manager := newTestManager(t, t.TempDir(), testDBPath(t))

	_, err := manager.Create(context.Background(), "relative")
	if !errors.Is(err, ErrInvalidRoot) {
		t.Fatalf("expected ErrInvalidRoot, got %v", err)
	}
}

func TestCreateRejectsRootOutsideAllowedRoots(t *testing.T) {
	allowed := t.TempDir()
	outside := initGitRepo(t, t.TempDir())
	manager := newTestManager(t, allowed, testDBPath(t))

	_, err := manager.Create(context.Background(), outside)
	if !errors.Is(err, ErrOutsideRoots) {
		t.Fatalf("expected ErrOutsideRoots, got %v", err)
	}
}

func TestCreateRejectsSymlinkThatResolvesOutsideAllowedRoots(t *testing.T) {
	allowed := t.TempDir()
	outside := initGitRepo(t, t.TempDir())
	link := filepath.Join(allowed, "linked-repo")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	manager := newTestManager(t, allowed, testDBPath(t))

	_, err := manager.Create(context.Background(), link)
	if !errors.Is(err, ErrOutsideRoots) {
		t.Fatalf("expected ErrOutsideRoots, got %v", err)
	}
}

func TestCreateRejectsNonGitDirectory(t *testing.T) {
	root := t.TempDir()
	manager := newTestManager(t, root, testDBPath(t))

	_, err := manager.Create(context.Background(), root)
	if !errors.Is(err, ErrNotGitRepo) {
		t.Fatalf("expected ErrNotGitRepo, got %v", err)
	}
}

func TestCreateAssignsWorkspaceIDAndNormalizesRoot(t *testing.T) {
	root := initGitRepo(t, t.TempDir())
	manager := newTestManager(t, root, testDBPath(t))

	ws, err := manager.Create(context.Background(), root)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if !strings.HasPrefix(ws.ID, "ws_") {
		t.Fatalf("expected ws_ ID, got %q", ws.ID)
	}
	if ws.Status != "ready" {
		t.Fatalf("expected ready status, got %q", ws.Status)
	}
	expectedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatalf("EvalSymlinks returned error: %v", err)
	}
	if ws.RootPath != expectedRoot {
		t.Fatalf("expected root path %q, got %q", expectedRoot, ws.RootPath)
	}
}

func TestCreateRestoresExistingWorkspaceByRoot(t *testing.T) {
	ctx := context.Background()
	root := initGitRepo(t, t.TempDir())
	dbPath := filepath.Join(t.TempDir(), "patchpilot.db")
	manager := newTestManager(t, root, dbPath)

	first, err := manager.Create(ctx, root)
	if err != nil {
		t.Fatalf("Create first returned error: %v", err)
	}
	time.Sleep(time.Millisecond)
	second, err := manager.Create(ctx, filepath.Join(root, "."))
	if err != nil {
		t.Fatalf("Create second returned error: %v", err)
	}
	if second.ID != first.ID {
		t.Fatalf("expected restored workspace ID %q, got %q", first.ID, second.ID)
	}
	if !second.UpdatedAt.After(first.UpdatedAt) {
		t.Fatalf("expected restored workspace to update recency: first=%v second=%v", first.UpdatedAt, second.UpdatedAt)
	}
}

func TestWorkspacesPersistAcrossManagers(t *testing.T) {
	ctx := context.Background()
	root := initGitRepo(t, t.TempDir())
	dbPath := filepath.Join(t.TempDir(), "patchpilot.db")
	firstManager := newTestManager(t, root, dbPath)
	created, err := firstManager.Create(ctx, root)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	secondManager := newTestManager(t, root, dbPath)
	restored, err := secondManager.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if restored.ID != created.ID || restored.RootPath != created.RootPath {
		t.Fatalf("expected restored workspace %+v, got %+v", created, restored)
	}
}

func TestPersistedWorkspacesMustRemainInsideAllowedRoots(t *testing.T) {
	ctx := context.Background()
	root := initGitRepo(t, t.TempDir())
	dbPath := testDBPath(t)
	firstManager := newTestManager(t, root, dbPath)
	created, err := firstManager.Create(ctx, root)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	secondManager := newTestManager(t, t.TempDir(), dbPath)
	_, err = secondManager.Get(ctx, created.ID)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound for workspace outside current allowed roots, got %v", err)
	}
	workspaces, err := secondManager.List(ctx)
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(workspaces) != 0 {
		t.Fatalf("expected outside-root workspace to be filtered, got %+v", workspaces)
	}
}

func TestListReturnsNewestFirstFromStore(t *testing.T) {
	ctx := context.Background()
	allowed := t.TempDir()
	firstRoot := initGitRepo(t, filepath.Join(allowed, "first"))
	secondRoot := initGitRepo(t, filepath.Join(allowed, "second"))
	manager := newTestManager(t, allowed, testDBPath(t))

	first, err := manager.Create(ctx, firstRoot)
	if err != nil {
		t.Fatalf("Create first returned error: %v", err)
	}
	time.Sleep(time.Millisecond)
	second, err := manager.Create(ctx, secondRoot)
	if err != nil {
		t.Fatalf("Create second returned error: %v", err)
	}

	workspaces, err := manager.List(ctx)
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(workspaces) != 2 {
		t.Fatalf("expected 2 workspaces, got %+v", workspaces)
	}
	if workspaces[0].ID != second.ID || workspaces[1].ID != first.ID {
		t.Fatalf("expected newest-first workspaces, got %+v", workspaces)
	}
}

func newTestManager(t *testing.T, allowedRoot string, dbPath string) *Manager {
	t.Helper()
	store, err := database.Open(dbPath)
	if err != nil {
		t.Fatalf("database.Open returned error: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	})
	manager, err := NewManager([]string{allowedRoot}, store, gitrepo.NewClient())
	if err != nil {
		t.Fatalf("NewManager returned error: %v", err)
	}
	return manager
}

func testDBPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "patchpilot.db")
}

func initGitRepo(t *testing.T, root string) string {
	t.Helper()
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", root, err)
	}
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
