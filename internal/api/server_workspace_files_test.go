package api

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/phamtanminhtien/patchpilot/internal/filestore"
	"github.com/phamtanminhtien/patchpilot/internal/gitrepo"
	"github.com/phamtanminhtien/patchpilot/internal/workspace"
)

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

func TestFileHandlerReadsWorkspaceFile(t *testing.T) {
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
