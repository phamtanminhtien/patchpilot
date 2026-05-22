package database

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestAgentTaskRepositoryPersistsEventsAndTools(t *testing.T) {
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
		"summary":    "inspect",
	})
	if err != nil {
		t.Fatalf("UpdateAgentTask returned error: %v", err)
	}
	if task.Status != "running" || task.Summary != "inspect" || task.StartedAt == nil {
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
		WorkspaceID:    "ws_1",
		TaskID:         task.ID,
		BatchID:        "batch_1",
		Sequence:       0,
		ProviderCallID: "call_1",
		Name:           "git_status",
		InputJSON:      "{}",
		OutputJSON:     "{}",
		Status:         "running",
		StartedAt:      &startedAt,
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

	approved := "approved"
	call, err = store.UpdateAgentToolCall(ctx, "ws_1", task.ID, call.ID, map[string]any{
		"status":   "approved",
		"decision": approved,
	})
	if err != nil {
		t.Fatalf("UpdateAgentToolCall returned error: %v", err)
	}
	if call.Status != "approved" || call.Decision == nil || *call.Decision != approved {
		t.Fatalf("unexpected updated call: %+v", call)
	}
	loaded, err := store.GetAgentToolCall(ctx, "ws_1", task.ID, call.ID)
	if err != nil {
		t.Fatalf("GetAgentToolCall returned error: %v", err)
	}
	if loaded.ID != call.ID {
		t.Fatalf("unexpected loaded call: %+v", loaded)
	}
}
