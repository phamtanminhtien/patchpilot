package agent

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/phamtanminhtien/patchpilot/internal/database"
	"github.com/phamtanminhtien/patchpilot/internal/events"
	"github.com/phamtanminhtien/patchpilot/internal/filestore"
	"github.com/phamtanminhtien/patchpilot/internal/gitrepo"
	"github.com/phamtanminhtien/patchpilot/internal/runner"
)

type testProvider struct {
	mu    sync.Mutex
	err   error
	turns []ProviderResult
	seen  []ProviderRequest
}

func (p *testProvider) Configured() bool {
	return true
}

func (p *testProvider) Generate(ctx context.Context, request ProviderRequest, stream Stream) (ProviderResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.seen = append(p.seen, request)
	if p.err != nil {
		return ProviderResult{}, p.err
	}
	if len(p.turns) == 0 {
		return ProviderResult{Text: "done", Done: true}, nil
	}
	turn := p.turns[0]
	p.turns = p.turns[1:]
	return turn, nil
}

type unconfiguredProvider struct{}

func (unconfiguredProvider) Configured() bool {
	return false
}

func (unconfiguredProvider) Generate(context.Context, ProviderRequest, Stream) (ProviderResult, error) {
	return ProviderResult{}, ErrProviderUnavailable
}

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
	messages, err := store.ListMessages(context.Background(), "ws_1", run.ConversationID)
	if err != nil {
		t.Fatalf("ListMessages returned error: %v", err)
	}
	if !hasAgentMessage(messages, "assistant", "I will inspect and prepare the patch.") {
		t.Fatalf("expected assistant progress message, got %+v", messages)
	}
}

func TestManagerApprovesPatchToolAndResumesAgent(t *testing.T) {
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

func hasEvent(events []database.AgentRunEventRecord, eventType string) bool {
	for _, event := range events {
		if event.Type == eventType {
			return true
		}
	}
	return false
}

func hasAgentMessage(messages []database.MessageRecord, role, content string) bool {
	for _, message := range messages {
		if message.Role == role && message.Content == content {
			return true
		}
	}
	return false
}

func newAgentTestManager(t *testing.T, provider Provider) (*Manager, *database.Store) {
	t.Helper()
	store, err := database.Open(filepath.Join(t.TempDir(), "patchpilot.db"))
	if err != nil {
		t.Fatalf("database.Open returned error: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	})
	manager := NewManager(store, filestore.NewService(), gitrepo.NewClient(), runner.NewRunner(), events.NewHub(), provider)
	return manager, store
}

func waitForAgentRun(t *testing.T, manager *Manager, workspaceID, conversationID, runID string, status RunStatus) Detail {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		detail, err := manager.Detail(context.Background(), workspaceID, conversationID, runID)
		if err != nil {
			t.Fatalf("Detail returned error: %v", err)
		}
		if detail.Run.Status == string(status) {
			return detail
		}
		time.Sleep(20 * time.Millisecond)
	}
	detail, _ := manager.Detail(context.Background(), workspaceID, conversationID, runID)
	taskError := ""
	if detail.Run.Error != nil {
		taskError = *detail.Run.Error
	}
	t.Fatalf("run did not reach %s: %+v error=%q", status, detail.Run, taskError)
	return Detail{}
}

func initAgentGitRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	cmd := exec.Command("git", "init")
	cmd.Dir = root
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v\n%s", err, output)
	}
	return root
}

func readAgentFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ""
		}
		t.Fatalf("read file: %v", err)
	}
	return string(content)
}
