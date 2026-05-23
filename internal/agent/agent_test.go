package agent

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
	mu              sync.Mutex
	err             error
	summary         string
	streamDeltas    []string
	turns           []ProviderResult
	seen            []ProviderRequest
	summaryRequests []SummaryRequest
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
		for _, delta := range p.streamDeltas {
			if stream != nil {
				stream.Delta(ctx, delta)
			}
		}
		return ProviderResult{Text: "done", Done: true}, nil
	}
	turn := p.turns[0]
	p.turns = p.turns[1:]
	for _, delta := range p.streamDeltas {
		if stream != nil {
			stream.Delta(ctx, delta)
		}
	}
	return turn, nil
}

func (p *testProvider) Summarize(ctx context.Context, request SummaryRequest) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.summaryRequests = append(p.summaryRequests, request)
	if p.err != nil {
		return "", p.err
	}
	if p.summary != "" {
		return p.summary, nil
	}
	return "summarized older context", nil
}

type unconfiguredProvider struct{}

func (unconfiguredProvider) Configured() bool {
	return false
}

func (unconfiguredProvider) Generate(context.Context, ProviderRequest, Stream) (ProviderResult, error) {
	return ProviderResult{}, ErrProviderUnavailable
}

func (unconfiguredProvider) Summarize(context.Context, SummaryRequest) (string, error) {
	return "", ErrProviderUnavailable
}

type blockingTestProvider struct {
	delta string
	done  chan struct{}
}

func (p blockingTestProvider) Configured() bool {
	return true
}

func (p blockingTestProvider) Generate(ctx context.Context, request ProviderRequest, stream Stream) (ProviderResult, error) {
	if stream != nil {
		stream.Delta(ctx, p.delta)
	}
	if p.done != nil {
		close(p.done)
	}
	<-ctx.Done()
	return ProviderResult{}, ctx.Err()
}

func (p blockingTestProvider) Summarize(context.Context, SummaryRequest) (string, error) {
	return "fake summary", nil
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

func TestManagerPublishesTransientDeltaWithoutStoringIt(t *testing.T) {
	root := initAgentGitRepo(t)
	provider := &testProvider{streamDeltas: []string{"hel", "lo"}}
	hub := events.NewHub()
	manager, store := newAgentTestManagerWithHub(t, provider, hub)
	eventsCh, unsubscribe := hub.Subscribe()
	defer unsubscribe()

	run, err := manager.Create(context.Background(), "ws_1", root, CreateRunInput{
		Prompt:           "say hello",
		ConversationID:   "conv_1",
		TriggerMessageID: "msg_1",
		Model:            "gpt-5.5",
		ReasoningEffort:  "medium",
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	waitForAgentRun(t, manager, "ws_1", run.ConversationID, run.ID, StatusDone)
	if !receivedAgentDelta(eventsCh) {
		t.Fatal("expected transient agent.delta on hub")
	}
	records, err := store.ListAgentRunEvents(context.Background(), "ws_1", run.ID)
	if err != nil {
		t.Fatalf("ListAgentRunEvents returned error: %v", err)
	}
	if hasEvent(records, "agent.delta") {
		t.Fatalf("agent.delta should not be stored, got %+v", records)
	}
}

func TestManagerStreamsWhitespaceOnlyDeltas(t *testing.T) {
	hub := events.NewHub()
	manager := NewManager(nil, nil, nil, nil, hub, nil)
	manager.setRuntime(&runRuntime{runID: "run_1"})
	eventsCh, unsubscribe := hub.Subscribe()
	defer unsubscribe()

	stream := manager.stream(Run{ID: "run_1", WorkspaceID: "ws_1"})
	for _, delta := range []string{"##", " ", "1", "\n", "```txt", "\n", "internal/"} {
		stream.Delta(context.Background(), delta)
	}

	const want = "## 1\n```txt\ninternal/"
	if got := manager.DraftText("run_1"); got != want {
		t.Fatalf("expected draft text to preserve whitespace, got %q", got)
	}
	if got := receiveAgentDeltaText(t, eventsCh, 7); got != want {
		t.Fatalf("expected streamed text to preserve whitespace, got %q", got)
	}
}

func TestManagerDraftTextTracksActiveStreamOnly(t *testing.T) {
	root := initAgentGitRepo(t)
	provider := &testProvider{
		streamDeltas: []string{"draft ", "text"},
		turns: []ProviderResult{{
			Text:      "I will inspect and prepare the patch.",
			ToolCalls: []ToolRequest{patchToolRequest("call_patch")},
		}},
	}
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
	if got := manager.DraftText(run.ID); got != "" {
		t.Fatalf("draft should clear after assistant message persistence, got %q", got)
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

func hasAgentDeltaText(events []database.AgentRunEventRecord, text string) bool {
	for _, event := range events {
		if event.Type != "agent.delta" {
			continue
		}
		if strings.Contains(event.PayloadJSON, text) {
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
	return newAgentTestManagerWithHub(t, provider, events.NewHub())
}

func newAgentTestManagerWithHub(t *testing.T, provider Provider, hub *events.Hub) (*Manager, *database.Store) {
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
	manager := NewManager(store, filestore.NewService(), gitrepo.NewClient(), runner.NewRunner(), hub, provider)
	if _, err := store.CreateConversation(context.Background(), database.ConversationRecord{
		ID:          "conv_1",
		WorkspaceID: "ws_1",
		Title:       "Test conversation",
	}); err != nil {
		t.Fatalf("CreateConversation returned error: %v", err)
	}
	return manager, store
}

func receivedAgentDelta(eventsCh <-chan events.Event) bool {
	deadline := time.After(500 * time.Millisecond)
	for {
		select {
		case event := <-eventsCh:
			if event.Type == "agent.delta" {
				return true
			}
		case <-deadline:
			return false
		}
	}
}

func receiveAgentDeltaText(t *testing.T, eventsCh <-chan events.Event, count int) string {
	t.Helper()
	var builder strings.Builder
	received := 0
	deadline := time.After(500 * time.Millisecond)
	for received < count {
		select {
		case event := <-eventsCh:
			if event.Type != "agent.delta" {
				continue
			}
			payload, ok := event.Payload.(map[string]string)
			if !ok {
				t.Fatalf("expected agent.delta payload map, got %T", event.Payload)
			}
			builder.WriteString(payload["text"])
			received++
		case <-deadline:
			t.Fatalf("timed out waiting for %d agent.delta events, got %q", count, builder.String())
		}
	}
	return builder.String()
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
