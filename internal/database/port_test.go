package database

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestDetectedPortReopensPreviouslyExposedPort(t *testing.T) {
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

	exposedPath := "/workspaces/ws_1/ports/5173/proxy/"
	created, _, err := store.UpsertDetectedPort(ctx, PortRecord{
		WorkspaceID: "ws_1",
		Port:        5173,
		Status:      "detected",
	})
	if err != nil {
		t.Fatalf("UpsertDetectedPort returned error: %v", err)
	}
	if _, err := store.ExposePort(ctx, "ws_1", created.Port, exposedPath); err != nil {
		t.Fatalf("ExposePort returned error: %v", err)
	}
	if _, err := store.MarkPortClosed(ctx, "ws_1", created.Port, time.Now().UTC()); err != nil {
		t.Fatalf("MarkPortClosed returned error: %v", err)
	}

	reopened, _, err := store.UpsertDetectedPort(ctx, PortRecord{
		WorkspaceID: "ws_1",
		Port:        5173,
		Status:      "detected",
	})
	if err != nil {
		t.Fatalf("UpsertDetectedPort reopen returned error: %v", err)
	}
	if reopened.Status != "exposed" || reopened.ExposedPath == nil || *reopened.ExposedPath != exposedPath {
		t.Fatalf("expected reopened exposed port, got %+v", reopened)
	}
}
