package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/phamtanminhtien/patchpilot/internal/agent"
	"github.com/phamtanminhtien/patchpilot/internal/auth"
	"github.com/phamtanminhtien/patchpilot/internal/database"
	"github.com/phamtanminhtien/patchpilot/internal/events"
	"github.com/phamtanminhtien/patchpilot/internal/filestore"
	"github.com/phamtanminhtien/patchpilot/internal/gitrepo"
	"github.com/phamtanminhtien/patchpilot/internal/runner"
	"github.com/phamtanminhtien/patchpilot/internal/workspace"
)

type fakeHealthChecker struct {
	err error
}

func (f fakeHealthChecker) Ping(context.Context) error {
	return f.err
}

type fakeAgentProvider struct{}

func (fakeAgentProvider) Configured() bool {
	return true
}

type unavailableAgentProvider struct{}

func (unavailableAgentProvider) Configured() bool {
	return false
}

func (unavailableAgentProvider) Generate(context.Context, agent.ProviderRequest, agent.Stream) (agent.ProviderResult, error) {
	return agent.ProviderResult{}, agent.ErrProviderUnavailable
}

func (unavailableAgentProvider) Summarize(context.Context, agent.SummaryRequest) (string, error) {
	return "", agent.ErrProviderUnavailable
}

func (fakeAgentProvider) Generate(ctx context.Context, request agent.ProviderRequest, stream agent.Stream) (agent.ProviderResult, error) {
	stream.Delta(ctx, "fake provider response")
	if len(request.History) > 0 {
		return agent.ProviderResult{Text: "Fake provider completed.", Done: true}, nil
	}
	return agent.ProviderResult{ToolCalls: []agent.ToolRequest{{
		CallID:    "call_patch",
		Name:      "apply_patch",
		Arguments: `{"summary":"update example","diff":"diff --git a/example.txt b/example.txt\nindex 7c8e5d0..ef49dd8 100644\n--- a/example.txt\n+++ b/example.txt\n@@ -1 +1 @@\n-before\n+after\n"}`,
	}}}, nil
}

func (fakeAgentProvider) Summarize(context.Context, agent.SummaryRequest) (string, error) {
	return "fake summary", nil
}

type titleAgentProvider struct {
	fakeAgentProvider
	err     error
	models  chan string
	prompts chan string
	release chan struct{}
	title   string
}

func (p titleAgentProvider) GenerateTitle(_ context.Context, prompt, model string) (string, error) {
	if p.models != nil {
		p.models <- model
	}
	if p.prompts != nil {
		p.prompts <- prompt
	}
	if p.release != nil {
		<-p.release
	}
	if p.err != nil {
		return "", p.err
	}
	if p.title != "" {
		return p.title, nil
	}
	return "Generated title", nil
}

type blockingAgentProvider struct {
	delta string
	done  chan struct{}
}

func (p blockingAgentProvider) Configured() bool {
	return true
}

func (p blockingAgentProvider) Generate(ctx context.Context, request agent.ProviderRequest, stream agent.Stream) (agent.ProviderResult, error) {
	stream.Delta(ctx, p.delta)
	if p.done != nil {
		close(p.done)
	}
	<-ctx.Done()
	return agent.ProviderResult{}, ctx.Err()
}

func (p blockingAgentProvider) Summarize(context.Context, agent.SummaryRequest) (string, error) {
	return "fake summary", nil
}

func newTestServer(t *testing.T, allowedRoot string) http.Handler {
	t.Helper()
	return newTestServerWithHealth(t, allowedRoot, fakeHealthChecker{})
}

type serverFixture struct {
	server *Server
	store  *database.Store
	hub    *events.Hub
	agent  *agent.Manager
	runner *runner.Runner
}

func newTestServerWithHealth(t *testing.T, allowedRoot string, health HealthChecker) http.Handler {
	t.Helper()
	return newTestServerWithDBPath(t, allowedRoot, filepath.Join(t.TempDir(), "patchpilot.db"), health)
}

func newTestServerWithDBPath(t *testing.T, allowedRoot string, dbPath string, health HealthChecker) http.Handler {
	return newTestServerWithAgentProvider(t, allowedRoot, dbPath, health, fakeAgentProvider{})
}

func newTestServerWithAgentProvider(t *testing.T, allowedRoot string, dbPath string, health HealthChecker, provider agent.Provider) http.Handler {
	t.Helper()
	store, err := database.Open(dbPath)
	if err != nil {
		t.Fatalf("database.Open returned error: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	})
	manager, err := workspace.NewManager([]string{allowedRoot}, store, gitrepo.NewClient())
	if err != nil {
		t.Fatalf("NewManager returned error: %v", err)
	}
	if health == nil {
		health = store
	}
	fileService := filestore.NewService()
	gitClient := gitrepo.NewClient()
	run := runner.NewRunner()
	hub := events.NewHub()
	agentManager := agent.NewManager(store, fileService, gitClient, run, hub, provider)
	t.Cleanup(func() {
		if err := agentManager.Shutdown(context.Background(), "test cleanup"); err != nil {
			t.Fatalf("agent Shutdown returned error: %v", err)
		}
	})
	return NewServer(manager, fileService, gitClient, run, store, hub, agentManager, health).Routes()
}

func newServerFixture(t *testing.T, allowedRoot string, provider agent.Provider) serverFixture {
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
	manager, err := workspace.NewManager([]string{allowedRoot}, store, gitrepo.NewClient())
	if err != nil {
		t.Fatalf("NewManager returned error: %v", err)
	}
	fileService := filestore.NewService()
	gitClient := gitrepo.NewClient()
	run := runner.NewRunner()
	hub := events.NewHub()
	agentManager := agent.NewManager(store, fileService, gitClient, run, hub, provider)
	t.Cleanup(func() {
		if err := agentManager.Shutdown(context.Background(), "test cleanup"); err != nil {
			t.Fatalf("agent Shutdown returned error: %v", err)
		}
	})
	return serverFixture{
		server: NewServer(manager, fileService, gitClient, run, store, hub, agentManager, store),
		store:  store,
		hub:    hub,
		agent:  agentManager,
		runner: run,
	}
}

func newAuthenticatedTestServer(t *testing.T, allowedRoot, adminToken string) http.Handler {
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
	gitClient := gitrepo.NewClient()
	manager, err := workspace.NewManager([]string{allowedRoot}, store, gitClient)
	if err != nil {
		t.Fatalf("NewManager returned error: %v", err)
	}
	fileService := filestore.NewService()
	run := runner.NewRunner()
	hub := events.NewHub()
	agentManager := agent.NewManager(store, fileService, gitClient, run, hub, fakeAgentProvider{})
	t.Cleanup(func() {
		if err := agentManager.Shutdown(context.Background(), "test cleanup"); err != nil {
			t.Fatalf("agent Shutdown returned error: %v", err)
		}
	})
	authService, err := auth.NewService(adminToken, store)
	if err != nil {
		t.Fatalf("NewService returned error: %v", err)
	}
	return NewServerWithAuth(manager, fileService, gitClient, run, store, hub, agentManager, authService, store).Routes()
}

type conversationDetailResponse struct {
	Conversation conversationResponse `json:"conversation"`
	Events       []agent.RunEvent     `json:"events"`
	Messages     []messageResponse    `json:"messages"`
	Runs         []agent.Run          `json:"runs"`
	ToolCalls    []agent.ToolCall     `json:"toolCalls"`
}

func waitForConversationDetail(t *testing.T, handler http.Handler, workspaceID, conversationID, runID, status string) conversationDetailResponse {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		response := request(handler, http.MethodGet, "/api/workspaces/"+workspaceID+"/conversations/"+conversationID, "")
		if response.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
		}
		var detail conversationDetailResponse
		mustDecode(t, response, &detail)
		for _, run := range detail.Runs {
			if run.ID == runID && run.Status == status {
				return detail
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("run did not reach %s", status)
	return conversationDetailResponse{}
}

func waitForConversationActiveState(t *testing.T, handler http.Handler, workspaceID, conversationID string, hasRunningRun bool) conversationDetailResponse {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		response := request(handler, http.MethodGet, "/api/workspaces/"+workspaceID+"/conversations/"+conversationID, "")
		if response.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
		}
		var detail conversationDetailResponse
		mustDecode(t, response, &detail)
		if detail.Conversation.HasRunningRun == hasRunningRun {
			return detail
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("conversation active state did not become %v", hasRunningRun)
	return conversationDetailResponse{}
}

func receiveString(t *testing.T, ch <-chan string) string {
	t.Helper()
	select {
	case value := <-ch:
		return value
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for string")
		return ""
	}
}

func waitForEvent(t *testing.T, eventsCh <-chan events.Event, eventType string) events.Event {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for {
		select {
		case event := <-eventsCh:
			if event.Type == eventType {
				return event
			}
		case <-deadline:
			t.Fatalf("timed out waiting for event %s", eventType)
			return events.Event{}
		}
	}
}

func waitForAgentStatus(t *testing.T, manager *agent.Manager, workspaceID, conversationID, runID, status string) agent.Detail {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		detail, err := manager.Detail(context.Background(), workspaceID, conversationID, runID)
		if err != nil {
			t.Fatalf("Detail returned error: %v", err)
		}
		if detail.Run.Status == status {
			return detail
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("run did not reach %s", status)
	return agent.Detail{}
}

func waitForCommandStatus(t *testing.T, store *database.Store, workspaceID, commandID, status string) database.CommandRecord {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		command, err := store.GetCommand(context.Background(), workspaceID, commandID)
		if err != nil {
			t.Fatalf("GetCommand returned error: %v", err)
		}
		if command.Status == status {
			return command
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("command did not reach %s", status)
	return database.CommandRecord{}
}

func receiveProcessExitedEvents(t *testing.T, eventsCh <-chan events.Event, count int) []events.Event {
	t.Helper()
	found := make([]events.Event, 0, count)
	deadline := time.After(2 * time.Second)
	for len(found) < count {
		select {
		case event := <-eventsCh:
			if event.Type == "process.exited" {
				found = append(found, event)
			}
		case <-deadline:
			t.Fatalf("timed out waiting for %d process.exited events, got %+v", count, found)
		}
	}
	return found
}

func getConversationDetail(t *testing.T, handler http.Handler, workspaceID, conversationID string) conversationDetailResponse {
	t.Helper()
	response := request(handler, http.MethodGet, "/api/workspaces/"+workspaceID+"/conversations/"+conversationID, "")
	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}
	var detail conversationDetailResponse
	mustDecode(t, response, &detail)
	return detail
}

func request(handler http.Handler, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	return recorder
}

func requestWithCookies(handler http.Handler, method, path, body string, cookies []*http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	return recorder
}

func mustDecode(t *testing.T, response *httptest.ResponseRecorder, target any) {
	t.Helper()
	if err := json.NewDecoder(response.Body).Decode(target); err != nil {
		t.Fatalf("decode response: %v", err)
	}
}

func hasMessage(messages []messageResponse, role, content string) bool {
	for _, message := range messages {
		if message.Role == role && message.Content == content {
			return true
		}
	}
	return false
}

func mustMkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

func initGitRepo(t *testing.T, root string) string {
	t.Helper()
	mustMkdirAll(t, root)
	run(t, root, "git", "init")
	return root
}

func configureCommitter(t *testing.T, root string) {
	t.Helper()
	run(t, root, "git", "config", "user.email", "test@example.com")
	run(t, root, "git", "config", "user.name", "Test")
}

func seedExampleFile(t *testing.T, root string) {
	t.Helper()
	configureCommitter(t, root)
	if err := os.WriteFile(filepath.Join(root, "example.txt"), []byte("before\n"), 0o644); err != nil {
		t.Fatalf("write example file: %v", err)
	}
	run(t, root, "git", "add", "example.txt")
	run(t, root, "git", "commit", "-m", "seed example")
}

func mustReadFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	return string(content)
}

func run(t *testing.T, dir, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v failed: %v\n%s", name, args, err, output)
	}
}
