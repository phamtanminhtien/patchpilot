package api

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/phamtanminhtien/patchpilot/internal/database"
	"github.com/phamtanminhtien/patchpilot/internal/runner"
)

func TestServerShutdownStopsActiveCommandsAndPublishesExit(t *testing.T) {
	fixture := newServerFixture(t, t.TempDir(), fakeAgentProvider{})
	eventsCh, unsubscribe := fixture.hub.Subscribe()
	defer unsubscribe()

	queued, err := fixture.store.CreateCommand(context.Background(), database.CommandRecord{
		WorkspaceID: "ws_1",
		Command:     "echo later",
		Cwd:         t.TempDir(),
		Status:      "queued",
	})
	if err != nil {
		t.Fatalf("CreateCommand queued returned error: %v", err)
	}
	running, err := fixture.store.CreateCommand(context.Background(), database.CommandRecord{
		WorkspaceID: "ws_1",
		Command:     "go test ./...",
		Cwd:         filepath.Join("..", "runner", "testdata", "slow"),
		Status:      "queued",
	})
	if err != nil {
		t.Fatalf("CreateCommand running returned error: %v", err)
	}
	if err := fixture.runner.Start(runner.RunSpec{
		ID:          running.ID,
		WorkspaceID: running.WorkspaceID,
		Command:     running.Command,
		Cwd:         running.Cwd,
	}, fixture.server.commandHooks(running.WorkspaceID, running.ID)); err != nil {
		t.Fatalf("runner.Start returned error: %v", err)
	}
	waitForCommandStatus(t, fixture.store, running.WorkspaceID, running.ID, "running")

	if err := fixture.server.Shutdown(context.Background(), "backend shutdown"); err != nil {
		t.Fatalf("Shutdown returned error: %v", err)
	}

	queued = waitForCommandStatus(t, fixture.store, queued.WorkspaceID, queued.ID, "stopped")
	running = waitForCommandStatus(t, fixture.store, running.WorkspaceID, running.ID, "stopped")
	if queued.FinishedAt == nil || running.FinishedAt == nil {
		t.Fatalf("expected finished_at on stopped commands, queued=%+v running=%+v", queued, running)
	}
	if got := receiveProcessExitedEvents(t, eventsCh, 2); len(got) != 2 {
		t.Fatalf("expected 2 process.exited events, got %+v", got)
	}
}
