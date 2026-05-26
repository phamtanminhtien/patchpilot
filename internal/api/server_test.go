package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
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

func TestExposedURLUsesBackendPort(t *testing.T) {
	server := &Server{}
	server.SetBackendAddr("127.0.0.1:8080")
	request := httptest.NewRequest(http.MethodPost, "http://localhost:5173/api/workspaces/ws_1/ports/3000/expose", nil)

	got := server.exposedURL(request, "/workspaces/ws_1/ports/3000/proxy/")
	want := "http://127.0.0.1:8080/workspaces/ws_1/ports/3000/proxy/"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestExposedURLKeepsRequestHostForWildcardBackendAddr(t *testing.T) {
	server := &Server{}
	server.SetBackendAddr("0.0.0.0:8080")
	request := httptest.NewRequest(http.MethodPost, "http://dev.example.test:5173/api/workspaces/ws_1/ports/3000/expose", nil)

	got := server.exposedURL(request, "/workspaces/ws_1/ports/3000/proxy/")
	want := "http://dev.example.test:8080/workspaces/ws_1/ports/3000/proxy/"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestPortHandlersListExposeAndMarkClosed(t *testing.T) {
	root := initGitRepo(t, t.TempDir())
	fixture := newServerFixture(t, root, fakeAgentProvider{})
	handler := fixture.server.Routes()
	create := request(handler, http.MethodPost, "/api/workspaces", `{"rootPath":"`+root+`"}`)
	var ws workspace.Workspace
	mustDecode(t, create, &ws)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()
	port := listener.Addr().(*net.TCPAddr).Port
	processID := "cmd_1"
	if _, _, err := fixture.store.UpsertDetectedPort(context.Background(), database.PortRecord{
		WorkspaceID: ws.ID,
		ProcessID:   &processID,
		Port:        port,
		Status:      "detected",
	}); err != nil {
		t.Fatalf("UpsertDetectedPort returned error: %v", err)
	}

	list := request(handler, http.MethodGet, "/api/workspaces/"+ws.ID+"/ports", "")
	if list.Code != http.StatusOK {
		t.Fatalf("expected list 200, got %d: %s", list.Code, list.Body.String())
	}
	var listBody struct {
		Ports []portResponse `json:"ports"`
	}
	mustDecode(t, list, &listBody)
	if len(listBody.Ports) != 1 || listBody.Ports[0].Port != port || listBody.Ports[0].Status != "detected" {
		t.Fatalf("unexpected port list: %+v", listBody.Ports)
	}

	expose := request(handler, http.MethodPost, "/api/workspaces/"+ws.ID+"/ports/"+listener.Addr().(*net.TCPAddr).String()+"/expose", "")
	if expose.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid port 400, got %d: %s", expose.Code, expose.Body.String())
	}

	expose = request(handler, http.MethodPost, "/api/workspaces/"+ws.ID+"/ports/"+strconv.Itoa(port)+"/expose", "")
	if expose.Code != http.StatusOK {
		t.Fatalf("expected expose 200, got %d: %s", expose.Code, expose.Body.String())
	}
	var exposeBody struct {
		Port portResponse `json:"port"`
	}
	mustDecode(t, expose, &exposeBody)
	if exposeBody.Port.Status != "exposed" || exposeBody.Port.ExposedURL == nil || !strings.Contains(*exposeBody.Port.ExposedURL, "/proxy/") {
		t.Fatalf("unexpected exposed port body: %+v", exposeBody)
	}

	if err := listener.Close(); err != nil {
		t.Fatalf("close listener: %v", err)
	}
	closed := request(handler, http.MethodPost, "/api/workspaces/"+ws.ID+"/ports/"+strconv.Itoa(port)+"/expose", "")
	if closed.Code != http.StatusBadGateway {
		t.Fatalf("expected closed port 502, got %d: %s", closed.Code, closed.Body.String())
	}
	var closedBody map[string]map[string]any
	mustDecode(t, closed, &closedBody)
	if closedBody["error"]["code"] != "port_unreachable" {
		t.Fatalf("unexpected closed port body: %+v", closedBody)
	}
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

func TestHealthCheckReturnsOK(t *testing.T) {
	server := newTestServer(t, t.TempDir())

	response := request(server, http.MethodGet, "/api/health", "")
	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}
	var body map[string]string
	mustDecode(t, response, &body)
	if body["status"] != "ok" {
		t.Fatalf("unexpected health body: %+v", body)
	}
}

func TestHealthCheckReturnsUnavailableWhenDatabaseFails(t *testing.T) {
	server := newTestServerWithHealth(t, t.TempDir(), fakeHealthChecker{err: errors.New("down")})

	response := request(server, http.MethodGet, "/api/health", "")
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d: %s", response.Code, response.Body.String())
	}
	var body map[string]map[string]any
	mustDecode(t, response, &body)
	if body["error"]["code"] != "database_unavailable" {
		t.Fatalf("unexpected error body: %+v", body)
	}
}

func TestAuthHandlersProtectWorkspaceRoutes(t *testing.T) {
	root := initGitRepo(t, t.TempDir())
	server := newAuthenticatedTestServer(t, root, "secret")

	unauthorized := request(server, http.MethodGet, "/api/workspaces", "")
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", unauthorized.Code, unauthorized.Body.String())
	}

	login := request(server, http.MethodPost, "/api/auth/login", `{"token":"secret"}`)
	if login.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", login.Code, login.Body.String())
	}
	var loginBody struct {
		Session auth.Session `json:"session"`
	}
	mustDecode(t, login, &loginBody)
	if !strings.HasPrefix(loginBody.Session.ID, "auth_") || loginBody.Session.ExpiresAt.IsZero() {
		t.Fatalf("unexpected login body: %+v", loginBody)
	}
	cookies := login.Result().Cookies()
	if len(cookies) == 0 || cookies[0].Name != auth.CookieName {
		t.Fatalf("expected session cookie, got %+v", cookies)
	}

	authorized := requestWithCookies(server, http.MethodGet, "/api/workspaces", "", cookies)
	if authorized.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", authorized.Code, authorized.Body.String())
	}

	session := requestWithCookies(server, http.MethodGet, "/api/auth/session", "", cookies)
	if session.Code != http.StatusOK {
		t.Fatalf("expected session 200, got %d: %s", session.Code, session.Body.String())
	}
	var sessionBody struct {
		Session auth.Session `json:"session"`
	}
	mustDecode(t, session, &sessionBody)
	if sessionBody.Session.ID != loginBody.Session.ID {
		t.Fatalf("expected same session %q, got %+v", loginBody.Session.ID, sessionBody)
	}

	logout := requestWithCookies(server, http.MethodPost, "/api/auth/logout", "", cookies)
	if logout.Code != http.StatusOK {
		t.Fatalf("expected logout 200, got %d: %s", logout.Code, logout.Body.String())
	}
	var logoutBody map[string]string
	mustDecode(t, logout, &logoutBody)
	if logoutBody["status"] != "ok" {
		t.Fatalf("unexpected logout body: %+v", logoutBody)
	}
	if cleared := logout.Result().Cookies(); len(cleared) == 0 || cleared[0].Name != auth.CookieName || cleared[0].MaxAge >= 0 {
		t.Fatalf("expected cleared session cookie, got %+v", cleared)
	}

	afterLogout := requestWithCookies(server, http.MethodGet, "/api/auth/session", "", cookies)
	if afterLogout.Code != http.StatusUnauthorized {
		t.Fatalf("expected session 401 after logout, got %d: %s", afterLogout.Code, afterLogout.Body.String())
	}
}

func TestCreateWorkspaceReturnsWorkspace(t *testing.T) {
	root := initGitRepo(t, t.TempDir())
	server := newTestServer(t, root)

	response := request(server, http.MethodPost, "/api/workspaces", `{"rootPath":"`+root+`"}`)
	if response.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", response.Code, response.Body.String())
	}
	var body map[string]any
	mustDecode(t, response, &body)
	if !strings.HasPrefix(body["id"].(string), "ws_") {
		t.Fatalf("expected ws_ ID, got %+v", body)
	}

	get := request(server, http.MethodGet, "/api/workspaces/"+body["id"].(string), "")
	if get.Code != http.StatusOK {
		t.Fatalf("expected get 200, got %d: %s", get.Code, get.Body.String())
	}

	deleted := request(server, http.MethodDelete, "/api/workspaces/"+body["id"].(string), "")
	if deleted.Code != http.StatusOK {
		t.Fatalf("expected delete 200, got %d: %s", deleted.Code, deleted.Body.String())
	}
	var deletedBody map[string]string
	mustDecode(t, deleted, &deletedBody)
	if deletedBody["status"] != "deleted" {
		t.Fatalf("unexpected delete body: %+v", deletedBody)
	}

	missing := request(server, http.MethodGet, "/api/workspaces/"+body["id"].(string), "")
	if missing.Code != http.StatusNotFound {
		t.Fatalf("expected missing workspace 404, got %d: %s", missing.Code, missing.Body.String())
	}
}

func TestCreateWorkspaceReturnsRestError(t *testing.T) {
	server := newTestServer(t, t.TempDir())

	response := request(server, http.MethodPost, "/api/workspaces", `{"rootPath":"relative"}`)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", response.Code)
	}
	var body map[string]map[string]any
	mustDecode(t, response, &body)
	if body["error"]["code"] != "invalid_workspace_root" {
		t.Fatalf("unexpected error body: %+v", body)
	}
}

func TestListWorkspacesReturnsNewestFirst(t *testing.T) {
	allowed := t.TempDir()
	firstRoot := initGitRepo(t, filepath.Join(allowed, "first"))
	secondRoot := initGitRepo(t, filepath.Join(allowed, "second"))
	server := newTestServer(t, filepath.Dir(firstRoot))

	firstCreate := request(server, http.MethodPost, "/api/workspaces", `{"rootPath":"`+firstRoot+`"}`)
	if firstCreate.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", firstCreate.Code, firstCreate.Body.String())
	}
	var firstWorkspace workspace.Workspace
	mustDecode(t, firstCreate, &firstWorkspace)
	secondCreate := request(server, http.MethodPost, "/api/workspaces", `{"rootPath":"`+secondRoot+`"}`)
	if secondCreate.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", secondCreate.Code, secondCreate.Body.String())
	}
	var secondWorkspace workspace.Workspace
	mustDecode(t, secondCreate, &secondWorkspace)

	response := request(server, http.MethodGet, "/api/workspaces", "")
	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}
	var body struct {
		Workspaces []workspace.Workspace `json:"workspaces"`
	}
	mustDecode(t, response, &body)
	if len(body.Workspaces) != 2 {
		t.Fatalf("expected 2 workspaces, got %+v", body.Workspaces)
	}
	if body.Workspaces[0].ID != secondWorkspace.ID || body.Workspaces[1].ID != firstWorkspace.ID {
		t.Fatalf("expected newest-first workspaces, got %+v", body.Workspaces)
	}
}

func TestWorkspacesPersistAcrossServers(t *testing.T) {
	root := initGitRepo(t, t.TempDir())
	dbPath := filepath.Join(t.TempDir(), "patchpilot.db")
	firstServer := newTestServerWithDBPath(t, root, dbPath, fakeHealthChecker{})

	create := request(firstServer, http.MethodPost, "/api/workspaces", `{"rootPath":"`+root+`"}`)
	if create.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", create.Code, create.Body.String())
	}
	var created workspace.Workspace
	mustDecode(t, create, &created)

	secondServer := newTestServerWithDBPath(t, root, dbPath, fakeHealthChecker{})
	list := request(secondServer, http.MethodGet, "/api/workspaces", "")
	if list.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", list.Code, list.Body.String())
	}
	var body struct {
		Workspaces []workspace.Workspace `json:"workspaces"`
	}
	mustDecode(t, list, &body)
	if len(body.Workspaces) != 1 || body.Workspaces[0].ID != created.ID {
		t.Fatalf("expected persisted workspace %q, got %+v", created.ID, body.Workspaces)
	}

	restore := request(secondServer, http.MethodPost, "/api/workspaces", `{"rootPath":"`+root+`"}`)
	if restore.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", restore.Code, restore.Body.String())
	}
	var restored workspace.Workspace
	mustDecode(t, restore, &restored)
	if restored.ID != created.ID {
		t.Fatalf("expected restored workspace ID %q, got %q", created.ID, restored.ID)
	}
}

func TestFileAndCommandHandlers(t *testing.T) {
	root := initGitRepo(t, t.TempDir())
	if err := os.WriteFile(filepath.Join(root, "note.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	server := newTestServer(t, root)
	create := request(server, http.MethodPost, "/api/workspaces", `{"rootPath":"`+root+`"}`)
	var ws workspace.Workspace
	mustDecode(t, create, &ws)

	fileResponse := request(server, http.MethodGet, "/api/workspaces/"+ws.ID+"/file?path=note.txt", "")
	if fileResponse.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", fileResponse.Code, fileResponse.Body.String())
	}

	commandRecorder := request(server, http.MethodPost, "/api/workspaces/"+ws.ID+"/commands", `{"command":"go test ./..."}`)
	if commandRecorder.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", commandRecorder.Code, commandRecorder.Body.String())
	}
	var command commandResponse
	mustDecode(t, commandRecorder, &command)
	if command.Status != "queued" || !strings.HasPrefix(command.ID, "cmd_") || command.WorkspaceID != ws.ID {
		t.Fatalf("unexpected command: %+v", command)
	}
}

func TestWriteFileHandlerUpdatesFileIndexAndGitStatus(t *testing.T) {
	root := initGitRepo(t, t.TempDir())
	configureCommitter(t, root)
	if err := os.WriteFile(filepath.Join(root, "note.txt"), []byte("before"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	run(t, root, "git", "add", "note.txt")
	run(t, root, "git", "commit", "-m", "initial")
	server := newTestServer(t, root)
	create := request(server, http.MethodPost, "/api/workspaces", `{"rootPath":"`+root+`"}`)
	var ws workspace.Workspace
	mustDecode(t, create, &ws)

	response := request(server, http.MethodPut, "/api/workspaces/"+ws.ID+"/file", `{"path":"note.txt","content":"after"}`)
	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}
	var file filestore.File
	mustDecode(t, response, &file)
	if file.Path != "note.txt" || file.Content != "after" {
		t.Fatalf("unexpected write response: %+v", file)
	}

	index := request(server, http.MethodGet, "/api/workspaces/"+ws.ID+"/files/index", "")
	var indexBody struct {
		Entries []workspace.FileIndexEntry `json:"entries"`
	}
	mustDecode(t, index, &indexBody)
	if len(indexBody.Entries) != 1 || indexBody.Entries[0].Path != "note.txt" || indexBody.Entries[0].Size != 5 {
		t.Fatalf("expected refreshed index, got %+v", indexBody)
	}

	status := request(server, http.MethodGet, "/api/workspaces/"+ws.ID+"/git/status", "")
	var statusBody gitrepo.Status
	mustDecode(t, status, &statusBody)
	if !strings.Contains(statusBody.Porcelain, " M note.txt") {
		t.Fatalf("expected modified git status, got %q", statusBody.Porcelain)
	}
}

func TestWriteFileHandlerRejectsUnsafeWrites(t *testing.T) {
	root := initGitRepo(t, t.TempDir())
	if err := os.WriteFile(filepath.Join(root, "note.txt"), []byte("before"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".env"), []byte("TOKEN=value"), 0o644); err != nil {
		t.Fatalf("write env file: %v", err)
	}
	server := newTestServer(t, root)
	create := request(server, http.MethodPost, "/api/workspaces", `{"rootPath":"`+root+`"}`)
	var ws workspace.Workspace
	mustDecode(t, create, &ws)

	tests := []struct {
		name string
		body string
		code string
	}{
		{name: "traversal", body: `{"path":"../secret.txt","content":"after"}`, code: "path_outside_workspace"},
		{name: "secret", body: `{"path":".env","content":"after"}`, code: "secret_path"},
		{name: "binary", body: "{\"path\":\"note.txt\",\"content\":\"after\\u0000\"}", code: "not_text_file"},
		{name: "missing", body: `{"path":"missing.txt","content":"after"}`, code: "path_not_found"},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			response := request(server, http.MethodPut, "/api/workspaces/"+ws.ID+"/file", testCase.body)
			if response.Code < 400 {
				t.Fatalf("expected error response, got %d: %s", response.Code, response.Body.String())
			}
			var body map[string]map[string]any
			mustDecode(t, response, &body)
			if body["error"]["code"] != testCase.code {
				t.Fatalf("expected %s, got %+v", testCase.code, body)
			}
		})
	}
}

func TestCommandHandlersBlockUnsafeCommands(t *testing.T) {
	root := initGitRepo(t, t.TempDir())
	server := newTestServer(t, root)
	create := request(server, http.MethodPost, "/api/workspaces", `{"rootPath":"`+root+`"}`)
	var ws workspace.Workspace
	mustDecode(t, create, &ws)

	for _, command := range []string{"node scripts/check.js", "rm -rf dist", "pnpm test > out.txt", "go test ../other"} {
		t.Run(command, func(t *testing.T) {
			blocked := request(server, http.MethodPost, "/api/workspaces/"+ws.ID+"/commands", `{"command":"`+command+`"}`)
			if blocked.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d: %s", blocked.Code, blocked.Body.String())
			}
			var blockedBody map[string]map[string]any
			mustDecode(t, blocked, &blockedBody)
			if blockedBody["error"]["code"] != "blocked_command" {
				t.Fatalf("unexpected blocked body: %+v", blockedBody)
			}
			if _, ok := blockedBody["error"]["details"].(map[string]any)["decision"]; !ok {
				t.Fatalf("expected blocked decision details, got %+v", blockedBody)
			}
		})
	}
}

func TestProcessListPaginatesWithCursor(t *testing.T) {
	root := initGitRepo(t, t.TempDir())
	fixture := newServerFixture(t, root, fakeAgentProvider{})
	handler := fixture.server.Routes()
	create := request(handler, http.MethodPost, "/api/workspaces", `{"rootPath":"`+root+`"}`)
	var ws workspace.Workspace
	mustDecode(t, create, &ws)

	baseTime := time.Date(2026, 5, 20, 10, 0, 0, 0, time.UTC)
	for i := 0; i < 51; i++ {
		if _, err := fixture.store.CreateCommand(context.Background(), database.CommandRecord{
			WorkspaceID: ws.ID,
			Command:     fmt.Sprintf("cmd %02d", i),
			Cwd:         root,
			Status:      "exited",
			CreatedAt:   baseTime.Add(time.Duration(i) * time.Second),
		}); err != nil {
			t.Fatalf("CreateCommand %d returned error: %v", i, err)
		}
	}

	defaultPage := request(handler, http.MethodGet, "/api/workspaces/"+ws.ID+"/processes", "")
	if defaultPage.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", defaultPage.Code, defaultPage.Body.String())
	}
	var defaultBody struct {
		Processes  []commandResponse `json:"processes"`
		NextCursor *string           `json:"nextCursor"`
	}
	mustDecode(t, defaultPage, &defaultBody)
	if len(defaultBody.Processes) != 50 || defaultBody.NextCursor == nil {
		t.Fatalf("expected default page of 50 with cursor, got %+v", defaultBody)
	}

	customPage := request(handler, http.MethodGet, "/api/workspaces/"+ws.ID+"/processes?limit=2", "")
	if customPage.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", customPage.Code, customPage.Body.String())
	}
	var customBody struct {
		Processes  []commandResponse `json:"processes"`
		NextCursor *string           `json:"nextCursor"`
	}
	mustDecode(t, customPage, &customBody)
	if len(customBody.Processes) != 2 || customBody.Processes[0].Command != "cmd 50" || customBody.Processes[1].Command != "cmd 49" || customBody.NextCursor == nil {
		t.Fatalf("unexpected custom page: %+v", customBody)
	}

	nextPage := request(handler, http.MethodGet, "/api/workspaces/"+ws.ID+"/processes?limit=2&cursor="+*customBody.NextCursor, "")
	if nextPage.Code != http.StatusOK {
		t.Fatalf("expected next page 200, got %d: %s", nextPage.Code, nextPage.Body.String())
	}
	var nextBody struct {
		Processes []commandResponse `json:"processes"`
	}
	mustDecode(t, nextPage, &nextBody)
	if len(nextBody.Processes) != 2 || nextBody.Processes[0].Command != "cmd 48" {
		t.Fatalf("unexpected next page: %+v", nextBody)
	}

	tooLarge := request(handler, http.MethodGet, "/api/workspaces/"+ws.ID+"/processes?limit=101", "")
	if tooLarge.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid limit 400, got %d: %s", tooLarge.Code, tooLarge.Body.String())
	}
	var tooLargeBody map[string]map[string]any
	mustDecode(t, tooLarge, &tooLargeBody)
	if tooLargeBody["error"]["code"] != "invalid_limit" {
		t.Fatalf("unexpected invalid limit body: %+v", tooLargeBody)
	}

	badCursor := request(handler, http.MethodGet, "/api/workspaces/"+ws.ID+"/processes?cursor=not-base64", "")
	if badCursor.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid cursor 400, got %d: %s", badCursor.Code, badCursor.Body.String())
	}
	var badCursorBody map[string]map[string]any
	mustDecode(t, badCursor, &badCursorBody)
	if badCursorBody["error"]["code"] != "invalid_cursor" {
		t.Fatalf("unexpected invalid cursor body: %+v", badCursorBody)
	}
}

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

func TestServerShutdownStopsActiveCommandsAndPublishesExit(t *testing.T) {
	fixture := newServerFixture(t, t.TempDir(), fakeAgentProvider{})
	eventsCh, unsubscribe := fixture.hub.Subscribe()
	defer unsubscribe()

	queued, err := fixture.store.CreateCommand(context.Background(), database.CommandRecord{
		WorkspaceID: "ws_1",
		Command:     "echo later",
		Cwd:         t.TempDir(),
		Status:      "queued",
	})
	if err != nil {
		t.Fatalf("CreateCommand queued returned error: %v", err)
	}
	running, err := fixture.store.CreateCommand(context.Background(), database.CommandRecord{
		WorkspaceID: "ws_1",
		Command:     "go test ./...",
		Cwd:         filepath.Join("..", "runner", "testdata", "slow"),
		Status:      "queued",
	})
	if err != nil {
		t.Fatalf("CreateCommand running returned error: %v", err)
	}
	if err := fixture.runner.Start(runner.RunSpec{
		ID:          running.ID,
		WorkspaceID: running.WorkspaceID,
		Command:     running.Command,
		Cwd:         running.Cwd,
	}, fixture.server.commandHooks(running.WorkspaceID, running.ID)); err != nil {
		t.Fatalf("runner.Start returned error: %v", err)
	}
	waitForCommandStatus(t, fixture.store, running.WorkspaceID, running.ID, "running")

	if err := fixture.server.Shutdown(context.Background(), "backend shutdown"); err != nil {
		t.Fatalf("Shutdown returned error: %v", err)
	}

	queued = waitForCommandStatus(t, fixture.store, queued.WorkspaceID, queued.ID, "stopped")
	running = waitForCommandStatus(t, fixture.store, running.WorkspaceID, running.ID, "stopped")
	if queued.FinishedAt == nil || running.FinishedAt == nil {
		t.Fatalf("expected finished_at on stopped commands, queued=%+v running=%+v", queued, running)
	}
	if got := receiveProcessExitedEvents(t, eventsCh, 2); len(got) != 2 {
		t.Fatalf("expected 2 process.exited events, got %+v", got)
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

func TestConversationRunHandlersValidateInputAndProvider(t *testing.T) {
	root := initGitRepo(t, t.TempDir())
	server := newTestServer(t, root)
	create := request(server, http.MethodPost, "/api/workspaces", `{"rootPath":"`+root+`"}`)
	var ws workspace.Workspace
	mustDecode(t, create, &ws)

	conversation := request(server, http.MethodPost, "/api/workspaces/"+ws.ID+"/conversations", `{"title":"Fix bug"}`)
	var createdConversation conversationResponse
	mustDecode(t, conversation, &createdConversation)

	for name, testCase := range map[string]struct {
		body string
		code string
	}{
		"empty prompt": {`{"content":"","model":"gpt-5.5","reasoningEffort":"medium"}`, "empty_prompt"},
		"bad model":    {`{"content":"fix","model":"bad","reasoningEffort":"medium"}`, "invalid_model"},
		"bad effort":   {`{"content":"fix","model":"gpt-5.5","reasoningEffort":"none"}`, "invalid_reasoning_effort"},
	} {
		response := request(server, http.MethodPost, "/api/workspaces/"+ws.ID+"/conversations/"+createdConversation.ID+"/messages", testCase.body)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("%s: expected 400, got %d: %s", name, response.Code, response.Body.String())
		}
		var errorBody map[string]map[string]any
		mustDecode(t, response, &errorBody)
		if errorBody["error"]["code"] != testCase.code {
			t.Fatalf("%s: expected %q, got %+v", name, testCase.code, errorBody)
		}
	}

	unavailable := newTestServerWithAgentProvider(t, root, filepath.Join(t.TempDir(), "patchpilot.db"), fakeHealthChecker{}, unavailableAgentProvider{})
	create = request(unavailable, http.MethodPost, "/api/workspaces", `{"rootPath":"`+root+`"}`)
	mustDecode(t, create, &ws)
	conversation = request(unavailable, http.MethodPost, "/api/workspaces/"+ws.ID+"/conversations", `{"title":"Fix bug"}`)
	mustDecode(t, conversation, &createdConversation)
	response := request(unavailable, http.MethodPost, "/api/workspaces/"+ws.ID+"/conversations/"+createdConversation.ID+"/messages", `{"content":"fix","model":"gpt-5.5","reasoningEffort":"medium"}`)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", response.Code, response.Body.String())
	}
	var errorBody map[string]map[string]any
	mustDecode(t, response, &errorBody)
	if errorBody["error"]["code"] != "agent_provider_unavailable" {
		t.Fatalf("unexpected provider error: %+v", errorBody)
	}
}

func TestGitStageAndCommitHandlers(t *testing.T) {
	root := initGitRepo(t, t.TempDir())
	configureCommitter(t, root)
	if err := os.WriteFile(filepath.Join(root, "first.txt"), []byte("first\n"), 0o644); err != nil {
		t.Fatalf("write first file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "second.txt"), []byte("second\n"), 0o644); err != nil {
		t.Fatalf("write second file: %v", err)
	}
	server := newTestServer(t, root)
	create := request(server, http.MethodPost, "/api/workspaces", `{"rootPath":"`+root+`"}`)
	var ws workspace.Workspace
	mustDecode(t, create, &ws)

	stageResponse := request(server, http.MethodPost, "/api/workspaces/"+ws.ID+"/git/stage", `{"paths":["first.txt"]}`)
	if stageResponse.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", stageResponse.Code, stageResponse.Body.String())
	}
	var stageBody gitrepo.Status
	mustDecode(t, stageResponse, &stageBody)
	if !strings.Contains(stageBody.Porcelain, "A  first.txt") || !strings.Contains(stageBody.Porcelain, "?? second.txt") {
		t.Fatalf("unexpected staged status: %q", stageBody.Porcelain)
	}

	commitResponse := request(server, http.MethodPost, "/api/workspaces/"+ws.ID+"/git/commit", `{"message":"add first","paths":["first.txt"]}`)
	if commitResponse.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", commitResponse.Code, commitResponse.Body.String())
	}
	var commitBody gitrepo.Commit
	mustDecode(t, commitResponse, &commitBody)
	if commitBody.Hash == "" {
		t.Fatal("expected commit hash")
	}

	statusResponse := request(server, http.MethodGet, "/api/workspaces/"+ws.ID+"/git/status", "")
	var statusBody gitrepo.Status
	mustDecode(t, statusResponse, &statusBody)
	if strings.Contains(statusBody.Porcelain, "first.txt") || !strings.Contains(statusBody.Porcelain, "?? second.txt") {
		t.Fatalf("expected only second file to remain changed, got %q", statusBody.Porcelain)
	}
}

func TestGitUnstageAndDiscardHandlers(t *testing.T) {
	root := initGitRepo(t, t.TempDir())
	configureCommitter(t, root)
	if err := os.WriteFile(filepath.Join(root, "tracked.txt"), []byte("before\n"), 0o644); err != nil {
		t.Fatalf("write tracked file: %v", err)
	}
	run(t, root, "git", "add", "tracked.txt")
	run(t, root, "git", "commit", "-m", "initial")
	if err := os.WriteFile(filepath.Join(root, "tracked.txt"), []byte("after\n"), 0o644); err != nil {
		t.Fatalf("modify tracked file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "new.txt"), []byte("new\n"), 0o644); err != nil {
		t.Fatalf("write new file: %v", err)
	}
	server := newTestServer(t, root)
	create := request(server, http.MethodPost, "/api/workspaces", `{"rootPath":"`+root+`"}`)
	var ws workspace.Workspace
	mustDecode(t, create, &ws)

	stageResponse := request(server, http.MethodPost, "/api/workspaces/"+ws.ID+"/git/stage", `{"paths":["tracked.txt","new.txt"]}`)
	if stageResponse.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", stageResponse.Code, stageResponse.Body.String())
	}

	unstageResponse := request(server, http.MethodPost, "/api/workspaces/"+ws.ID+"/git/unstage", `{"paths":["new.txt"]}`)
	if unstageResponse.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", unstageResponse.Code, unstageResponse.Body.String())
	}
	var unstageBody gitrepo.Status
	mustDecode(t, unstageResponse, &unstageBody)
	if !strings.Contains(unstageBody.Porcelain, "M  tracked.txt") || !strings.Contains(unstageBody.Porcelain, "?? new.txt") {
		t.Fatalf("unexpected unstage status: %q", unstageBody.Porcelain)
	}

	discardResponse := request(server, http.MethodPost, "/api/workspaces/"+ws.ID+"/git/discard", `{"paths":["new.txt"]}`)
	if discardResponse.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", discardResponse.Code, discardResponse.Body.String())
	}
	var discardBody gitrepo.Status
	mustDecode(t, discardResponse, &discardBody)
	if strings.Contains(discardBody.Porcelain, "new.txt") || !strings.Contains(discardBody.Porcelain, "M  tracked.txt") {
		t.Fatalf("unexpected discard status: %q", discardBody.Porcelain)
	}
}

func TestGitHandlersReturnValidationErrors(t *testing.T) {
	root := initGitRepo(t, t.TempDir())
	server := newTestServer(t, root)
	create := request(server, http.MethodPost, "/api/workspaces", `{"rootPath":"`+root+`"}`)
	var ws workspace.Workspace
	mustDecode(t, create, &ws)

	stageResponse := request(server, http.MethodPost, "/api/workspaces/"+ws.ID+"/git/stage", `{"paths":[]}`)
	if stageResponse.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", stageResponse.Code, stageResponse.Body.String())
	}
	var stageBody map[string]map[string]any
	mustDecode(t, stageResponse, &stageBody)
	if stageBody["error"]["code"] != "empty_path_list" {
		t.Fatalf("unexpected stage error: %+v", stageBody)
	}

	commitResponse := request(server, http.MethodPost, "/api/workspaces/"+ws.ID+"/git/commit", `{"message":"","paths":["note.txt"]}`)
	if commitResponse.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", commitResponse.Code, commitResponse.Body.String())
	}
	var commitBody map[string]map[string]any
	mustDecode(t, commitResponse, &commitBody)
	if commitBody["error"]["code"] != "empty_commit_message" {
		t.Fatalf("unexpected commit error: %+v", commitBody)
	}
}

func TestCreateWorkspaceRefreshesFileIndex(t *testing.T) {
	root := initGitRepo(t, t.TempDir())
	mustMkdirAll(t, filepath.Join(root, "src"))
	if err := os.WriteFile(filepath.Join(root, "src", "note.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	server := newTestServer(t, root)

	create := request(server, http.MethodPost, "/api/workspaces", `{"rootPath":"`+root+`"}`)
	if create.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", create.Code, create.Body.String())
	}
	var ws workspace.Workspace
	mustDecode(t, create, &ws)

	response := request(server, http.MethodGet, "/api/workspaces/"+ws.ID+"/files/index", "")
	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}
	var body struct {
		Entries []workspace.FileIndexEntry `json:"entries"`
	}
	mustDecode(t, response, &body)
	if len(body.Entries) != 1 || body.Entries[0].Path != "src/note.txt" || body.Entries[0].Size != 5 || body.Entries[0].ModifiedAt.IsZero() {
		t.Fatalf("unexpected index body: %+v", body)
	}
}

func TestRefreshFileIndexRebuildsEntries(t *testing.T) {
	root := initGitRepo(t, t.TempDir())
	if err := os.WriteFile(filepath.Join(root, "old.txt"), []byte("old"), 0o644); err != nil {
		t.Fatalf("write old file: %v", err)
	}
	server := newTestServer(t, root)
	create := request(server, http.MethodPost, "/api/workspaces", `{"rootPath":"`+root+`"}`)
	var ws workspace.Workspace
	mustDecode(t, create, &ws)
	if err := os.Remove(filepath.Join(root, "old.txt")); err != nil {
		t.Fatalf("remove old file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "new.txt"), []byte("new file"), 0o644); err != nil {
		t.Fatalf("write new file: %v", err)
	}

	response := request(server, http.MethodPost, "/api/workspaces/"+ws.ID+"/files/index/refresh", "")
	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}
	var body struct {
		Entries []workspace.FileIndexEntry `json:"entries"`
	}
	mustDecode(t, response, &body)
	if len(body.Entries) != 1 || body.Entries[0].Path != "new.txt" || body.Entries[0].Size != 8 {
		t.Fatalf("unexpected refreshed index body: %+v", body)
	}
}

func TestGetWorkspaceRefreshesFileIndex(t *testing.T) {
	root := initGitRepo(t, t.TempDir())
	if err := os.WriteFile(filepath.Join(root, "first.txt"), []byte("first"), 0o644); err != nil {
		t.Fatalf("write first file: %v", err)
	}
	server := newTestServer(t, root)
	create := request(server, http.MethodPost, "/api/workspaces", `{"rootPath":"`+root+`"}`)
	var ws workspace.Workspace
	mustDecode(t, create, &ws)
	if err := os.WriteFile(filepath.Join(root, "second.txt"), []byte("second"), 0o644); err != nil {
		t.Fatalf("write second file: %v", err)
	}

	get := request(server, http.MethodGet, "/api/workspaces/"+ws.ID, "")
	if get.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", get.Code, get.Body.String())
	}
	response := request(server, http.MethodGet, "/api/workspaces/"+ws.ID+"/files/index", "")
	var body struct {
		Entries []workspace.FileIndexEntry `json:"entries"`
	}
	mustDecode(t, response, &body)
	if len(body.Entries) != 2 || body.Entries[1].Path != "second.txt" {
		t.Fatalf("expected get workspace to refresh index, got %+v", body)
	}
}

func TestSearchFilesReturnsMatches(t *testing.T) {
	root := initGitRepo(t, t.TempDir())
	mustMkdirAll(t, filepath.Join(root, "src"))
	if err := os.WriteFile(filepath.Join(root, "src", "note.txt"), []byte("hello world"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	server := newTestServer(t, root)
	create := request(server, http.MethodPost, "/api/workspaces", `{"rootPath":"`+root+`"}`)
	var ws workspace.Workspace
	mustDecode(t, create, &ws)

	response := request(server, http.MethodGet, "/api/workspaces/"+ws.ID+"/search?q=hello", "")
	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}
	var body struct {
		Results []filestore.SearchResult `json:"results"`
	}
	mustDecode(t, response, &body)
	if len(body.Results) != 1 || body.Results[0].Path != "src/note.txt" || body.Results[0].Kind != "content" {
		t.Fatalf("unexpected search results: %+v", body.Results)
	}
}

func TestReadLargeFileReturnsRestError(t *testing.T) {
	root := initGitRepo(t, t.TempDir())
	if err := os.WriteFile(filepath.Join(root, "large.txt"), make([]byte, filestore.MaxReadableFileSize+1), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	server := newTestServer(t, root)
	create := request(server, http.MethodPost, "/api/workspaces", `{"rootPath":"`+root+`"}`)
	var ws workspace.Workspace
	mustDecode(t, create, &ws)

	response := request(server, http.MethodGet, "/api/workspaces/"+ws.ID+"/file?path=large.txt", "")
	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", response.Code, response.Body.String())
	}
	var body map[string]map[string]any
	mustDecode(t, response, &body)
	if body["error"]["code"] != "file_too_large" {
		t.Fatalf("unexpected error body: %+v", body)
	}
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
