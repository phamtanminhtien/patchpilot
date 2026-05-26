package api

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/phamtanminhtien/patchpilot/internal/database"
	"github.com/phamtanminhtien/patchpilot/internal/runner"
	"github.com/phamtanminhtien/patchpilot/internal/workspace"
)

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
