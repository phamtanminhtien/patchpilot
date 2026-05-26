package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/phamtanminhtien/patchpilot/internal/agent"
	"github.com/phamtanminhtien/patchpilot/internal/database"
	"github.com/phamtanminhtien/patchpilot/internal/events"
	"github.com/phamtanminhtien/patchpilot/internal/filestore"
	"github.com/phamtanminhtien/patchpilot/internal/gitrepo"
	"github.com/phamtanminhtien/patchpilot/internal/runner"
	"github.com/phamtanminhtien/patchpilot/internal/workspace"
)

func TestConversationRunHandlersCreateListAndGet(t *testing.T) {
	root := initGitRepo(t, t.TempDir())
	seedExampleFile(t, root)
	server := newTestServer(t, root)
	create := request(server, http.MethodPost, "/api/workspaces", `{"rootPath":"`+root+`"}`)
	var ws workspace.Workspace
	mustDecode(t, create, &ws)

	conversation := request(server, http.MethodPost, "/api/workspaces/"+ws.ID+"/conversations", `{"title":"Fix bug"}`)
	if conversation.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", conversation.Code, conversation.Body.String())
	}
	var createdConversation conversationResponse
	mustDecode(t, conversation, &createdConversation)
	if createdConversation.HasRunningRun {
		t.Fatalf("new conversation should not start active: %+v", createdConversation)
	}

	response := request(server, http.MethodPost, "/api/workspaces/"+ws.ID+"/conversations/"+createdConversation.ID+"/messages", `{"content":"fix bug","model":"gpt-5.5","reasoningEffort":"medium"}`)
	if response.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", response.Code, response.Body.String())
	}
	var created struct {
		Message messageResponse `json:"message"`
		Run     agent.Run       `json:"run"`
	}
	mustDecode(t, response, &created)
	if !strings.HasPrefix(created.Run.ID, "run_") || created.Run.Model != "gpt-5.5" || created.Run.ReasoningEffort != "medium" {
		t.Fatalf("unexpected run: %+v", created.Run)
	}

	detail := waitForConversationDetail(t, server, ws.ID, createdConversation.ID, created.Run.ID, "waiting_tool_approval")
	if !detail.Conversation.HasRunningRun {
		t.Fatalf("expected detail conversation to show active run, got %+v", detail.Conversation)
	}
	if len(detail.ToolCalls) != 1 || detail.ToolCalls[0].Name != "apply_patch" {
		t.Fatalf("expected one patch tool call, got %+v", detail.ToolCalls)
	}

	list := request(server, http.MethodGet, "/api/workspaces/"+ws.ID+"/conversations", "")
	if list.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", list.Code, list.Body.String())
	}
	var listBody struct {
		Conversations []conversationResponse `json:"conversations"`
	}
	mustDecode(t, list, &listBody)
	if len(listBody.Conversations) != 1 || listBody.Conversations[0].ID != createdConversation.ID {
		t.Fatalf("unexpected conversation list: %+v", listBody.Conversations)
	}
	if !listBody.Conversations[0].HasRunningRun {
		t.Fatalf("expected conversation list to show active run, got %+v", listBody.Conversations[0])
	}
}

func TestConversationTitleGenerationPublishesUpdateEvent(t *testing.T) {
	root := initGitRepo(t, t.TempDir())
	provider := titleAgentProvider{
		models:  make(chan string, 1),
		prompts: make(chan string, 1),
		release: make(chan struct{}),
		title:   "Investigate flaky tests",
	}
	fixture := newServerFixture(t, root, provider)
	fixture.server.SetLightModel("gpt-light")
	handler := fixture.server.Routes()
	events, unsubscribe := fixture.hub.Subscribe()
	defer unsubscribe()

	create := request(handler, http.MethodPost, "/api/workspaces", `{"rootPath":"`+root+`"}`)
	var ws workspace.Workspace
	mustDecode(t, create, &ws)
	conversation := request(handler, http.MethodPost, "/api/workspaces/"+ws.ID+"/conversations", `{}`)
	if conversation.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", conversation.Code, conversation.Body.String())
	}
	var createdConversation conversationResponse
	mustDecode(t, conversation, &createdConversation)
	if createdConversation.Title != defaultConversationTitle {
		t.Fatalf("expected default title, got %+v", createdConversation)
	}

	response := request(handler, http.MethodPost, "/api/workspaces/"+ws.ID+"/conversations/"+createdConversation.ID+"/messages", `{"content":"fix flaky tests","model":"gpt-5.5","reasoningEffort":"medium"}`)
	if response.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", response.Code, response.Body.String())
	}
	var created struct {
		Conversation conversationResponse `json:"conversation"`
		Message      messageResponse      `json:"message"`
		Run          agent.Run            `json:"run"`
	}
	mustDecode(t, response, &created)
	if created.Conversation.Title != defaultConversationTitle {
		t.Fatalf("expected create message response to keep placeholder title, got %+v", created.Conversation)
	}

	if got := receiveString(t, provider.models); got != "gpt-light" {
		t.Fatalf("expected light model, got %q", got)
	}
	if got := receiveString(t, provider.prompts); got != "fix flaky tests" {
		t.Fatalf("expected first prompt, got %q", got)
	}
	close(provider.release)

	event := waitForEvent(t, events, "conversation.updated")
	updated, ok := event.Payload.(conversationResponse)
	if !ok {
		t.Fatalf("expected conversation payload, got %T", event.Payload)
	}
	if updated.Title != "Investigate flaky tests" || updated.ID != createdConversation.ID {
		t.Fatalf("unexpected updated conversation: %+v", updated)
	}
	detail := getConversationDetail(t, handler, ws.ID, createdConversation.ID)
	if detail.Conversation.Title != "Investigate flaky tests" {
		t.Fatalf("expected generated title in detail, got %+v", detail.Conversation)
	}
}

func TestConversationTitleGenerationDoesNotOverwriteManualTitle(t *testing.T) {
	root := initGitRepo(t, t.TempDir())
	provider := titleAgentProvider{
		models:  make(chan string, 1),
		release: make(chan struct{}),
		title:   "Generated title",
	}
	fixture := newServerFixture(t, root, provider)
	handler := fixture.server.Routes()

	create := request(handler, http.MethodPost, "/api/workspaces", `{"rootPath":"`+root+`"}`)
	var ws workspace.Workspace
	mustDecode(t, create, &ws)
	conversation := request(handler, http.MethodPost, "/api/workspaces/"+ws.ID+"/conversations", `{}`)
	var createdConversation conversationResponse
	mustDecode(t, conversation, &createdConversation)
	response := request(handler, http.MethodPost, "/api/workspaces/"+ws.ID+"/conversations/"+createdConversation.ID+"/messages", `{"content":"fix flaky tests","model":"gpt-5.5","reasoningEffort":"medium"}`)
	if response.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", response.Code, response.Body.String())
	}
	_ = receiveString(t, provider.models)

	rename := request(handler, http.MethodPatch, "/api/workspaces/"+ws.ID+"/conversations/"+createdConversation.ID, `{"title":"Manual title"}`)
	if rename.Code != http.StatusOK {
		t.Fatalf("expected rename 200, got %d: %s", rename.Code, rename.Body.String())
	}
	close(provider.release)
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		detail := getConversationDetail(t, handler, ws.ID, createdConversation.ID)
		if detail.Conversation.Title == "Generated title" {
			t.Fatalf("generated title overwrote manual title: %+v", detail.Conversation)
		}
		if detail.Conversation.Title == "Manual title" {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	detail := getConversationDetail(t, handler, ws.ID, createdConversation.ID)
	if detail.Conversation.Title != "Manual title" {
		t.Fatalf("expected manual title, got %+v", detail.Conversation)
	}
}

func TestConversationTitleGenerationFailureDoesNotFailMessageCreate(t *testing.T) {
	root := initGitRepo(t, t.TempDir())
	provider := titleAgentProvider{
		err:    errors.New("title failed"),
		models: make(chan string, 1),
	}
	fixture := newServerFixture(t, root, provider)
	handler := fixture.server.Routes()

	create := request(handler, http.MethodPost, "/api/workspaces", `{"rootPath":"`+root+`"}`)
	var ws workspace.Workspace
	mustDecode(t, create, &ws)
	conversation := request(handler, http.MethodPost, "/api/workspaces/"+ws.ID+"/conversations", `{}`)
	var createdConversation conversationResponse
	mustDecode(t, conversation, &createdConversation)
	response := request(handler, http.MethodPost, "/api/workspaces/"+ws.ID+"/conversations/"+createdConversation.ID+"/messages", `{"content":"fix flaky tests","model":"gpt-5.5","reasoningEffort":"medium"}`)
	if response.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", response.Code, response.Body.String())
	}
	_ = receiveString(t, provider.models)
	detail := getConversationDetail(t, handler, ws.ID, createdConversation.ID)
	if detail.Conversation.Title != defaultConversationTitle {
		t.Fatalf("expected placeholder title after title failure, got %+v", detail.Conversation)
	}
}

func TestConversationListSearchesTitlesAndPaginates(t *testing.T) {
	root := initGitRepo(t, t.TempDir())
	fixture := newServerFixture(t, root, fakeAgentProvider{})
	handler := fixture.server.Routes()
	create := request(handler, http.MethodPost, "/api/workspaces", `{"rootPath":"`+root+`"}`)
	var ws workspace.Workspace
	mustDecode(t, create, &ws)

	baseTime := time.Date(2026, 5, 20, 10, 0, 0, 0, time.UTC)
	for i, title := range []string{
		"Fix Search Modal",
		"Release notes",
		"search sidebar polish",
		"Patch approval flow",
	} {
		if _, err := fixture.store.CreateConversation(context.Background(), database.ConversationRecord{
			WorkspaceID:   ws.ID,
			Title:         title,
			CreatedAt:     baseTime.Add(time.Duration(i) * time.Second),
			UpdatedAt:     baseTime.Add(time.Duration(i) * time.Second),
			LastMessageAt: baseTime.Add(time.Duration(i) * time.Second),
		}); err != nil {
			t.Fatalf("CreateConversation %q returned error: %v", title, err)
		}
	}

	firstPage := request(handler, http.MethodGet, "/api/workspaces/"+ws.ID+"/conversations?q=%20SEARCH%20&limit=1", "")
	if firstPage.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", firstPage.Code, firstPage.Body.String())
	}
	var firstBody struct {
		Conversations []conversationResponse `json:"conversations"`
		NextCursor    *string                `json:"nextCursor"`
	}
	mustDecode(t, firstPage, &firstBody)
	if len(firstBody.Conversations) != 1 || firstBody.Conversations[0].Title != "search sidebar polish" || firstBody.NextCursor == nil {
		t.Fatalf("unexpected first search page: %+v", firstBody)
	}

	nextPage := request(handler, http.MethodGet, "/api/workspaces/"+ws.ID+"/conversations?q=search&limit=1&cursor="+*firstBody.NextCursor, "")
	if nextPage.Code != http.StatusOK {
		t.Fatalf("expected next page 200, got %d: %s", nextPage.Code, nextPage.Body.String())
	}
	var nextBody struct {
		Conversations []conversationResponse `json:"conversations"`
		NextCursor    *string                `json:"nextCursor"`
	}
	mustDecode(t, nextPage, &nextBody)
	if len(nextBody.Conversations) != 1 || nextBody.Conversations[0].Title != "Fix Search Modal" || nextBody.NextCursor != nil {
		t.Fatalf("unexpected next search page: %+v", nextBody)
	}
}

func TestToolApprovalAppliesPatchAndRemovedPatchEndpointsAreGone(t *testing.T) {
	root := initGitRepo(t, t.TempDir())
	seedExampleFile(t, root)
	server := newTestServer(t, root)
	create := request(server, http.MethodPost, "/api/workspaces", `{"rootPath":"`+root+`"}`)
	var ws workspace.Workspace
	mustDecode(t, create, &ws)

	conversation := request(server, http.MethodPost, "/api/workspaces/"+ws.ID+"/conversations", `{"title":"Fix bug"}`)
	var createdConversation conversationResponse
	mustDecode(t, conversation, &createdConversation)

	response := request(server, http.MethodPost, "/api/workspaces/"+ws.ID+"/conversations/"+createdConversation.ID+"/messages", `{"content":"fix bug","model":"gpt-5.5","reasoningEffort":"medium"}`)
	if response.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", response.Code, response.Body.String())
	}
	var created struct {
		Message messageResponse `json:"message"`
		Run     agent.Run       `json:"run"`
	}
	mustDecode(t, response, &created)
	detail := waitForConversationDetail(t, server, ws.ID, createdConversation.ID, created.Run.ID, "waiting_tool_approval")
	toolCallID := detail.ToolCalls[0].ID

	oldPatchRoute := request(server, http.MethodPost, "/api/workspaces/"+ws.ID+"/patches/patch_1/apply", "")
	if oldPatchRoute.Code != http.StatusNotFound {
		t.Fatalf("expected old patch route 404, got %d: %s", oldPatchRoute.Code, oldPatchRoute.Body.String())
	}

	apply := request(server, http.MethodPost, "/api/workspaces/"+ws.ID+"/conversations/"+createdConversation.ID+"/runs/"+created.Run.ID+"/tool-calls/"+toolCallID+"/approve", "")
	if apply.Code != http.StatusOK {
		t.Fatalf("expected approve 200, got %d: %s\nfile:%q", apply.Code, apply.Body.String(), mustReadFile(t, filepath.Join(root, "example.txt")))
	}
	var applyBody struct {
		ToolCall agent.ToolCall `json:"toolCall"`
	}
	mustDecode(t, apply, &applyBody)
	if applyBody.ToolCall.ID != toolCallID || applyBody.ToolCall.Decision == nil || *applyBody.ToolCall.Decision != "approved" {
		t.Fatalf("unexpected approve body: %+v", applyBody)
	}
	detail = waitForConversationDetail(t, server, ws.ID, createdConversation.ID, created.Run.ID, "done")
	detail = waitForConversationActiveState(t, server, ws.ID, createdConversation.ID, false)
	if detail.Conversation.HasRunningRun {
		t.Fatalf("expected completed conversation to clear active flag, got %+v", detail.Conversation)
	}
	if detail.ToolCalls[0].Status != "finished" {
		t.Fatalf("expected finished tool call, got %+v", detail.ToolCalls[0])
	}
	if len(detail.Events) != 0 {
		t.Fatalf("finished runs should not replay historical events, got %+v", detail.Events)
	}
	if !hasMessage(detail.Messages, "assistant", "Fake provider completed.") {
		t.Fatalf("expected assistant message in conversation detail, got %+v", detail.Messages)
	}
	if got := mustReadFile(t, filepath.Join(root, "example.txt")); got != "after\n" {
		t.Fatalf("expected approved patch to apply, got %q", got)
	}
}

func TestWorkspaceEventsDoesNotReplayHistoricalEvents(t *testing.T) {
	root := initGitRepo(t, t.TempDir())
	seedExampleFile(t, root)
	server := newTestServer(t, root)
	create := request(server, http.MethodPost, "/api/workspaces", `{"rootPath":"`+root+`"}`)
	var ws workspace.Workspace
	mustDecode(t, create, &ws)

	conversation := request(server, http.MethodPost, "/api/workspaces/"+ws.ID+"/conversations", `{"title":"Fix bug"}`)
	var createdConversation conversationResponse
	mustDecode(t, conversation, &createdConversation)

	response := request(server, http.MethodPost, "/api/workspaces/"+ws.ID+"/conversations/"+createdConversation.ID+"/messages", `{"content":"fix bug","model":"gpt-5.5","reasoningEffort":"medium"}`)
	if response.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", response.Code, response.Body.String())
	}
	var created struct {
		Message messageResponse `json:"message"`
		Run     agent.Run       `json:"run"`
	}
	mustDecode(t, response, &created)
	detail := waitForConversationDetail(t, server, ws.ID, createdConversation.ID, created.Run.ID, "waiting_tool_approval")
	if len(detail.Events) == 0 {
		t.Fatal("test setup expected stored run events")
	}

	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/api/workspaces/"+ws.ID+"/events", nil).WithContext(ctx)
	recorder := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		server.ServeHTTP(recorder, req)
		close(done)
	}()
	time.Sleep(20 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("events stream did not close after request cancellation")
	}
	if body := recorder.Body.String(); body != "" {
		t.Fatalf("expected no historical SSE replay, got %q", body)
	}
}

func TestAgentRunEventsReplayDurableEventsOnly(t *testing.T) {
	root := initGitRepo(t, t.TempDir())
	seedExampleFile(t, root)
	server := newTestServer(t, root)
	create := request(server, http.MethodPost, "/api/workspaces", `{"rootPath":"`+root+`"}`)
	var ws workspace.Workspace
	mustDecode(t, create, &ws)

	conversation := request(server, http.MethodPost, "/api/workspaces/"+ws.ID+"/conversations", `{"title":"Fix bug"}`)
	var createdConversation conversationResponse
	mustDecode(t, conversation, &createdConversation)

	response := request(server, http.MethodPost, "/api/workspaces/"+ws.ID+"/conversations/"+createdConversation.ID+"/messages", `{"content":"fix bug","model":"gpt-5.5","reasoningEffort":"medium"}`)
	if response.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", response.Code, response.Body.String())
	}
	var created struct {
		Message messageResponse `json:"message"`
		Run     agent.Run       `json:"run"`
	}
	mustDecode(t, response, &created)
	waitForConversationDetail(t, server, ws.ID, createdConversation.ID, created.Run.ID, "waiting_tool_approval")

	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/api/workspaces/"+ws.ID+"/conversations/"+createdConversation.ID+"/runs/"+created.Run.ID+"/events", nil).WithContext(ctx)
	recorder := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		server.ServeHTTP(recorder, req)
		close(done)
	}()
	time.Sleep(20 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("run events stream did not close after request cancellation")
	}
	body := recorder.Body.String()
	if !strings.Contains(body, "agent.approval_required") || !strings.Contains(body, "agent.run.status_changed") {
		t.Fatalf("expected durable run events replay, got %q", body)
	}
	if strings.Contains(body, "event: agent.delta") {
		t.Fatalf("expected transient deltas to be excluded from replay, got %q", body)
	}
}

func TestAgentRunEventsEmitsTransientSnapshot(t *testing.T) {
	root := initGitRepo(t, t.TempDir())
	store, err := database.Open(filepath.Join(t.TempDir(), "patchpilot.db"))
	if err != nil {
		t.Fatalf("database.Open returned error: %v", err)
	}
	defer store.Close()
	manager, err := workspace.NewManager([]string{root}, store, gitrepo.NewClient())
	if err != nil {
		t.Fatalf("NewManager returned error: %v", err)
	}
	fileService := filestore.NewService()
	gitClient := gitrepo.NewClient()
	run := runner.NewRunner()
	hub := events.NewHub()
	agentManager := agent.NewManager(store, fileService, gitClient, run, hub, blockingAgentProvider{delta: "draft text"})
	server := NewServer(manager, fileService, gitClient, run, store, hub, agentManager, store).Routes()

	create := request(server, http.MethodPost, "/api/workspaces", `{"rootPath":"`+root+`"}`)
	var ws workspace.Workspace
	mustDecode(t, create, &ws)
	conversation := request(server, http.MethodPost, "/api/workspaces/"+ws.ID+"/conversations", `{"title":"Fix bug"}`)
	var createdConversation conversationResponse
	mustDecode(t, conversation, &createdConversation)
	response := request(server, http.MethodPost, "/api/workspaces/"+ws.ID+"/conversations/"+createdConversation.ID+"/messages", `{"content":"fix bug","model":"gpt-5.5","reasoningEffort":"medium"}`)
	var created struct {
		Message messageResponse `json:"message"`
		Run     agent.Run       `json:"run"`
	}
	mustDecode(t, response, &created)
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if agentManager.DraftText(created.Run.ID) == "draft text" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if got := agentManager.DraftText(created.Run.ID); got != "draft text" {
		t.Fatalf("expected draft text, got %q", got)
	}

	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/api/workspaces/"+ws.ID+"/conversations/"+createdConversation.ID+"/runs/"+created.Run.ID+"/events", nil).WithContext(ctx)
	recorder := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		server.ServeHTTP(recorder, req)
		close(done)
	}()
	time.Sleep(20 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("run events stream did not close after request cancellation")
	}
	body := recorder.Body.String()
	if !strings.Contains(body, "agent.output.snapshot") || !strings.Contains(body, "draft text") {
		t.Fatalf("expected transient snapshot, got %q", body)
	}
	if strings.Contains(body, "event: agent.delta") {
		t.Fatalf("expected replay to exclude agent.delta, got %q", body)
	}
	_ = request(server, http.MethodPost, "/api/workspaces/"+ws.ID+"/conversations/"+createdConversation.ID+"/runs/"+created.Run.ID+"/cancel", "")
}

func TestCancelAgentRunMarksCanceled(t *testing.T) {
	root := initGitRepo(t, t.TempDir())
	seedExampleFile(t, root)
	server := newTestServer(t, root)
	create := request(server, http.MethodPost, "/api/workspaces", `{"rootPath":"`+root+`"}`)
	var ws workspace.Workspace
	mustDecode(t, create, &ws)

	conversation := request(server, http.MethodPost, "/api/workspaces/"+ws.ID+"/conversations", `{"title":"Fix bug"}`)
	var createdConversation conversationResponse
	mustDecode(t, conversation, &createdConversation)

	response := request(server, http.MethodPost, "/api/workspaces/"+ws.ID+"/conversations/"+createdConversation.ID+"/messages", `{"content":"fix bug","model":"gpt-5.5","reasoningEffort":"medium"}`)
	var created struct {
		Message messageResponse `json:"message"`
		Run     agent.Run       `json:"run"`
	}
	mustDecode(t, response, &created)
	waitForConversationDetail(t, server, ws.ID, createdConversation.ID, created.Run.ID, "waiting_tool_approval")

	cancel := request(server, http.MethodPost, "/api/workspaces/"+ws.ID+"/conversations/"+createdConversation.ID+"/runs/"+created.Run.ID+"/cancel", "")
	if cancel.Code != http.StatusOK {
		t.Fatalf("expected cancel 200, got %d: %s", cancel.Code, cancel.Body.String())
	}
	var canceled agent.Run
	mustDecode(t, cancel, &canceled)
	if canceled.Status != "canceled" {
		t.Fatalf("expected canceled status, got %+v", canceled)
	}
}

func TestServerShutdownFailsActiveRunsAndIsIdempotent(t *testing.T) {
	root := initGitRepo(t, t.TempDir())
	seedExampleFile(t, root)
	fixture := newServerFixture(t, root, fakeAgentProvider{})

	conversation, err := fixture.store.CreateConversation(context.Background(), database.ConversationRecord{
		ID:          "conv_1",
		WorkspaceID: "ws_1",
		Title:       "Tracked",
	})
	if err != nil {
		t.Fatalf("CreateConversation returned error: %v", err)
	}
	trigger, err := fixture.store.CreateMessage(context.Background(), database.MessageRecord{
		ID:             "msg_1",
		WorkspaceID:    "ws_1",
		ConversationID: conversation.ID,
		Role:           "user",
		Content:        "fix bug",
	})
	if err != nil {
		t.Fatalf("CreateMessage returned error: %v", err)
	}
	run, err := fixture.agent.Create(context.Background(), "ws_1", root, agent.CreateRunInput{
		Prompt:           trigger.Content,
		ConversationID:   conversation.ID,
		TriggerMessageID: trigger.ID,
		Model:            "gpt-5.5",
		ReasoningEffort:  "medium",
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	waitForAgentStatus(t, fixture.agent, "ws_1", conversation.ID, run.ID, "waiting_tool_approval")

	if err := fixture.server.Shutdown(context.Background(), "backend shutdown"); err != nil {
		t.Fatalf("first Shutdown returned error: %v", err)
	}
	if err := fixture.server.Shutdown(context.Background(), "backend shutdown"); err != nil {
		t.Fatalf("second Shutdown returned error: %v", err)
	}

	detail := waitForAgentStatus(t, fixture.agent, "ws_1", conversation.ID, run.ID, "failed")
	if detail.Run.Error == nil || *detail.Run.Error != "backend shutdown" {
		t.Fatalf("expected shutdown failure message, got %+v", detail.Run)
	}
	loadedConversation, err := fixture.store.GetConversation(context.Background(), "ws_1", conversation.ID)
	if err != nil {
		t.Fatalf("GetConversation returned error: %v", err)
	}
	if loadedConversation.HasRunningRun {
		t.Fatalf("expected shutdown to clear active flag, got %+v", loadedConversation)
	}
}
