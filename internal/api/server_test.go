package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/phamtanminhtien/patchpilot/internal/database"
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

func TestCommandHandlersConfirmAndBlockBySafety(t *testing.T) {
	root := initGitRepo(t, t.TempDir())
	server := newTestServer(t, root)
	create := request(server, http.MethodPost, "/api/workspaces", `{"rootPath":"`+root+`"}`)
	var ws workspace.Workspace
	mustDecode(t, create, &ws)

	confirmation := request(server, http.MethodPost, "/api/workspaces/"+ws.ID+"/commands", `{"command":"node scripts/check.js"}`)
	if confirmation.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", confirmation.Code, confirmation.Body.String())
	}
	var confirmationBody map[string]map[string]any
	mustDecode(t, confirmation, &confirmationBody)
	if confirmationBody["error"]["code"] != "confirmation_required" {
		t.Fatalf("unexpected confirmation body: %+v", confirmationBody)
	}

	blocked := request(server, http.MethodPost, "/api/workspaces/"+ws.ID+"/commands", `{"command":"rm -rf dist"}`)
	if blocked.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", blocked.Code, blocked.Body.String())
	}
	var blockedBody map[string]map[string]any
	mustDecode(t, blocked, &blockedBody)
	if blockedBody["error"]["code"] != "blocked_command" {
		t.Fatalf("unexpected blocked body: %+v", blockedBody)
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

func newTestServerWithHealth(t *testing.T, allowedRoot string, health HealthChecker) http.Handler {
	t.Helper()
	return newTestServerWithDBPath(t, allowedRoot, filepath.Join(t.TempDir(), "patchpilot.db"), health)
}

func newTestServerWithDBPath(t *testing.T, allowedRoot string, dbPath string, health HealthChecker) http.Handler {
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
	return NewServer(manager, filestore.NewService(), gitrepo.NewClient(), runner.NewRunner(), store, health).Routes()
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

func mustDecode(t *testing.T, response *httptest.ResponseRecorder, target any) {
	t.Helper()
	if err := json.NewDecoder(response.Body).Decode(target); err != nil {
		t.Fatalf("decode response: %v", err)
	}
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

func run(t *testing.T, dir, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v failed: %v\n%s", name, args, err, output)
	}
}
