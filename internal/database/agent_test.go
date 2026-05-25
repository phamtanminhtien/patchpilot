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
	conversation, err := store.CreateConversation(ctx, ConversationRecord{
		ID:          "conv_1",
		WorkspaceID: "ws_1",
		Title:       "Tracked",
		CreatedAt:   createdAt,
		UpdatedAt:   createdAt,
	})
	if err != nil {
		t.Fatalf("CreateConversation returned error: %v", err)
	}
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
	conversation, err = store.GetConversation(ctx, "ws_1", conversation.ID)
	if err != nil {
		t.Fatalf("GetConversation returned error: %v", err)
	}
	if !conversation.HasRunningRun {
		t.Fatalf("expected conversation to be marked active, got %+v", conversation)
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
	conversation, err = store.GetConversation(ctx, "ws_1", conversation.ID)
	if err != nil {
		t.Fatalf("GetConversation returned error: %v", err)
	}
	if !conversation.HasRunningRun {
		t.Fatalf("expected running conversation to stay active, got %+v", conversation)
	}

	finishedAt := startedAt.Add(2 * time.Second)
	run, err = store.UpdateAgentRun(ctx, "ws_1", "conv_1", run.ID, map[string]any{
		"status":      "done",
		"finished_at": finishedAt,
	})
	if err != nil {
		t.Fatalf("UpdateAgentRun done returned error: %v", err)
	}
	if run.Status != "done" || run.FinishedAt == nil {
		t.Fatalf("unexpected terminal run: %+v", run)
	}
	conversation, err = store.GetConversation(ctx, "ws_1", conversation.ID)
	if err != nil {
		t.Fatalf("GetConversation returned error: %v", err)
	}
	if conversation.HasRunningRun {
		t.Fatalf("expected done conversation to clear active flag, got %+v", conversation)
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
		Name:           "run_command",
		InputJSON:      `{"command":"git status"}`,
		OutputJSON:     "{}",
		Status:         "running",
		StartedAt:      &startedAt,
	})
	if err != nil {
		t.Fatalf("CreateAgentToolCall returned error: %v", err)
	}
	finishedAt = startedAt.Add(time.Second)
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

func TestUpdateAgentRunKeepsConversationActiveWhileAnotherRunIsActive(t *testing.T) {
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

	conversation, err := store.CreateConversation(ctx, ConversationRecord{
		ID:          "conv_1",
		WorkspaceID: "ws_1",
		Title:       "Tracked",
	})
	if err != nil {
		t.Fatalf("CreateConversation returned error: %v", err)
	}
	firstRun, err := store.CreateAgentRun(ctx, AgentRunRecord{
		WorkspaceID:      "ws_1",
		ConversationID:   conversation.ID,
		TriggerMessageID: "msg_1",
		Model:            "gpt-5.5",
		ReasoningEffort:  "medium",
		Status:           "queued",
	})
	if err != nil {
		t.Fatalf("CreateAgentRun first returned error: %v", err)
	}
	secondRun, err := store.CreateAgentRun(ctx, AgentRunRecord{
		WorkspaceID:      "ws_1",
		ConversationID:   conversation.ID,
		TriggerMessageID: "msg_2",
		Model:            "gpt-5.5",
		ReasoningEffort:  "medium",
		Status:           "running",
	})
	if err != nil {
		t.Fatalf("CreateAgentRun second returned error: %v", err)
	}

	if _, err := store.UpdateAgentRun(ctx, "ws_1", conversation.ID, firstRun.ID, map[string]any{
		"status":      "done",
		"finished_at": time.Now().UTC(),
	}); err != nil {
		t.Fatalf("UpdateAgentRun first done returned error: %v", err)
	}
	conversation, err = store.GetConversation(ctx, "ws_1", conversation.ID)
	if err != nil {
		t.Fatalf("GetConversation returned error: %v", err)
	}
	if !conversation.HasRunningRun {
		t.Fatalf("expected second active run to keep flag true, got %+v", conversation)
	}

	if _, err := store.UpdateAgentRun(ctx, "ws_1", conversation.ID, secondRun.ID, map[string]any{
		"status":      "canceled",
		"finished_at": time.Now().UTC(),
	}); err != nil {
		t.Fatalf("UpdateAgentRun second canceled returned error: %v", err)
	}
	conversation, err = store.GetConversation(ctx, "ws_1", conversation.ID)
	if err != nil {
		t.Fatalf("GetConversation returned error: %v", err)
	}
	if conversation.HasRunningRun {
		t.Fatalf("expected no active runs after terminal transitions, got %+v", conversation)
	}
}

func TestListActiveAgentRuns(t *testing.T) {
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

	for _, conversationID := range []string{"conv_1", "conv_2", "conv_3", "conv_4"} {
		if _, err := store.CreateConversation(ctx, ConversationRecord{
			ID:          conversationID,
			WorkspaceID: "ws_1",
			Title:       "Tracked",
		}); err != nil {
			t.Fatalf("CreateConversation returned error: %v", err)
		}
	}
	for _, run := range []AgentRunRecord{
		{WorkspaceID: "ws_1", ConversationID: "conv_1", TriggerMessageID: "msg_1", Model: "gpt-5.5", ReasoningEffort: "medium", Status: "queued"},
		{WorkspaceID: "ws_1", ConversationID: "conv_2", TriggerMessageID: "msg_2", Model: "gpt-5.5", ReasoningEffort: "medium", Status: "running"},
		{WorkspaceID: "ws_1", ConversationID: "conv_3", TriggerMessageID: "msg_3", Model: "gpt-5.5", ReasoningEffort: "medium", Status: "waiting_tool_approval"},
		{WorkspaceID: "ws_1", ConversationID: "conv_4", TriggerMessageID: "msg_4", Model: "gpt-5.5", ReasoningEffort: "medium", Status: "done"},
	} {
		if _, err := store.CreateAgentRun(ctx, run); err != nil {
			t.Fatalf("CreateAgentRun returned error: %v", err)
		}
	}

	active, err := store.ListActiveAgentRuns(ctx)
	if err != nil {
		t.Fatalf("ListActiveAgentRuns returned error: %v", err)
	}
	if len(active) != 3 {
		t.Fatalf("expected 3 active runs, got %+v", active)
	}
	if active[0].Status != "queued" || active[1].Status != "running" || active[2].Status != "waiting_tool_approval" {
		t.Fatalf("unexpected active runs: %+v", active)
	}
}
