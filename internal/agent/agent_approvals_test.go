package agent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/phamtanminhtien/patchpilot/internal/config"
)

func TestManagerCreatesApprovalRequiredPatchTool(t *testing.T) {
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

	detail := waitForAgentRun(t, manager, "ws_1", run.ConversationID, run.ID, StatusWaitingToolApproval)
	if detail.Run.Model != "gpt-5.5" || detail.Run.ReasoningEffort != "medium" {
		t.Fatalf("unexpected run selections: %+v", detail.Run)
	}
	if len(detail.ToolCalls) != 1 || detail.ToolCalls[0].Name != "apply_patch" || !detail.ToolCalls[0].RequiresApproval {
		t.Fatalf("expected pending patch tool, got %+v", detail.ToolCalls)
	}
	if got := readAgentFile(t, filepath.Join(root, "a.txt")); got != "" {
		t.Fatalf("patch should not be applied before approval, got %q", got)
	}
	events, err := store.ListAgentRunEvents(context.Background(), "ws_1", run.ID)
	if err != nil {
		t.Fatalf("ListAgentRunEvents returned error: %v", err)
	}
	if !hasEvent(events, "agent.approval_required") {
		t.Fatalf("expected approval event, got %+v", events)
	}
	if hasAgentDeltaText(events, "Preparing workspace context.") {
		t.Fatalf("workspace preparation progress should not be stored as UI event, got %+v", events)
	}
	messages, err := store.ListMessages(context.Background(), "ws_1", run.ConversationID)
	if err != nil {
		t.Fatalf("ListMessages returned error: %v", err)
	}
	if !hasAgentMessage(messages, "assistant", "I will inspect and prepare the patch.") {
		t.Fatalf("expected assistant progress message, got %+v", messages)
	}
	conversation, err := store.GetConversation(context.Background(), "ws_1", run.ConversationID)
	if err != nil {
		t.Fatalf("GetConversation returned error: %v", err)
	}
	if !conversation.HasRunningRun {
		t.Fatalf("expected waiting approval conversation to stay active, got %+v", conversation)
	}
}

func TestManagerApprovesPatchToolAndResumesAgent(t *testing.T) {
	root := initAgentGitRepo(t)
	provider := &testProvider{turns: []ProviderResult{
		{ToolCalls: []ToolRequest{patchToolRequest("call_patch")}},
		{Text: "Patch applied.", Done: true},
	}}
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
	detail := waitForAgentRun(t, manager, "ws_1", run.ConversationID, run.ID, StatusWaitingToolApproval)

	if _, err := manager.ApproveToolCall(context.Background(), "ws_1", run.ID, detail.ToolCalls[0].ID); err != nil {
		t.Fatalf("ApproveToolCall returned error: %v", err)
	}
	detail = waitForAgentRun(t, manager, "ws_1", run.ConversationID, run.ID, StatusDone)
	if got := readAgentFile(t, filepath.Join(root, "a.txt")); got != "content\n" {
		t.Fatalf("expected applied patch, got %q", got)
	}
	if detail.ToolCalls[0].Status != ToolStatusFinished {
		t.Fatalf("expected finished tool call, got %+v", detail.ToolCalls)
	}
	if len(provider.seen) < 2 || len(provider.seen[1].History) == 0 {
		t.Fatalf("expected resumed provider history, got %+v", provider.seen)
	}
	conversation, err := store.GetConversation(context.Background(), "ws_1", run.ConversationID)
	if err != nil {
		t.Fatalf("GetConversation returned error: %v", err)
	}
	if conversation.HasRunningRun {
		t.Fatalf("expected done conversation to clear active flag, got %+v", conversation)
	}
}

func TestManagerRejectsPatchToolAndResumesAgent(t *testing.T) {
	root := initAgentGitRepo(t)
	provider := &testProvider{turns: []ProviderResult{
		{ToolCalls: []ToolRequest{patchToolRequest("call_patch")}},
		{Text: "Patch rejected.", Done: true},
	}}
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
	detail := waitForAgentRun(t, manager, "ws_1", run.ConversationID, run.ID, StatusWaitingToolApproval)

	if _, err := manager.RejectToolCall(context.Background(), "ws_1", run.ID, detail.ToolCalls[0].ID); err != nil {
		t.Fatalf("RejectToolCall returned error: %v", err)
	}
	detail = waitForAgentRun(t, manager, "ws_1", run.ConversationID, run.ID, StatusDone)
	if got := readAgentFile(t, filepath.Join(root, "a.txt")); got != "" {
		t.Fatalf("rejected patch should not be applied, got %q", got)
	}
	if detail.ToolCalls[0].Status != ToolStatusRejected {
		t.Fatalf("expected rejected tool call, got %+v", detail.ToolCalls)
	}
}

func TestManagerWaitsForApprovalsBeforeExecutingMixedBatch(t *testing.T) {
	root := initAgentGitRepo(t)
	if err := os.WriteFile(filepath.Join(root, "note.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatalf("write note: %v", err)
	}
	provider := &testProvider{turns: []ProviderResult{
		{ToolCalls: []ToolRequest{
			{CallID: "call_search", Name: "search_files", Arguments: `{"query":"hello"}`},
			patchToolRequest("call_patch"),
		}},
		{Text: "done", Done: true},
	}}
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
	detail := waitForAgentRun(t, manager, "ws_1", run.ConversationID, run.ID, StatusWaitingToolApproval)
	if len(detail.ToolCalls) != 2 {
		t.Fatalf("expected two tool calls, got %+v", detail.ToolCalls)
	}
	if detail.ToolCalls[0].Status != ToolStatusPending {
		t.Fatalf("safe tool should wait while batch approvals are pending: %+v", detail.ToolCalls)
	}

	if _, err := manager.ApproveToolCall(context.Background(), "ws_1", run.ID, detail.ToolCalls[1].ID); err != nil {
		t.Fatalf("ApproveToolCall returned error: %v", err)
	}
	detail = waitForAgentRun(t, manager, "ws_1", run.ConversationID, run.ID, StatusDone)
	if detail.ToolCalls[0].Status != ToolStatusFinished || detail.ToolCalls[1].Status != ToolStatusFinished {
		t.Fatalf("expected both approved/safe tools to finish, got %+v", detail.ToolCalls)
	}
}

func TestManagerAutonomousAppliesPatchWithoutApproval(t *testing.T) {
	root := initAgentGitRepo(t)
	provider := &testProvider{turns: []ProviderResult{
		{ToolCalls: []ToolRequest{patchToolRequest("call_patch")}},
		{Text: "Patch applied.", Done: true},
	}}
	manager, _ := newAgentTestManager(t, provider)

	run, err := manager.Create(context.Background(), "ws_1", root, CreateRunInput{
		Prompt:           "fix bug",
		ConversationID:   "conv_1",
		TriggerMessageID: "msg_1",
		Model:            "gpt-5.5",
		ReasoningEffort:  "medium",
		Permissions:      config.WorkspacePermission{Mode: "autonomous", EditFiles: true, RunCommands: true, GitOperations: true},
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	detail := waitForAgentRun(t, manager, "ws_1", run.ConversationID, run.ID, StatusDone)
	if got := readAgentFile(t, filepath.Join(root, "a.txt")); got != "content\n" {
		t.Fatalf("expected autonomous patch to apply, got %q", got)
	}
	if len(detail.ToolCalls) != 1 || detail.ToolCalls[0].RequiresApproval || detail.ToolCalls[0].Status != ToolStatusFinished {
		t.Fatalf("expected patch to run without approval, got %+v", detail.ToolCalls)
	}
}

func TestManagerSafeModeRequiresApprovalForSafeCommand(t *testing.T) {
	root := initAgentGitRepo(t)
	provider := &testProvider{turns: []ProviderResult{{
		ToolCalls: []ToolRequest{{CallID: "call_cmd", Name: "run_command", Arguments: `{"command":"git status"}`}},
	}}}
	manager, _ := newAgentTestManager(t, provider)

	run, err := manager.Create(context.Background(), "ws_1", root, CreateRunInput{
		Prompt:           "check status",
		ConversationID:   "conv_1",
		TriggerMessageID: "msg_1",
		Model:            "gpt-5.5",
		ReasoningEffort:  "medium",
		Permissions:      config.WorkspacePermission{Mode: "safe", EditFiles: true, RunCommands: true, GitOperations: true},
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	detail := waitForAgentRun(t, manager, "ws_1", run.ConversationID, run.ID, StatusWaitingToolApproval)
	if len(detail.ToolCalls) != 1 || !detail.ToolCalls[0].RequiresApproval {
		t.Fatalf("expected safe command approval, got %+v", detail.ToolCalls)
	}
}

func TestManagerBalancedRunsSafeCommandWithoutApproval(t *testing.T) {
	root := initAgentGitRepo(t)
	provider := &testProvider{turns: []ProviderResult{
		{ToolCalls: []ToolRequest{{CallID: "call_cmd", Name: "run_command", Arguments: `{"command":"git status"}`}}},
		{Text: "done", Done: true},
	}}
	manager, _ := newAgentTestManager(t, provider)

	run, err := manager.Create(context.Background(), "ws_1", root, CreateRunInput{
		Prompt:           "check status",
		ConversationID:   "conv_1",
		TriggerMessageID: "msg_1",
		Model:            "gpt-5.5",
		ReasoningEffort:  "medium",
		Permissions:      config.WorkspacePermission{Mode: "balanced", EditFiles: true, RunCommands: true, GitOperations: true},
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	detail := waitForAgentRun(t, manager, "ws_1", run.ConversationID, run.ID, StatusDone)
	if len(detail.ToolCalls) != 1 || detail.ToolCalls[0].RequiresApproval || detail.ToolCalls[0].Status != ToolStatusFinished {
		t.Fatalf("expected balanced safe command to run, got %+v", detail.ToolCalls)
	}
}

func TestManagerAutonomousRunsConfirmationCommandWithoutApproval(t *testing.T) {
	root := initAgentGitRepo(t)
	if err := os.WriteFile(filepath.Join(root, ".env"), []byte("SECRET=value\n"), 0o600); err != nil {
		t.Fatalf("write .env: %v", err)
	}
	provider := &testProvider{turns: []ProviderResult{
		{ToolCalls: []ToolRequest{{CallID: "call_cmd", Name: "run_command", Arguments: `{"command":"cat .env"}`}}},
		{Text: "done", Done: true},
	}}
	manager, _ := newAgentTestManager(t, provider)

	run, err := manager.Create(context.Background(), "ws_1", root, CreateRunInput{
		Prompt:           "read env",
		ConversationID:   "conv_1",
		TriggerMessageID: "msg_1",
		Model:            "gpt-5.5",
		ReasoningEffort:  "medium",
		Permissions:      config.WorkspacePermission{Mode: "autonomous", EditFiles: true, RunCommands: true, GitOperations: true},
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	detail := waitForAgentRun(t, manager, "ws_1", run.ConversationID, run.ID, StatusDone)
	if len(detail.ToolCalls) != 1 || detail.ToolCalls[0].RequiresApproval || detail.ToolCalls[0].Status != ToolStatusFinished {
		t.Fatalf("expected autonomous confirmation command to run, got %+v", detail.ToolCalls)
	}
}

func TestManagerDisabledPermissionsBlockTools(t *testing.T) {
	tests := []struct {
		name        string
		permission  config.WorkspacePermission
		toolRequest ToolRequest
		wantReason  string
	}{
		{
			name:        "edit files",
			permission:  config.WorkspacePermission{Mode: "autonomous", EditFiles: false, RunCommands: true, GitOperations: true},
			toolRequest: patchToolRequest("call_patch"),
			wantReason:  "file edits are disabled",
		},
		{
			name:        "run commands",
			permission:  config.WorkspacePermission{Mode: "autonomous", EditFiles: true, RunCommands: false, GitOperations: true},
			toolRequest: ToolRequest{CallID: "call_cmd", Name: "run_command", Arguments: `{"command":"git status"}`},
			wantReason:  "command execution is disabled",
		},
		{
			name:        "git operations",
			permission:  config.WorkspacePermission{Mode: "autonomous", EditFiles: true, RunCommands: true, GitOperations: false},
			toolRequest: ToolRequest{CallID: "call_cmd", Name: "run_command", Arguments: `{"command":"git status"}`},
			wantReason:  "git operations are disabled",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := initAgentGitRepo(t)
			provider := &testProvider{turns: []ProviderResult{
				{ToolCalls: []ToolRequest{tt.toolRequest}},
				{Text: "done", Done: true},
			}}
			manager, _ := newAgentTestManager(t, provider)

			run, err := manager.Create(context.Background(), "ws_1", root, CreateRunInput{
				Prompt:           "try tool",
				ConversationID:   "conv_1",
				TriggerMessageID: "msg_1",
				Model:            "gpt-5.5",
				ReasoningEffort:  "medium",
				Permissions:      tt.permission,
			})
			if err != nil {
				t.Fatalf("Create returned error: %v", err)
			}
			detail := waitForAgentRun(t, manager, "ws_1", run.ConversationID, run.ID, StatusDone)
			if len(detail.ToolCalls) != 1 || detail.ToolCalls[0].Status != ToolStatusFailed || !strings.Contains(detail.ToolCalls[0].Output, tt.wantReason) {
				t.Fatalf("expected blocked failed tool containing %q, got %+v", tt.wantReason, detail.ToolCalls)
			}
		})
	}
}
