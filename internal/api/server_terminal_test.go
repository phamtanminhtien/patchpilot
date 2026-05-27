package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/phamtanminhtien/patchpilot/internal/workspace"
	"nhooyr.io/websocket"
)

func TestTerminalSessionHandlersLifecycle(t *testing.T) {
	root := initGitRepo(t, t.TempDir())
	fixture := newServerFixture(t, root, fakeAgentProvider{})
	handler := fixture.server.Routes()
	createWorkspace := request(handler, http.MethodPost, "/api/workspaces", `{"rootPath":"`+root+`"}`)
	var ws workspace.Workspace
	mustDecode(t, createWorkspace, &ws)

	create := request(handler, http.MethodPost, "/api/workspaces/"+ws.ID+"/terminal/sessions", `{"title":"Dev","rows":30,"cols":100}`)
	if create.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", create.Code, create.Body.String())
	}
	var created struct {
		Session terminalSessionResponse `json:"session"`
	}
	mustDecode(t, create, &created)
	if created.Session.ID == "" || created.Session.Title != "Dev" || created.Session.Rows != 30 || created.Session.Cols != 100 || created.Session.Status != "open" {
		t.Fatalf("unexpected created terminal session: %+v", created.Session)
	}

	patch := request(handler, http.MethodPatch, "/api/workspaces/"+ws.ID+"/terminal/sessions/"+created.Session.ID, `{"title":"Renamed","rows":35,"cols":120}`)
	if patch.Code != http.StatusOK {
		t.Fatalf("expected patch 200, got %d: %s", patch.Code, patch.Body.String())
	}
	var patched struct {
		Session terminalSessionResponse `json:"session"`
	}
	mustDecode(t, patch, &patched)
	if patched.Session.Title != "Renamed" || patched.Session.Rows != 35 || patched.Session.Cols != 120 {
		t.Fatalf("unexpected patched terminal session: %+v", patched.Session)
	}

	list := request(handler, http.MethodGet, "/api/workspaces/"+ws.ID+"/terminal/sessions", "")
	if list.Code != http.StatusOK {
		t.Fatalf("expected list 200, got %d: %s", list.Code, list.Body.String())
	}
	var listed struct {
		Sessions []terminalSessionResponse `json:"sessions"`
	}
	mustDecode(t, list, &listed)
	if len(listed.Sessions) != 1 || listed.Sessions[0].ID != created.Session.ID {
		t.Fatalf("unexpected listed sessions: %+v", listed)
	}

	closeResponse := request(handler, http.MethodPost, "/api/workspaces/"+ws.ID+"/terminal/sessions/"+created.Session.ID+"/close", "")
	if closeResponse.Code != http.StatusOK {
		t.Fatalf("expected close 200, got %d: %s", closeResponse.Code, closeResponse.Body.String())
	}
	var closed struct {
		Session terminalSessionResponse `json:"session"`
	}
	mustDecode(t, closeResponse, &closed)
	if closed.Session.Status != "closed" || closed.Session.ClosedAt == nil {
		t.Fatalf("unexpected closed session: %+v", closed.Session)
	}
}

func TestWorkspaceCommandRoutesAreNotExposed(t *testing.T) {
	root := initGitRepo(t, t.TempDir())
	handler := newTestServer(t, root)
	createWorkspace := request(handler, http.MethodPost, "/api/workspaces", `{"rootPath":"`+root+`"}`)
	var ws workspace.Workspace
	mustDecode(t, createWorkspace, &ws)

	command := request(handler, http.MethodPost, "/api/workspaces/"+ws.ID+"/commands", `{"command":"go test ./..."}`)
	if command.Code != http.StatusNotFound {
		t.Fatalf("expected command route 404, got %d: %s", command.Code, command.Body.String())
	}
	processes := request(handler, http.MethodGet, "/api/workspaces/"+ws.ID+"/processes", "")
	if processes.Code != http.StatusNotFound {
		t.Fatalf("expected process route 404, got %d: %s", processes.Code, processes.Body.String())
	}
}

func TestTerminalWebSocketStreamsInputAndOutput(t *testing.T) {
	root := initGitRepo(t, t.TempDir())
	fixture := newServerFixture(t, root, fakeAgentProvider{})
	handler := fixture.server.Routes()
	createWorkspace := request(handler, http.MethodPost, "/api/workspaces", `{"rootPath":"`+root+`"}`)
	var ws workspace.Workspace
	mustDecode(t, createWorkspace, &ws)
	create := request(handler, http.MethodPost, "/api/workspaces/"+ws.ID+"/terminal/sessions", `{"rows":24,"cols":80}`)
	var created struct {
		Session terminalSessionResponse `json:"session"`
	}
	mustDecode(t, create, &created)

	server := httptest.NewServer(handler)
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http")+"/api/workspaces/"+ws.ID+"/terminal/sessions/"+created.Session.ID+"/socket", nil)
	if err != nil {
		t.Fatalf("websocket Dial returned error: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "test done")

	if got := readTerminalMessage(t, ctx, conn); got.Type != "ready" {
		t.Fatalf("expected ready message, got %+v", got)
	}
	if err := conn.Write(ctx, websocket.MessageText, []byte(`{"type":"input","data":"echo __PP_OK__\n"}`)); err != nil {
		t.Fatalf("write input returned error: %v", err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		message := readTerminalMessage(t, ctx, conn)
		if message.Type == "output" && strings.Contains(message.Data, "__PP_OK__") {
			return
		}
	}
	t.Fatal("expected terminal output to contain marker")
}

type terminalTestMessage struct {
	Type string `json:"type"`
	Data string `json:"data"`
}

func readTerminalMessage(t *testing.T, ctx context.Context, conn *websocket.Conn) terminalTestMessage {
	t.Helper()
	_, reader, err := conn.Reader(ctx)
	if err != nil {
		t.Fatalf("read websocket message returned error: %v", err)
	}
	var message terminalTestMessage
	if err := json.NewDecoder(reader).Decode(&message); err != nil {
		t.Fatalf("decode websocket message returned error: %v", err)
	}
	return message
}
