package database

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestAgentRunRepositoryPersistsEventsAndTools(t *testing.T) {
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
	run, err := store.CreateAgentRun(ctx, AgentRunRecord{
		WorkspaceID:      "ws_1",
		ConversationID:   "conv_1",
		TriggerMessageID: "msg_1",
		Model:            "gpt-5.5",
		ReasoningEffort:  "medium",
		Status:           "queued",
		CreatedAt:        createdAt,
		UpdatedAt:        createdAt,
	})
	if err != nil {
		t.Fatalf("CreateAgentRun returned error: %v", err)
	}
	if run.ID == "" || run.Model != "gpt-5.5" || run.ReasoningEffort != "medium" {
		t.Fatalf("unexpected run: %+v", run)
	}

	startedAt := createdAt.Add(time.Second)
	run, err = store.UpdateAgentRun(ctx, "ws_1", "conv_1", run.ID, map[string]any{
		"status":     "running",
		"started_at": startedAt,
		"summary":    "inspect",
	})
	if err != nil {
		t.Fatalf("UpdateAgentRun returned error: %v", err)
	}
	if run.Status != "running" || run.Summary != "inspect" || run.StartedAt == nil {
		t.Fatalf("unexpected updated run: %+v", run)
	}

	event, err := store.CreateAgentRunEvent(ctx, AgentRunEventRecord{
		WorkspaceID: "ws_1",
		RunID:       run.ID,
		Type:        "agent.delta",
		PayloadJSON: `{"text":"hello"}`,
		CreatedAt:   startedAt.Add(time.Millisecond),
	})
	if err != nil {
		t.Fatalf("CreateAgentRunEvent returned error: %v", err)
	}
	events, err := store.ListAgentRunEvents(ctx, "ws_1", run.ID)
	if err != nil {
		t.Fatalf("ListAgentRunEvents returned error: %v", err)
	}
	if len(events) != 1 || events[0].ID != event.ID {
		t.Fatalf("unexpected events: %+v", events)
	}

	call, err := store.CreateAgentToolCall(ctx, AgentToolCallRecord{
		WorkspaceID:    "ws_1",
		RunID:          run.ID,
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
	call, err = store.FinishAgentToolCall(ctx, "ws_1", run.ID, call.ID, "finished", `{"ok":true}`, finishedAt)
	if err != nil {
		t.Fatalf("FinishAgentToolCall returned error: %v", err)
	}
	if call.Status != "finished" || call.OutputJSON != `{"ok":true}` || call.FinishedAt == nil {
		t.Fatalf("unexpected call: %+v", call)
	}

	approved := "approved"
	call, err = store.UpdateAgentToolCall(ctx, "ws_1", run.ID, call.ID, map[string]any{
		"status":   "approved",
		"decision": approved,
	})
	if err != nil {
		t.Fatalf("UpdateAgentToolCall returned error: %v", err)
	}
	if call.Status != "approved" || call.Decision == nil || *call.Decision != approved {
		t.Fatalf("unexpected updated call: %+v", call)
	}
	loaded, err := store.GetAgentToolCall(ctx, "ws_1", run.ID, call.ID)
	if err != nil {
		t.Fatalf("GetAgentToolCall returned error: %v", err)
	}
	if loaded.ID != call.ID {
		t.Fatalf("unexpected loaded call: %+v", loaded)
	}
}
