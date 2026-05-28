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
	state := WorkspaceIndexStateRecord{WorkspaceID: "ws_1", Status: "ready", LastIndexedAt: &indexedAt, LastFullScanAt: &indexedAt, FileCount: 2, UpdatedAt: indexedAt}
	if err := store.ReplaceFileIndex(ctx, "ws_1", []FileIndexRecord{
		{WorkspaceID: "ws_1", Path: "src/app.tsx", Name: "app.tsx", Dir: "src", Extension: "tsx", Kind: "file", IndexStatus: "indexed", PathLower: "src/app.tsx", NameLower: "app.tsx", Depth: 2, Size: 10, ModifiedAt: indexedAt, IndexedAt: indexedAt},
		{WorkspaceID: "ws_1", Path: "README.md", Name: "README.md", Dir: "", Extension: "md", Kind: "file", IndexStatus: "indexed", PathLower: "readme.md", NameLower: "readme.md", Depth: 1, Size: 5, ModifiedAt: indexedAt, IndexedAt: indexedAt},
	}, state); err != nil {
		t.Fatalf("ReplaceFileIndex returned error: %v", err)
	}
	entries, err := store.ListFileIndex(ctx, "ws_1", FileIndexListOptions{})
	if err != nil {
		t.Fatalf("ListFileIndex returned error: %v", err)
	}
	if len(entries) != 2 || entries[0].Path != "README.md" || entries[1].Path != "src/app.tsx" {
		t.Fatalf("expected sorted file index, got %+v", entries)
	}

	state.FileCount = 1
	if err := store.ReplaceFileIndex(ctx, "ws_1", []FileIndexRecord{
		{WorkspaceID: "ws_1", Path: "main.go", Name: "main.go", Extension: "go", Kind: "file", IndexStatus: "indexed", PathLower: "main.go", NameLower: "main.go", Depth: 1, Size: 20, ModifiedAt: indexedAt, IndexedAt: indexedAt},
	}, state); err != nil {
		t.Fatalf("ReplaceFileIndex second returned error: %v", err)
	}
	entries, err = store.ListFileIndex(ctx, "ws_1", FileIndexListOptions{})
	if err != nil {
		t.Fatalf("ListFileIndex second returned error: %v", err)
	}
	if len(entries) != 1 || entries[0].Path != "main.go" {
		t.Fatalf("expected replaced file index, got %+v", entries)
	}
}

func TestFileIndexRepositoryFuzzySearchesAndRanksEntries(t *testing.T) {
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
	state := WorkspaceIndexStateRecord{WorkspaceID: "ws_1", Status: "ready", LastIndexedAt: &indexedAt, LastFullScanAt: &indexedAt, FileCount: 3, UpdatedAt: indexedAt}
	if err := store.ReplaceFileIndex(ctx, "ws_1", []FileIndexRecord{
		{WorkspaceID: "ws_1", Path: "internal/agent/agent.go", Name: "agent.go", Dir: "internal/agent", Extension: "go", Kind: "file", IndexStatus: "indexed", PathLower: "internal/agent/agent.go", NameLower: "agent.go", Depth: 3, Size: 10, ModifiedAt: indexedAt, IndexedAt: indexedAt},
		{WorkspaceID: "ws_1", Path: "web/src/app.tsx", Name: "app.tsx", Dir: "web/src", Extension: "tsx", Kind: "file", IndexStatus: "indexed", PathLower: "web/src/app.tsx", NameLower: "app.tsx", Depth: 3, Size: 10, ModifiedAt: indexedAt, IndexedAt: indexedAt},
		{WorkspaceID: "ws_1", Path: "docs/architecture.md", Name: "architecture.md", Dir: "docs", Extension: "md", Kind: "file", IndexStatus: "indexed", PathLower: "docs/architecture.md", NameLower: "architecture.md", Depth: 2, Size: 10, ModifiedAt: indexedAt, IndexedAt: indexedAt},
	}, state); err != nil {
		t.Fatalf("ReplaceFileIndex returned error: %v", err)
	}

	entries, err := store.ListFileIndex(ctx, "ws_1", FileIndexListOptions{Query: "agt"})
	if err != nil {
		t.Fatalf("ListFileIndex returned error: %v", err)
	}
	if len(entries) != 1 || entries[0].Path != "internal/agent/agent.go" {
		t.Fatalf("expected fuzzy agent match, got %+v", entries)
	}

	entries, err = store.ListFileIndex(ctx, "ws_1", FileIndexListOptions{Query: "app"})
	if err != nil {
		t.Fatalf("ListFileIndex app returned error: %v", err)
	}
	if len(entries) != 1 || entries[0].Path != "web/src/app.tsx" {
		t.Fatalf("expected exact-ish app match first, got %+v", entries)
	}
}
