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
	if !store.db.Migrator().HasTable(&CommandRecord{}) {
		t.Fatal("expected commands table to be migrated")
	}
	if !store.db.Migrator().HasTable(&CommandOutputRecord{}) {
		t.Fatal("expected command_output table to be migrated")
	}
	if !store.db.Migrator().HasTable(&AgentTaskRecord{}) {
		t.Fatal("expected agent_tasks table to be migrated")
	}
	if !store.db.Migrator().HasTable(&AgentTaskEventRecord{}) {
		t.Fatal("expected agent_task_events table to be migrated")
	}
	if !store.db.Migrator().HasTable(&AgentToolCallRecord{}) {
		t.Fatal("expected agent_tool_calls table to be migrated")
	}
	if !store.db.Migrator().HasTable(&PatchRecord{}) {
		t.Fatal("expected patches table to be migrated")
	}
}

func TestAgentTaskRepositoryPersistsEventsToolsAndPatches(t *testing.T) {
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
	task, err := store.CreateAgentTask(ctx, AgentTaskRecord{
		WorkspaceID:     "ws_1",
		Prompt:          "fix bug",
		Model:           "gpt-5.5",
		ReasoningEffort: "medium",
		Status:          "queued",
		CreatedAt:       createdAt,
		UpdatedAt:       createdAt,
	})
	if err != nil {
		t.Fatalf("CreateAgentTask returned error: %v", err)
	}
	if task.ID == "" || task.Model != "gpt-5.5" || task.ReasoningEffort != "medium" {
		t.Fatalf("unexpected task: %+v", task)
	}

	startedAt := createdAt.Add(time.Second)
	task, err = store.UpdateAgentTask(ctx, "ws_1", task.ID, map[string]any{
		"status":     "running",
		"started_at": startedAt,
		"plan":       "inspect",
	})
	if err != nil {
		t.Fatalf("UpdateAgentTask returned error: %v", err)
	}
	if task.Status != "running" || task.Plan != "inspect" || task.StartedAt == nil {
		t.Fatalf("unexpected updated task: %+v", task)
	}

	event, err := store.CreateAgentTaskEvent(ctx, AgentTaskEventRecord{
		WorkspaceID: "ws_1",
		TaskID:      task.ID,
		Type:        "agent.delta",
		PayloadJSON: `{"text":"hello"}`,
		CreatedAt:   startedAt.Add(time.Millisecond),
	})
	if err != nil {
		t.Fatalf("CreateAgentTaskEvent returned error: %v", err)
	}
	events, err := store.ListAgentTaskEvents(ctx, "ws_1", task.ID)
	if err != nil {
		t.Fatalf("ListAgentTaskEvents returned error: %v", err)
	}
	if len(events) != 1 || events[0].ID != event.ID {
		t.Fatalf("unexpected events: %+v", events)
	}

	call, err := store.CreateAgentToolCall(ctx, AgentToolCallRecord{
		WorkspaceID: "ws_1",
		TaskID:      task.ID,
		Name:        "git_status",
		InputJSON:   "{}",
		OutputJSON:  "{}",
		Status:      "running",
		StartedAt:   &startedAt,
	})
	if err != nil {
		t.Fatalf("CreateAgentToolCall returned error: %v", err)
	}
	finishedAt := startedAt.Add(time.Second)
	call, err = store.FinishAgentToolCall(ctx, "ws_1", task.ID, call.ID, "finished", `{"ok":true}`, finishedAt)
	if err != nil {
		t.Fatalf("FinishAgentToolCall returned error: %v", err)
	}
	if call.Status != "finished" || call.OutputJSON != `{"ok":true}` || call.FinishedAt == nil {
		t.Fatalf("unexpected call: %+v", call)
	}

	patch, err := store.CreatePatch(ctx, PatchRecord{
		WorkspaceID: "ws_1",
		TaskID:      task.ID,
		Diff:        "diff --git a/a b/a\n",
		Summary:     "patch",
		Status:      "created",
	})
	if err != nil {
		t.Fatalf("CreatePatch returned error: %v", err)
	}
	patches, err := store.ListPatches(ctx, "ws_1", task.ID)
	if err != nil {
		t.Fatalf("ListPatches returned error: %v", err)
	}
	if len(patches) != 1 || patches[0].ID != patch.ID {
		t.Fatalf("unexpected patches: %+v", patches)
	}
}

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
