package workspace

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateRejectsRelativeRoot(t *testing.T) {
	manager := newTestManager(t, t.TempDir())

	_, err := manager.Create("relative")
	if !errors.Is(err, ErrInvalidRoot) {
		t.Fatalf("expected ErrInvalidRoot, got %v", err)
	}
}

func TestCreateRejectsRootOutsideAllowedRoots(t *testing.T) {
	allowed := t.TempDir()
	outside := t.TempDir()
	mustMkdir(t, filepath.Join(outside, ".git"))
	manager := newTestManager(t, allowed)

	_, err := manager.Create(outside)
	if !errors.Is(err, ErrOutsideRoots) {
		t.Fatalf("expected ErrOutsideRoots, got %v", err)
	}
}

func TestCreateRejectsNonGitDirectory(t *testing.T) {
	root := t.TempDir()
	manager := newTestManager(t, root)

	_, err := manager.Create(root)
	if !errors.Is(err, ErrNotGitRepo) {
		t.Fatalf("expected ErrNotGitRepo, got %v", err)
	}
}

func TestCreateAssignsWorkspaceID(t *testing.T) {
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, ".git"))
	manager := newTestManager(t, root)

	ws, err := manager.Create(root)
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

func newTestManager(t *testing.T, allowedRoot string) *Manager {
	t.Helper()
	manager, err := NewManager([]string{allowedRoot})
	if err != nil {
		t.Fatalf("NewManager returned error: %v", err)
	}
	return manager
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}
