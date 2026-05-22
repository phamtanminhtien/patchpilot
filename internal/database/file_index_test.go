package database

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

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
