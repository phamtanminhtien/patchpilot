package api

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/phamtanminhtien/patchpilot/internal/database"
	"github.com/phamtanminhtien/patchpilot/internal/workspace"
)

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
