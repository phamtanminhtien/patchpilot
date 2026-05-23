package database

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestCommandRepositoryPersistsLifecycleAndOutput(t *testing.T) {
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

	createdAt := time.Date(2026, 5, 20, 10, 0, 0, 0, time.UTC)
	command, err := store.CreateCommand(ctx, CommandRecord{
		WorkspaceID: "ws_1",
		Command:     "go test ./...",
		Cwd:         "/repo",
		Status:      "queued",
		CreatedAt:   createdAt,
	})
	if err != nil {
		t.Fatalf("CreateCommand returned error: %v", err)
	}
	if command.ID == "" || command.Status != "queued" {
		t.Fatalf("unexpected command: %+v", command)
	}

	startedAt := createdAt.Add(time.Second)
	command, err = store.MarkCommandRunning(ctx, "ws_1", command.ID, startedAt)
	if err != nil {
		t.Fatalf("MarkCommandRunning returned error: %v", err)
	}
	if command.Status != "running" || command.StartedAt == nil || !command.StartedAt.Equal(startedAt) {
		t.Fatalf("unexpected running command: %+v", command)
	}

	if _, err := store.AppendCommandOutput(ctx, CommandOutputRecord{
		CommandID: command.ID,
		Stream:    "stdout",
		Chunk:     "hello\n",
		CreatedAt: startedAt.Add(time.Millisecond),
	}, 1024); err != nil {
		t.Fatalf("AppendCommandOutput stdout returned error: %v", err)
	}
	if _, err := store.AppendCommandOutput(ctx, CommandOutputRecord{
		CommandID: command.ID,
		Stream:    "stderr",
		Chunk:     "warning\n",
		CreatedAt: startedAt.Add(2 * time.Millisecond),
	}, 1024); err != nil {
		t.Fatalf("AppendCommandOutput stderr returned error: %v", err)
	}
	output, err := store.ListCommandOutput(ctx, command.ID)
	if err != nil {
		t.Fatalf("ListCommandOutput returned error: %v", err)
	}
	if len(output) != 2 || output[0].Stream != "stdout" || output[1].Stream != "stderr" {
		t.Fatalf("unexpected output: %+v", output)
	}

	exitCode := 0
	finishedAt := startedAt.Add(time.Second)
	command, err = store.FinishCommand(ctx, "ws_1", command.ID, "exited", &exitCode, finishedAt)
	if err != nil {
		t.Fatalf("FinishCommand returned error: %v", err)
	}
	if command.Status != "exited" || command.ExitCode == nil || *command.ExitCode != 0 || command.FinishedAt == nil {
		t.Fatalf("unexpected finished command: %+v", command)
	}
}

func TestCommandOutputRepositoryKeepsLatestBytes(t *testing.T) {
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

	command, err := store.CreateCommand(ctx, CommandRecord{
		WorkspaceID: "ws_1",
		Command:     "test",
		Cwd:         "/repo",
		Status:      "running",
	})
	if err != nil {
		t.Fatalf("CreateCommand returned error: %v", err)
	}
	for i, chunk := range []string{"aaaa", "bbbb", "cccc"} {
		if _, err := store.AppendCommandOutput(ctx, CommandOutputRecord{
			CommandID: command.ID,
			Stream:    "stdout",
			Chunk:     chunk,
			CreatedAt: time.Date(2026, 5, 20, 10, 0, i, 0, time.UTC),
		}, 8); err != nil {
			t.Fatalf("AppendCommandOutput %d returned error: %v", i, err)
		}
	}
	output, err := store.ListCommandOutput(ctx, command.ID)
	if err != nil {
		t.Fatalf("ListCommandOutput returned error: %v", err)
	}
	if len(output) != 2 || output[0].Chunk != "bbbb" || output[1].Chunk != "cccc" {
		t.Fatalf("expected latest chunks within cap, got %+v", output)
	}
}

func TestCommandRepositoryListsActiveCommands(t *testing.T) {
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

	records := []CommandRecord{
		{WorkspaceID: "ws_1", Command: "go test ./...", Cwd: "/repo", Status: "queued"},
		{WorkspaceID: "ws_2", Command: "sleep 10", Cwd: "/repo", Status: "running"},
		{WorkspaceID: "ws_3", Command: "echo done", Cwd: "/repo", Status: "exited"},
	}
	for _, record := range records {
		if _, err := store.CreateCommand(ctx, record); err != nil {
			t.Fatalf("CreateCommand returned error: %v", err)
		}
	}

	active, err := store.ListActiveCommands(ctx)
	if err != nil {
		t.Fatalf("ListActiveCommands returned error: %v", err)
	}
	if len(active) != 2 {
		t.Fatalf("expected 2 active commands, got %+v", active)
	}
	if active[0].Status != "queued" || active[1].Status != "running" {
		t.Fatalf("unexpected active commands: %+v", active)
	}
}
