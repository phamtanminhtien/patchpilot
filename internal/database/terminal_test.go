package database

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestTerminalSessionLifecycle(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "patchpilot.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()

	pid := 123
	created, err := store.CreateTerminalSession(context.Background(), TerminalSessionRecord{
		WorkspaceID: "ws_1",
		Title:       "Dev server",
		Cwd:         "/tmp/repo",
		Status:      "open",
		PID:         &pid,
		Rows:        30,
		Cols:        100,
	})
	if err != nil {
		t.Fatalf("CreateTerminalSession returned error: %v", err)
	}
	if created.ID == "" || created.CreatedAt.IsZero() || created.UpdatedAt.IsZero() {
		t.Fatalf("expected generated id and timestamps, got %+v", created)
	}

	listed, err := store.ListTerminalSessions(context.Background(), "ws_1")
	if err != nil {
		t.Fatalf("ListTerminalSessions returned error: %v", err)
	}
	if len(listed) != 1 || listed[0].ID != created.ID {
		t.Fatalf("unexpected listed sessions: %+v", listed)
	}

	updated, err := store.UpdateTerminalSession(context.Background(), "ws_1", created.ID, map[string]any{"title": "Renamed", "rows": 40, "cols": 120})
	if err != nil {
		t.Fatalf("UpdateTerminalSession returned error: %v", err)
	}
	if updated.Title != "Renamed" || updated.Rows != 40 || updated.Cols != 120 {
		t.Fatalf("unexpected updated session: %+v", updated)
	}

	exitCode := 0
	closedAt := time.Now().UTC()
	closed, err := store.CloseTerminalSession(context.Background(), "ws_1", created.ID, "closed", &exitCode, closedAt)
	if err != nil {
		t.Fatalf("CloseTerminalSession returned error: %v", err)
	}
	if closed.Status != "closed" || closed.ExitCode == nil || *closed.ExitCode != 0 || closed.ClosedAt == nil {
		t.Fatalf("unexpected closed session: %+v", closed)
	}
}
