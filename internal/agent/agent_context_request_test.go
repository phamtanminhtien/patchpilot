package agent

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/phamtanminhtien/patchpilot/internal/database"
)

func TestManagerSendsConversationContextToProvider(t *testing.T) {
	root := initAgentGitRepo(t)
	provider := &testProvider{}
	manager, store := newAgentTestManager(t, provider)
	conversation, err := store.CreateConversation(context.Background(), database.ConversationRecord{
		WorkspaceID: "ws_1",
		Title:       "Follow up",
	})
	if err != nil {
		t.Fatalf("CreateConversation returned error: %v", err)
	}
	if _, err := store.CreateMessage(context.Background(), database.MessageRecord{
		WorkspaceID:    "ws_1",
		ConversationID: conversation.ID,
		Role:           "user",
		Content:        "Earlier user request",
	}); err != nil {
		t.Fatalf("CreateMessage returned error: %v", err)
	}
	trigger, err := store.CreateMessage(context.Background(), database.MessageRecord{
		WorkspaceID:    "ws_1",
		ConversationID: conversation.ID,
		Role:           "user",
		Content:        "Now continue",
	})
	if err != nil {
		t.Fatalf("CreateMessage trigger returned error: %v", err)
	}

	run, err := manager.Create(context.Background(), "ws_1", root, CreateRunInput{
		Prompt:           trigger.Content,
		ConversationID:   conversation.ID,
		TriggerMessageID: trigger.ID,
		Model:            "gpt-5.5",
		ReasoningEffort:  "medium",
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	waitForAgentRun(t, manager, "ws_1", run.ConversationID, run.ID, StatusDone)

	if len(provider.seen) != 1 {
		t.Fatalf("expected one provider request, got %+v", provider.seen)
	}
	context := provider.seen[0].ConversationContext
	if len(context) != 2 || context[0].Content != "Earlier user request" || context[1].Content != "Now continue" {
		t.Fatalf("expected conversation context through trigger, got %+v", context)
	}
}

func TestManagerSummarizesOlderConversationContext(t *testing.T) {
	root := initAgentGitRepo(t)
	provider := &testProvider{summary: "compressed decisions"}
	manager, store := newAgentTestManager(t, provider)
	var trigger database.MessageRecord
	for i := 0; i < 9; i++ {
		message, err := store.CreateMessage(context.Background(), database.MessageRecord{
			WorkspaceID:    "ws_1",
			ConversationID: "conv_1",
			Role:           "user",
			Content:        strings.Repeat("long context ", 2500) + string(rune('a'+i)),
		})
		if err != nil {
			t.Fatalf("CreateMessage returned error: %v", err)
		}
		trigger = message
	}

	run, err := manager.Create(context.Background(), "ws_1", root, CreateRunInput{
		Prompt:           trigger.Content,
		ConversationID:   "conv_1",
		TriggerMessageID: trigger.ID,
		Model:            "gpt-5.5",
		ReasoningEffort:  "medium",
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	waitForAgentRun(t, manager, "ws_1", run.ConversationID, run.ID, StatusDone)

	if len(provider.summaryRequests) != 1 {
		t.Fatalf("expected one summary request, got %+v", provider.summaryRequests)
	}
	if len(provider.summaryRequests[0].Messages) == 0 {
		t.Fatalf("expected older messages in summary request")
	}
	conversation, err := store.GetConversation(context.Background(), "ws_1", "conv_1")
	if err != nil {
		t.Fatalf("GetConversation returned error: %v", err)
	}
	if conversation.ContextSummary != "compressed decisions" || conversation.ContextSummaryThroughMessageID == nil {
		t.Fatalf("expected persisted context summary, got %+v", conversation)
	}
	if len(provider.seen) != 1 || provider.seen[0].ContextSummary != "compressed decisions" {
		t.Fatalf("expected main provider call to receive summary, got %+v", provider.seen)
	}
}

func TestManagerReadsFilesThroughCommandAndSearchScope(t *testing.T) {
	root := initAgentGitRepo(t)
	if err := os.MkdirAll(filepath.Join(root, "src"), 0o755); err != nil {
		t.Fatalf("mkdir src: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0o755); err != nil {
		t.Fatalf("mkdir docs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "src", "note.txt"), []byte("one\ntwo\nthree\n"), 0o644); err != nil {
		t.Fatalf("write src note: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "note.txt"), []byte("two in docs\n"), 0o644); err != nil {
		t.Fatalf("write docs note: %v", err)
	}
	provider := &testProvider{turns: []ProviderResult{
		{ToolCalls: []ToolRequest{
			{CallID: "call_read", Name: "run_command", Arguments: `{"command":"cat src/note.txt"}`},
			{CallID: "call_search", Name: "search_files", Arguments: `{"query":"two","path":"src"}`},
		}},
		{Text: "done", Done: true},
	}}
	manager, _ := newAgentTestManager(t, provider)

	run, err := manager.Create(context.Background(), "ws_1", root, CreateRunInput{
		Prompt:           "inspect note",
		ConversationID:   "conv_1",
		TriggerMessageID: "msg_1",
		Model:            "gpt-5.5",
		ReasoningEffort:  "medium",
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	detail := waitForAgentRun(t, manager, "ws_1", run.ConversationID, run.ID, StatusDone)
	if len(detail.ToolCalls) != 2 {
		t.Fatalf("expected two tool calls, got %+v", detail.ToolCalls)
	}
	if !strings.Contains(detail.ToolCalls[0].Output, `"output":"one\ntwo\nthree\n"`) {
		t.Fatalf("expected command read output, got %s", detail.ToolCalls[0].Output)
	}
	if !strings.Contains(detail.ToolCalls[1].Output, `"path":"src/note.txt"`) || strings.Contains(detail.ToolCalls[1].Output, `"path":"docs/note.txt"`) {
		t.Fatalf("expected scoped search output, got %s", detail.ToolCalls[1].Output)
	}
}

func TestManagerValidatesInputAndProvider(t *testing.T) {
	manager, _ := newAgentTestManager(t, unconfiguredProvider{})
	_, err := manager.Create(context.Background(), "ws_1", t.TempDir(), CreateRunInput{
		Prompt:           "fix",
		ConversationID:   "conv_1",
		TriggerMessageID: "msg_1",
		Model:            "gpt-5.5",
		ReasoningEffort:  "medium",
	})
	if !errors.Is(err, ErrProviderUnavailable) {
		t.Fatalf("expected provider unavailable, got %v", err)
	}

	manager, _ = newAgentTestManager(t, &testProvider{})
	_, err = manager.Create(context.Background(), "ws_1", t.TempDir(), CreateRunInput{
		Prompt:           "",
		ConversationID:   "conv_1",
		TriggerMessageID: "msg_1",
		Model:            "gpt-5.5",
		ReasoningEffort:  "medium",
	})
	if !errors.Is(err, ErrEmptyPrompt) {
		t.Fatalf("expected empty prompt, got %v", err)
	}
	_, err = manager.Create(context.Background(), "ws_1", t.TempDir(), CreateRunInput{
		Prompt:           "fix",
		ConversationID:   "conv_1",
		TriggerMessageID: "msg_1",
		Model:            "unknown",
		ReasoningEffort:  "medium",
	})
	if !errors.Is(err, ErrInvalidModel) {
		t.Fatalf("expected invalid model, got %v", err)
	}
	_, err = manager.Create(context.Background(), "ws_1", t.TempDir(), CreateRunInput{
		Prompt:           "fix",
		ConversationID:   "conv_1",
		TriggerMessageID: "msg_1",
		Model:            "gpt-5.5",
		ReasoningEffort:  "none",
	})
	if !errors.Is(err, ErrInvalidEffort) {
		t.Fatalf("expected invalid effort, got %v", err)
	}
}

func patchToolRequest(callID string) ToolRequest {
	return ToolRequest{
		CallID:    callID,
		Name:      "apply_patch",
		Arguments: `{"summary":"add file","diff":"diff --git a/a.txt b/a.txt\nnew file mode 100644\nindex 0000000..7898192\n--- /dev/null\n+++ b/a.txt\n@@ -0,0 +1 @@\n+content\n"}`,
	}
}
