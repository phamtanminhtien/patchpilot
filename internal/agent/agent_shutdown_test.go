package agent

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/phamtanminhtien/patchpilot/internal/database"
)

func TestManagerCancelClearsConversationActiveRunFlag(t *testing.T) {
	root := initAgentGitRepo(t)
	done := make(chan struct{})
	provider := blockingTestProvider{
		delta: "working",
		done:  done,
	}
	manager, store := newAgentTestManager(t, provider)

	run, err := manager.Create(context.Background(), "ws_1", root, CreateRunInput{
		Prompt:           "fix bug",
		ConversationID:   "conv_1",
		TriggerMessageID: "msg_1",
		Model:            "gpt-5.5",
		ReasoningEffort:  "medium",
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	<-done
	waitForAgentRun(t, manager, "ws_1", run.ConversationID, run.ID, StatusRunning)

	if _, err := manager.Cancel(context.Background(), "ws_1", run.ConversationID, run.ID); err != nil {
		t.Fatalf("Cancel returned error: %v", err)
	}
	waitForAgentRun(t, manager, "ws_1", run.ConversationID, run.ID, StatusCanceled)
	conversation, err := store.GetConversation(context.Background(), "ws_1", run.ConversationID)
	if err != nil {
		t.Fatalf("GetConversation returned error: %v", err)
	}
	if conversation.HasRunningRun {
		t.Fatalf("expected canceled conversation to clear active flag, got %+v", conversation)
	}
}

func TestManagerCancelRejectsWaitingApprovalToolCalls(t *testing.T) {
	root := initAgentGitRepo(t)
	provider := &testProvider{turns: []ProviderResult{{
		Text:      "I will prepare the patch.",
		ToolCalls: []ToolRequest{patchToolRequest("call_patch")},
	}}}
	manager, _ := newAgentTestManager(t, provider)

	run, err := manager.Create(context.Background(), "ws_1", root, CreateRunInput{
		Prompt:           "fix bug",
		ConversationID:   "conv_1",
		TriggerMessageID: "msg_1",
		Model:            "gpt-5.5",
		ReasoningEffort:  "medium",
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	waitForAgentRun(t, manager, "ws_1", run.ConversationID, run.ID, StatusWaitingToolApproval)

	canceled, err := manager.Cancel(context.Background(), "ws_1", run.ConversationID, run.ID)
	if err != nil {
		t.Fatalf("Cancel returned error: %v", err)
	}
	if canceled.Status != string(StatusCanceled) {
		t.Fatalf("expected canceled run, got %+v", canceled)
	}
	detail := waitForAgentRun(t, manager, "ws_1", run.ConversationID, run.ID, StatusCanceled)
	if len(detail.ToolCalls) != 1 {
		t.Fatalf("expected one tool call, got %+v", detail.ToolCalls)
	}
	toolCall := detail.ToolCalls[0]
	if toolCall.Status != ToolStatusRejected {
		t.Fatalf("expected waiting approval tool call to be rejected, got %+v", toolCall)
	}
	if toolCall.Decision == nil || *toolCall.Decision != ToolStatusRejected {
		t.Fatalf("expected rejected decision, got %+v", toolCall)
	}
	if !strings.Contains(toolCall.Output, "canceled") {
		t.Fatalf("expected canceled output, got %+v", toolCall)
	}
	if got := readAgentFile(t, filepath.Join(root, "a.txt")); got != "" {
		t.Fatalf("canceled approval should not apply patch, got %q", got)
	}
}

func TestManagerShutdownFailsRunningRun(t *testing.T) {
	root := initAgentGitRepo(t)
	done := make(chan struct{})
	manager, store := newAgentTestManager(t, blockingTestProvider{
		delta: "working",
		done:  done,
	})

	run, err := manager.Create(context.Background(), "ws_1", root, CreateRunInput{
		Prompt:           "fix bug",
		ConversationID:   "conv_1",
		TriggerMessageID: "msg_1",
		Model:            "gpt-5.5",
		ReasoningEffort:  "medium",
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	<-done
	detail := waitForAgentRun(t, manager, "ws_1", run.ConversationID, run.ID, StatusRunning)

	if err := manager.Shutdown(context.Background(), shutdownFailureMessage); err != nil {
		t.Fatalf("Shutdown returned error: %v", err)
	}

	detail = waitForAgentRun(t, manager, "ws_1", run.ConversationID, run.ID, StatusFailed)
	if detail.Run.Error == nil || *detail.Run.Error != shutdownFailureMessage {
		t.Fatalf("expected shutdown error, got %+v", detail.Run)
	}
	if detail.Run.FinishedAt == nil {
		t.Fatalf("expected finished_at for failed run, got %+v", detail.Run)
	}
	conversation, err := store.GetConversation(context.Background(), "ws_1", run.ConversationID)
	if err != nil {
		t.Fatalf("GetConversation returned error: %v", err)
	}
	if conversation.HasRunningRun {
		t.Fatalf("expected shutdown to clear active flag, got %+v", conversation)
	}
	if manager.DraftText(run.ID) != "" {
		t.Fatalf("expected runtime to be removed after shutdown")
	}
}

func TestManagerShutdownFailsWaitingApprovalRunAndToolCalls(t *testing.T) {
	root := initAgentGitRepo(t)
	provider := &testProvider{turns: []ProviderResult{{
		Text:      "I will inspect and prepare the patch.",
		ToolCalls: []ToolRequest{patchToolRequest("call_patch")},
	}}}
	manager, store := newAgentTestManager(t, provider)

	run, err := manager.Create(context.Background(), "ws_1", root, CreateRunInput{
		Prompt:           "fix bug",
		ConversationID:   "conv_1",
		TriggerMessageID: "msg_1",
		Model:            "gpt-5.5",
		ReasoningEffort:  "medium",
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	waitForAgentRun(t, manager, "ws_1", run.ConversationID, run.ID, StatusWaitingToolApproval)

	if err := manager.Shutdown(context.Background(), shutdownFailureMessage); err != nil {
		t.Fatalf("Shutdown returned error: %v", err)
	}

	detail := waitForAgentRun(t, manager, "ws_1", run.ConversationID, run.ID, StatusFailed)
	if detail.Run.Error == nil || *detail.Run.Error != shutdownFailureMessage {
		t.Fatalf("expected shutdown error, got %+v", detail.Run)
	}
	if len(detail.ToolCalls) != 1 || detail.ToolCalls[0].Status != ToolStatusFailed {
		t.Fatalf("expected failed tool call after shutdown, got %+v", detail.ToolCalls)
	}
	if !strings.Contains(detail.ToolCalls[0].Output, shutdownFailureMessage) {
		t.Fatalf("expected shutdown reason in tool output, got %+v", detail.ToolCalls[0])
	}
	conversation, err := store.GetConversation(context.Background(), "ws_1", run.ConversationID)
	if err != nil {
		t.Fatalf("GetConversation returned error: %v", err)
	}
	if conversation.HasRunningRun {
		t.Fatalf("expected shutdown to clear active flag, got %+v", conversation)
	}
}

func TestManagerShutdownFailsQueuedRunWithoutRuntime(t *testing.T) {
	manager, store := newAgentTestManager(t, &testProvider{})
	run, err := store.CreateAgentRun(context.Background(), database.AgentRunRecord{
		WorkspaceID:      "ws_1",
		ConversationID:   "conv_1",
		TriggerMessageID: "msg_1",
		Model:            "gpt-5.5",
		ReasoningEffort:  "medium",
		Status:           string(StatusQueued),
	})
	if err != nil {
		t.Fatalf("CreateAgentRun returned error: %v", err)
	}

	if err := manager.Shutdown(context.Background(), shutdownFailureMessage); err != nil {
		t.Fatalf("Shutdown returned error: %v", err)
	}

	detail, err := manager.Detail(context.Background(), "ws_1", "conv_1", run.ID)
	if err != nil {
		t.Fatalf("Detail returned error: %v", err)
	}
	if detail.Run.Status != string(StatusFailed) || detail.Run.Error == nil || *detail.Run.Error != shutdownFailureMessage {
		t.Fatalf("expected queued run to fail on shutdown, got %+v", detail.Run)
	}
	conversation, err := store.GetConversation(context.Background(), "ws_1", "conv_1")
	if err != nil {
		t.Fatalf("GetConversation returned error: %v", err)
	}
	if conversation.HasRunningRun {
		t.Fatalf("expected shutdown to clear active flag, got %+v", conversation)
	}
}
