package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/phamtanminhtien/patchpilot/internal/agent"
	"github.com/phamtanminhtien/patchpilot/internal/database"
	"github.com/phamtanminhtien/patchpilot/internal/events"
	"github.com/phamtanminhtien/patchpilot/internal/filestore"
	"github.com/phamtanminhtien/patchpilot/internal/gitrepo"
	"github.com/phamtanminhtien/patchpilot/internal/runner"
	"github.com/phamtanminhtien/patchpilot/internal/workspace"
)

type Server struct {
	workspaces *workspace.Manager
	files      *filestore.Service
	git        *gitrepo.Client
	runner     *runner.Runner
	store      *database.Store
	events     *events.Hub
	agent      *agent.Manager
	health     HealthChecker
}

type HealthChecker interface {
	Ping(context.Context) error
}

func NewServer(workspaces *workspace.Manager, files *filestore.Service, git *gitrepo.Client, runner *runner.Runner, store *database.Store, hub *events.Hub, agentManager *agent.Manager, health HealthChecker) *Server {
	if hub == nil {
		hub = events.NewHub()
	}
	return &Server{workspaces: workspaces, files: files, git: git, runner: runner, store: store, events: hub, agent: agentManager, health: health}
}

func (s *Server) Routes() http.Handler {
	return s.RoutesWithStatic("")
}

func (s *Server) RoutesWithStatic(staticDir string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", s.healthCheck)
	mux.HandleFunc("POST /api/workspaces", s.createWorkspace)
	mux.HandleFunc("GET /api/workspaces", s.listWorkspaces)
	mux.HandleFunc("GET /api/workspaces/{workspaceId}", s.getWorkspace)
	mux.HandleFunc("GET /api/workspaces/{workspaceId}/files", s.listFiles)
	mux.HandleFunc("GET /api/workspaces/{workspaceId}/files/index", s.listFileIndex)
	mux.HandleFunc("POST /api/workspaces/{workspaceId}/files/index/refresh", s.refreshFileIndex)
	mux.HandleFunc("GET /api/workspaces/{workspaceId}/file", s.readFile)
	mux.HandleFunc("GET /api/workspaces/{workspaceId}/search", s.searchFiles)
	mux.HandleFunc("GET /api/workspaces/{workspaceId}/git/status", s.gitStatus)
	mux.HandleFunc("GET /api/workspaces/{workspaceId}/git/diff", s.gitDiff)
	mux.HandleFunc("POST /api/workspaces/{workspaceId}/git/stage", s.gitStage)
	mux.HandleFunc("POST /api/workspaces/{workspaceId}/git/unstage", s.gitUnstage)
	mux.HandleFunc("POST /api/workspaces/{workspaceId}/git/discard", s.gitDiscard)
	mux.HandleFunc("POST /api/workspaces/{workspaceId}/git/commit", s.gitCommit)
	mux.HandleFunc("POST /api/workspaces/{workspaceId}/agent/tasks", s.createAgentTask)
	mux.HandleFunc("GET /api/workspaces/{workspaceId}/agent/tasks", s.listAgentTasks)
	mux.HandleFunc("GET /api/workspaces/{workspaceId}/agent/tasks/{taskId}", s.getAgentTask)
	mux.HandleFunc("POST /api/workspaces/{workspaceId}/agent/tasks/{taskId}/cancel", s.cancelAgentTask)
	mux.HandleFunc("POST /api/workspaces/{workspaceId}/commands", s.createCommand)
	mux.HandleFunc("GET /api/workspaces/{workspaceId}/processes", s.listProcesses)
	mux.HandleFunc("GET /api/workspaces/{workspaceId}/processes/{processId}", s.getProcess)
	mux.HandleFunc("POST /api/workspaces/{workspaceId}/processes/{processId}/stop", s.stopProcess)
	mux.HandleFunc("GET /api/workspaces/{workspaceId}/events", s.workspaceEvents)
	if staticDir != "" {
		mux.HandleFunc("GET /", serveStatic(staticDir))
		mux.HandleFunc("HEAD /", serveStatic(staticDir))
	}
	return mux
}

func serveStatic(staticDir string) http.HandlerFunc {
	fileServer := http.FileServer(http.Dir(staticDir))
	return func(w http.ResponseWriter, r *http.Request) {
		path := filepath.Clean("/" + r.URL.Path)
		if path == string(filepath.Separator) {
			fileServer.ServeHTTP(w, r)
			return
		}
		if _, err := os.Stat(filepath.Join(staticDir, path[1:])); err == nil {
			fileServer.ServeHTTP(w, r)
			return
		}
		http.ServeFile(w, r, filepath.Join(staticDir, "index.html"))
	}
}

func (s *Server) healthCheck(w http.ResponseWriter, r *http.Request) {
	if s.health != nil {
		if err := s.health.Ping(r.Context()); err != nil {
			writeError(w, http.StatusServiceUnavailable, "database_unavailable", "Database is unavailable", nil)
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type createWorkspaceRequest struct {
	RootPath string `json:"rootPath"`
}

type createCommandRequest struct {
	Command   string `json:"command"`
	Confirmed bool   `json:"confirmed"`
}

type createAgentTaskRequest struct {
	Prompt          string `json:"prompt"`
	Model           string `json:"model"`
	ReasoningEffort string `json:"reasoningEffort"`
}

type gitStageRequest struct {
	Paths []string `json:"paths"`
}

type gitCommitRequest struct {
	Message string   `json:"message"`
	Paths   []string `json:"paths"`
}

func (s *Server) createWorkspace(w http.ResponseWriter, r *http.Request) {
	var req createWorkspaceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Request body must be valid JSON", nil)
		return
	}
	ws, err := s.workspaces.Create(r.Context(), req.RootPath)
	if err != nil {
		writeWorkspaceError(w, err)
		return
	}
	if err := s.refreshWorkspaceIndex(r.Context(), ws); err != nil {
		writeIndexRefreshError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, ws)
}

func (s *Server) listWorkspaces(w http.ResponseWriter, r *http.Request) {
	workspaces, err := s.workspaces.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "workspace_list_failed", "Workspaces could not be listed", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"workspaces": workspaces})
}

func (s *Server) getWorkspace(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.workspaceFromRequest(w, r)
	if !ok {
		return
	}
	if err := s.refreshWorkspaceIndex(r.Context(), ws); err != nil {
		writeIndexRefreshError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ws)
}

func (s *Server) listFiles(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.workspaceFromRequest(w, r)
	if !ok {
		return
	}
	path := r.URL.Query().Get("path")
	if path == "" {
		path = "."
	}
	entries, err := s.files.List(ws.RootPath, path)
	if err != nil {
		writeFileError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"entries": entries})
}

func (s *Server) readFile(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.workspaceFromRequest(w, r)
	if !ok {
		return
	}
	file, err := s.files.Read(ws.RootPath, r.URL.Query().Get("path"))
	if err != nil {
		writeFileError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, file)
}

func (s *Server) listFileIndex(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.workspaceFromRequest(w, r)
	if !ok {
		return
	}
	entries, err := s.workspaces.FileIndex(r.Context(), ws.ID)
	if err != nil {
		writeWorkspaceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"entries": entries})
}

func (s *Server) refreshFileIndex(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.workspaceFromRequest(w, r)
	if !ok {
		return
	}
	if err := s.refreshWorkspaceIndex(r.Context(), ws); err != nil {
		writeIndexRefreshError(w, err)
		return
	}
	entries, err := s.workspaces.FileIndex(r.Context(), ws.ID)
	if err != nil {
		writeWorkspaceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"entries": entries})
}

func (s *Server) searchFiles(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.workspaceFromRequest(w, r)
	if !ok {
		return
	}
	results, err := s.files.Search(ws.RootPath, r.URL.Query().Get("q"))
	if err != nil {
		writeFileError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": results})
}

func (s *Server) refreshWorkspaceIndex(ctx context.Context, ws workspace.Workspace) error {
	fileEntries, err := s.files.Index(ws.RootPath)
	if err != nil {
		return err
	}
	entries := make([]workspace.FileIndexEntry, 0, len(fileEntries))
	for _, entry := range fileEntries {
		entries = append(entries, workspace.FileIndexEntry{
			Path:       entry.Path,
			Size:       entry.Size,
			ModifiedAt: entry.ModifiedAt,
		})
	}
	return s.workspaces.ReplaceFileIndex(ctx, ws.ID, entries)
}

func (s *Server) gitStatus(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.workspaceFromRequest(w, r)
	if !ok {
		return
	}
	status, err := s.git.Status(r.Context(), ws.RootPath)
	if err != nil {
		writeGitError(w, err, "git_status_failed", "Git status failed")
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (s *Server) gitDiff(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.workspaceFromRequest(w, r)
	if !ok {
		return
	}
	diff, err := s.git.Diff(r.Context(), ws.RootPath, r.URL.Query().Get("path"))
	if err != nil {
		writeGitError(w, err, "git_diff_failed", "Git diff failed")
		return
	}
	writeJSON(w, http.StatusOK, diff)
}

func (s *Server) gitStage(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.workspaceFromRequest(w, r)
	if !ok {
		return
	}
	var req gitStageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Request body must be valid JSON", nil)
		return
	}
	status, err := s.git.Stage(r.Context(), ws.RootPath, req.Paths)
	if err != nil {
		writeGitError(w, err, "git_stage_failed", "Git stage failed")
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (s *Server) gitUnstage(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.workspaceFromRequest(w, r)
	if !ok {
		return
	}
	var req gitStageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Request body must be valid JSON", nil)
		return
	}
	status, err := s.git.Unstage(r.Context(), ws.RootPath, req.Paths)
	if err != nil {
		writeGitError(w, err, "git_unstage_failed", "Git unstage failed")
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (s *Server) gitDiscard(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.workspaceFromRequest(w, r)
	if !ok {
		return
	}
	var req gitStageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Request body must be valid JSON", nil)
		return
	}
	status, err := s.git.Discard(r.Context(), ws.RootPath, req.Paths)
	if err != nil {
		writeGitError(w, err, "git_discard_failed", "Git discard failed")
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (s *Server) gitCommit(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.workspaceFromRequest(w, r)
	if !ok {
		return
	}
	var req gitCommitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Request body must be valid JSON", nil)
		return
	}
	commit, err := s.git.Commit(r.Context(), ws.RootPath, req.Message, req.Paths)
	if err != nil {
		writeGitError(w, err, "git_commit_failed", "Git commit failed")
		return
	}
	writeJSON(w, http.StatusOK, commit)
}

func (s *Server) createAgentTask(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.workspaceFromRequest(w, r)
	if !ok {
		return
	}
	if s.agent == nil {
		writeError(w, http.StatusServiceUnavailable, "agent_unavailable", "Agent runtime is unavailable", nil)
		return
	}
	var req createAgentTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Request body must be valid JSON", nil)
		return
	}
	task, err := s.agent.Create(r.Context(), ws.ID, ws.RootPath, agent.CreateTaskInput{
		Prompt:          req.Prompt,
		Model:           req.Model,
		ReasoningEffort: req.ReasoningEffort,
	})
	if err != nil {
		writeAgentError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, task)
}

func (s *Server) listAgentTasks(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.workspaceFromRequest(w, r)
	if !ok {
		return
	}
	if s.agent == nil {
		writeError(w, http.StatusServiceUnavailable, "agent_unavailable", "Agent runtime is unavailable", nil)
		return
	}
	tasks, err := s.agent.List(r.Context(), ws.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "agent_task_list_failed", "Agent tasks could not be listed", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tasks": tasks})
}

func (s *Server) getAgentTask(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.workspaceFromRequest(w, r)
	if !ok {
		return
	}
	if s.agent == nil {
		writeError(w, http.StatusServiceUnavailable, "agent_unavailable", "Agent runtime is unavailable", nil)
		return
	}
	detail, err := s.agent.Detail(r.Context(), ws.ID, r.PathValue("taskId"))
	if err != nil {
		writeAgentError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (s *Server) cancelAgentTask(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.workspaceFromRequest(w, r)
	if !ok {
		return
	}
	if s.agent == nil {
		writeError(w, http.StatusServiceUnavailable, "agent_unavailable", "Agent runtime is unavailable", nil)
		return
	}
	task, err := s.agent.Cancel(r.Context(), ws.ID, r.PathValue("taskId"))
	if err != nil {
		writeAgentError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, task)
}

func (s *Server) createCommand(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.workspaceFromRequest(w, r)
	if !ok {
		return
	}
	var req createCommandRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Request body must be valid JSON", nil)
		return
	}
	decision, err := runner.Classify(req.Command)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_command", "Command is required", nil)
		return
	}
	if decision.Level == runner.SafetyBlocked {
		writeError(w, http.StatusBadRequest, "blocked_command", decision.Reason, map[string]any{"decision": decision})
		return
	}
	if decision.Level == runner.SafetyNeedsConfirmation && !req.Confirmed {
		writeError(w, http.StatusConflict, "confirmation_required", "Command requires confirmation", map[string]any{"decision": decision})
		return
	}
	created, err := s.store.CreateCommand(r.Context(), database.CommandRecord{
		WorkspaceID: ws.ID,
		Command:     req.Command,
		Cwd:         ws.RootPath,
		Status:      "queued",
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "command_create_failed", "Command could not be created", nil)
		return
	}
	if err := s.runner.Start(runner.RunSpec{
		ID:          created.ID,
		WorkspaceID: ws.ID,
		Command:     created.Command,
		Cwd:         created.Cwd,
	}, s.commandHooks(ws.ID, created.ID)); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_command", "Command is invalid", nil)
		return
	}
	writeJSON(w, http.StatusAccepted, commandResponseFromRecord(created))
}

func (s *Server) listProcesses(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.workspaceFromRequest(w, r)
	if !ok {
		return
	}
	commands, err := s.store.ListCommands(r.Context(), ws.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "process_list_failed", "Processes could not be listed", nil)
		return
	}
	response := make([]commandResponse, 0, len(commands))
	for _, command := range commands {
		response = append(response, commandResponseFromRecord(command))
	}
	writeJSON(w, http.StatusOK, map[string]any{"processes": response})
}

func (s *Server) getProcess(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.workspaceFromRequest(w, r)
	if !ok {
		return
	}
	command, err := s.store.GetCommand(r.Context(), ws.ID, r.PathValue("processId"))
	if err != nil {
		writeProcessError(w, err)
		return
	}
	output, err := s.store.ListCommandOutput(r.Context(), command.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "process_output_failed", "Process output could not be loaded", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"command": commandResponseFromRecord(command),
		"output":  outputResponsesFromRecords(output),
	})
}

func (s *Server) stopProcess(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.workspaceFromRequest(w, r)
	if !ok {
		return
	}
	command, err := s.store.GetCommand(r.Context(), ws.ID, r.PathValue("processId"))
	if err != nil {
		writeProcessError(w, err)
		return
	}
	if command.Status == "running" || command.Status == "queued" {
		if stopped := s.runner.Stop(command.ID); !stopped {
			now := time.Now().UTC()
			command, err = s.store.FinishCommand(r.Context(), ws.ID, command.ID, "stopped", nil, now)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "process_stop_failed", "Process could not be stopped", nil)
				return
			}
			s.publishProcessExited(command)
		}
	}
	command, err = s.store.GetCommand(r.Context(), ws.ID, command.ID)
	if err != nil {
		writeProcessError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, commandResponseFromRecord(command))
}

func (s *Server) workspaceEvents(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.workspaceFromRequest(w, r)
	if !ok {
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "sse_unsupported", "Streaming is unavailable", nil)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	s.replayAgentEvents(r.Context(), w, ws.ID)
	s.replayCommandEvents(r.Context(), w, ws.ID)
	flusher.Flush()
	events, unsubscribe := s.events.Subscribe()
	defer unsubscribe()
	for {
		select {
		case <-r.Context().Done():
			return
		case event := <-events:
			if event.WorkspaceID != ws.ID {
				continue
			}
			writeSSE(w, event)
			flusher.Flush()
		}
	}
}

func (s *Server) commandHooks(workspaceID, commandID string) runner.Hooks {
	return runner.Hooks{
		OnStarted: func() {
			startedAt := time.Now().UTC()
			command, err := s.store.MarkCommandRunning(context.Background(), workspaceID, commandID, startedAt)
			if err == nil {
				s.publishProcessStarted(command)
			}
		},
		OnOutput: func(stream, chunk string) {
			output, err := s.store.AppendCommandOutput(context.Background(), database.CommandOutputRecord{
				CommandID: commandID,
				Stream:    stream,
				Chunk:     chunk,
			}, 1024*1024)
			if err == nil {
				s.events.Publish(events.Event{
					WorkspaceID: workspaceID,
					Type:        "command.output",
					Payload:     outputResponseFromRecord(output),
				})
			}
		},
		OnFinished: func(result runner.FinishResult) {
			finishedAt := time.Now().UTC()
			command, err := s.store.FinishCommand(context.Background(), workspaceID, commandID, result.Status, result.ExitCode, finishedAt)
			if err == nil {
				s.publishProcessExited(command)
			}
		},
	}
}

func (s *Server) replayCommandEvents(ctx context.Context, w http.ResponseWriter, workspaceID string) {
	commands, err := s.store.ListCommands(ctx, workspaceID)
	if err != nil {
		return
	}
	for i := len(commands) - 1; i >= 0; i-- {
		command := commands[i]
		if command.StartedAt != nil {
			writeSSE(w, events.Event{
				ID:          "evt_replay_started_" + command.ID,
				WorkspaceID: workspaceID,
				Type:        "process.started",
				CreatedAt:   *command.StartedAt,
				Payload:     commandResponseFromRecord(command),
			})
		}
		output, err := s.store.ListCommandOutput(ctx, command.ID)
		if err != nil {
			continue
		}
		for _, chunk := range output {
			writeSSE(w, events.Event{
				ID:          "evt_replay_output_" + chunk.ID,
				WorkspaceID: workspaceID,
				Type:        "command.output",
				CreatedAt:   chunk.CreatedAt,
				Payload:     outputResponseFromRecord(chunk),
			})
		}
		if command.FinishedAt != nil {
			writeSSE(w, events.Event{
				ID:          "evt_replay_exited_" + command.ID,
				WorkspaceID: workspaceID,
				Type:        "process.exited",
				CreatedAt:   *command.FinishedAt,
				Payload:     commandResponseFromRecord(command),
			})
		}
	}
}

func (s *Server) replayAgentEvents(ctx context.Context, w http.ResponseWriter, workspaceID string) {
	taskEvents, err := s.store.ListAgentTaskEvents(ctx, workspaceID, "")
	if err != nil {
		return
	}
	for _, event := range taskEvents {
		var payload any
		if err := json.Unmarshal([]byte(event.PayloadJSON), &payload); err != nil {
			continue
		}
		writeSSE(w, events.Event{
			ID:          event.ID,
			WorkspaceID: workspaceID,
			Type:        event.Type,
			CreatedAt:   event.CreatedAt,
			Payload:     payload,
		})
	}
}

func (s *Server) publishProcessStarted(command database.CommandRecord) {
	s.events.Publish(events.Event{
		WorkspaceID: command.WorkspaceID,
		Type:        "process.started",
		Payload:     commandResponseFromRecord(command),
	})
}

func (s *Server) publishProcessExited(command database.CommandRecord) {
	s.events.Publish(events.Event{
		WorkspaceID: command.WorkspaceID,
		Type:        "process.exited",
		Payload:     commandResponseFromRecord(command),
	})
}

func (s *Server) workspaceFromRequest(w http.ResponseWriter, r *http.Request) (workspace.Workspace, bool) {
	ws, err := s.workspaces.Get(r.Context(), r.PathValue("workspaceId"))
	if err != nil {
		writeError(w, http.StatusNotFound, "workspace_not_found", "Workspace not found", nil)
		return workspace.Workspace{}, false
	}
	return ws, true
}

type commandResponse struct {
	ID          string     `json:"id"`
	WorkspaceID string     `json:"workspaceId"`
	Command     string     `json:"command"`
	Cwd         string     `json:"cwd"`
	Status      string     `json:"status"`
	ExitCode    *int       `json:"exitCode"`
	StartedAt   *time.Time `json:"startedAt"`
	FinishedAt  *time.Time `json:"finishedAt"`
	CreatedAt   time.Time  `json:"createdAt"`
	DurationMs  *int64     `json:"durationMs"`
}

type commandOutputResponse struct {
	ID        string    `json:"id"`
	CommandID string    `json:"commandId"`
	Stream    string    `json:"stream"`
	Chunk     string    `json:"chunk"`
	CreatedAt time.Time `json:"createdAt"`
}

func commandResponseFromRecord(record database.CommandRecord) commandResponse {
	var durationMs *int64
	if record.StartedAt != nil && record.FinishedAt != nil {
		duration := record.FinishedAt.Sub(*record.StartedAt).Milliseconds()
		durationMs = &duration
	}
	return commandResponse{
		ID:          record.ID,
		WorkspaceID: record.WorkspaceID,
		Command:     record.Command,
		Cwd:         record.Cwd,
		Status:      record.Status,
		ExitCode:    record.ExitCode,
		StartedAt:   record.StartedAt,
		FinishedAt:  record.FinishedAt,
		CreatedAt:   record.CreatedAt,
		DurationMs:  durationMs,
	}
}

func outputResponseFromRecord(record database.CommandOutputRecord) commandOutputResponse {
	return commandOutputResponse{
		ID:        record.ID,
		CommandID: record.CommandID,
		Stream:    record.Stream,
		Chunk:     record.Chunk,
		CreatedAt: record.CreatedAt,
	}
}

func outputResponsesFromRecords(records []database.CommandOutputRecord) []commandOutputResponse {
	output := make([]commandOutputResponse, 0, len(records))
	for _, record := range records {
		output = append(output, outputResponseFromRecord(record))
	}
	return output
}

func writeWorkspaceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, workspace.ErrInvalidRoot):
		writeError(w, http.StatusBadRequest, "invalid_workspace_root", "Workspace root must be an absolute path", nil)
	case errors.Is(err, workspace.ErrOutsideRoots):
		writeError(w, http.StatusBadRequest, "workspace_root_not_allowed", "Workspace root is outside allowed roots", nil)
	case errors.Is(err, workspace.ErrNotGitRepo):
		writeError(w, http.StatusBadRequest, "not_git_repository", "Workspace root must be a Git repository", nil)
	default:
		writeError(w, http.StatusInternalServerError, "workspace_create_failed", "Workspace could not be created", nil)
	}
}

func writeGitError(w http.ResponseWriter, err error, fallbackCode, fallbackMessage string) {
	switch {
	case errors.Is(err, gitrepo.ErrInvalidPath):
		writeError(w, http.StatusBadRequest, "invalid_path", "Path must be workspace-relative", nil)
	case errors.Is(err, gitrepo.ErrEmptyPathList):
		writeError(w, http.StatusBadRequest, "empty_path_list", "At least one path is required", nil)
	case errors.Is(err, gitrepo.ErrEmptyCommitMessage):
		writeError(w, http.StatusBadRequest, "empty_commit_message", "Commit message is required", nil)
	case errors.Is(err, gitrepo.ErrInvalidRoot):
		writeError(w, http.StatusBadRequest, "invalid_workspace_root", "Workspace root must be an absolute path", nil)
	case errors.Is(err, gitrepo.ErrNotRepository):
		writeError(w, http.StatusBadRequest, "not_git_repository", "Workspace root must be a Git repository", nil)
	default:
		writeError(w, http.StatusInternalServerError, fallbackCode, fallbackMessage, nil)
	}
}

func writeFileError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, filestore.ErrInvalidPath):
		writeError(w, http.StatusBadRequest, "invalid_path", "Path must be workspace-relative", nil)
	case errors.Is(err, filestore.ErrOutsideRoot):
		writeError(w, http.StatusBadRequest, "path_outside_workspace", "Path escapes workspace root", nil)
	case errors.Is(err, filestore.ErrIgnoredPath):
		writeError(w, http.StatusBadRequest, "ignored_path", "Path is ignored by the File API", nil)
	case errors.Is(err, filestore.ErrNotTextFile):
		writeError(w, http.StatusBadRequest, "not_text_file", "File is not a readable text file", nil)
	case errors.Is(err, filestore.ErrFileTooLarge):
		writeError(w, http.StatusBadRequest, "file_too_large", "File exceeds the max readable size", nil)
	default:
		writeError(w, http.StatusNotFound, "path_not_found", "Path not found", nil)
	}
}

func writeProcessError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, database.ErrNotFound):
		writeError(w, http.StatusNotFound, "process_not_found", "Process not found", nil)
	default:
		writeError(w, http.StatusInternalServerError, "process_failed", "Process request failed", nil)
	}
}

func writeAgentError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, agent.ErrEmptyPrompt):
		writeError(w, http.StatusBadRequest, "empty_prompt", "Prompt is required", nil)
	case errors.Is(err, agent.ErrInvalidModel):
		writeError(w, http.StatusBadRequest, "invalid_model", "Model is not supported", nil)
	case errors.Is(err, agent.ErrInvalidEffort):
		writeError(w, http.StatusBadRequest, "invalid_reasoning_effort", "Reasoning effort is not supported", nil)
	case errors.Is(err, agent.ErrProviderUnavailable):
		writeError(w, http.StatusBadRequest, "agent_provider_unavailable", "OpenAI provider is not configured", nil)
	case errors.Is(err, agent.ErrTaskNotFound):
		writeError(w, http.StatusNotFound, "agent_task_not_found", "Agent task not found", nil)
	default:
		writeError(w, http.StatusInternalServerError, "agent_task_failed", "Agent task request failed", nil)
	}
}

func writeIndexRefreshError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, filestore.ErrInvalidPath),
		errors.Is(err, filestore.ErrOutsideRoot),
		errors.Is(err, filestore.ErrIgnoredPath),
		errors.Is(err, filestore.ErrNotTextFile),
		errors.Is(err, filestore.ErrFileTooLarge):
		writeFileError(w, err)
	default:
		writeError(w, http.StatusInternalServerError, "file_index_refresh_failed", "File index could not be refreshed", nil)
	}
}

type errorResponse struct {
	Error errorBody `json:"error"`
}

type errorBody struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details"`
}

func writeError(w http.ResponseWriter, status int, code, message string, details map[string]any) {
	if details == nil {
		details = map[string]any{}
	}
	writeJSON(w, status, errorResponse{Error: errorBody{Code: code, Message: message, Details: details}})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeSSE(w http.ResponseWriter, event events.Event) {
	if event.ID == "" {
		event = events.Event{
			ID:          "evt_inline",
			WorkspaceID: event.WorkspaceID,
			Type:        event.Type,
			CreatedAt:   event.CreatedAt,
			Payload:     event.Payload,
		}
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}
	data, err := json.Marshal(event)
	if err != nil {
		return
	}
	_, _ = fmt.Fprintf(w, "id: %s\n", event.ID)
	_, _ = fmt.Fprintf(w, "event: %s\n", event.Type)
	_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
}
