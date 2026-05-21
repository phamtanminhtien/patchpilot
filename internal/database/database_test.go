package database

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestOpenCreatesSQLiteDatabaseAndEnablesForeignKeys(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "nested", "patchpilot.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()

	if err := store.Ping(context.Background()); err != nil {
		t.Fatalf("Ping returned error: %v", err)
	}

	var enabled int
	if err := store.db.Raw("PRAGMA foreign_keys").Scan(&enabled).Error; err != nil {
		t.Fatalf("read foreign_keys pragma: %v", err)
	}
	if enabled != 1 {
		t.Fatalf("expected foreign keys enabled, got %d", enabled)
	}

	if !store.db.Migrator().HasTable(&Metadata{}) {
		t.Fatal("expected app metadata table to be migrated")
	}
	if !store.db.Migrator().HasTable(&WorkspaceRecord{}) {
		t.Fatal("expected workspaces table to be migrated")
	}
	if !store.db.Migrator().HasTable(&FileIndexRecord{}) {
		t.Fatal("expected file_index table to be migrated")
	}
}

func TestWorkspaceRepositoryPersistsAndListsNewestFirst(t *testing.T) {
	ctx := context.Background()
	store, err := Open(filepath.Join(t.TempDir(), "patchpilot.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()

	firstTime := time.Date(2026, 5, 20, 10, 0, 0, 0, time.UTC)
	secondTime := firstTime.Add(time.Minute)
	first, err := store.CreateWorkspace(ctx, WorkspaceRecord{
		ID:        "ws_first",
		Name:      "first",
		RootPath:  "/tmp/first",
		Status:    "ready",
		CreatedAt: firstTime,
		UpdatedAt: firstTime,
	})
	if err != nil {
		t.Fatalf("CreateWorkspace first returned error: %v", err)
	}
	second, err := store.CreateWorkspace(ctx, WorkspaceRecord{
		ID:        "ws_second",
		Name:      "second",
		RootPath:  "/tmp/second",
		Status:    "ready",
		CreatedAt: secondTime,
		UpdatedAt: secondTime,
	})
	if err != nil {
		t.Fatalf("CreateWorkspace second returned error: %v", err)
	}

	found, err := store.FindWorkspaceByRoot(ctx, first.RootPath)
	if err != nil {
		t.Fatalf("FindWorkspaceByRoot returned error: %v", err)
	}
	if found.ID != first.ID {
		t.Fatalf("expected %q, got %q", first.ID, found.ID)
	}

	list, err := store.ListWorkspaces(ctx)
	if err != nil {
		t.Fatalf("ListWorkspaces returned error: %v", err)
	}
	if len(list) != 2 || list[0].ID != second.ID || list[1].ID != first.ID {
		t.Fatalf("expected newest-first workspaces, got %+v", list)
	}

	touched, err := store.TouchWorkspace(ctx, first.ID, secondTime.Add(time.Minute))
	if err != nil {
		t.Fatalf("TouchWorkspace returned error: %v", err)
	}
	if !touched.UpdatedAt.After(second.UpdatedAt) {
		t.Fatalf("expected touched workspace to have newer updated_at, got %+v", touched)
	}
	list, err = store.ListWorkspaces(ctx)
	if err != nil {
		t.Fatalf("ListWorkspaces after touch returned error: %v", err)
	}
	if list[0].ID != first.ID {
		t.Fatalf("expected touched workspace first, got %+v", list)
	}
}

func TestFileIndexRepositoryReplacesAndListsEntries(t *testing.T) {
	ctx := context.Background()
	store, err := Open(filepath.Join(t.TempDir(), "patchpilot.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()

	indexedAt := time.Date(2026, 5, 20, 10, 0, 0, 0, time.UTC)
	if err := store.ReplaceFileIndex(ctx, "ws_1", []FileIndexRecord{
		{WorkspaceID: "ws_1", Path: "src/app.tsx", Size: 10, ModifiedAt: indexedAt, IndexedAt: indexedAt},
		{WorkspaceID: "ws_1", Path: "README.md", Size: 5, ModifiedAt: indexedAt, IndexedAt: indexedAt},
	}); err != nil {
		t.Fatalf("ReplaceFileIndex returned error: %v", err)
	}
	entries, err := store.ListFileIndex(ctx, "ws_1")
	if err != nil {
		t.Fatalf("ListFileIndex returned error: %v", err)
	}
	if len(entries) != 2 || entries[0].Path != "README.md" || entries[1].Path != "src/app.tsx" {
		t.Fatalf("expected sorted file index, got %+v", entries)
	}

	if err := store.ReplaceFileIndex(ctx, "ws_1", []FileIndexRecord{
		{WorkspaceID: "ws_1", Path: "main.go", Size: 20, ModifiedAt: indexedAt, IndexedAt: indexedAt},
	}); err != nil {
		t.Fatalf("ReplaceFileIndex second returned error: %v", err)
	}
	entries, err = store.ListFileIndex(ctx, "ws_1")
	if err != nil {
		t.Fatalf("ListFileIndex second returned error: %v", err)
	}
	if len(entries) != 1 || entries[0].Path != "main.go" {
		t.Fatalf("expected replaced file index, got %+v", entries)
	}
}
