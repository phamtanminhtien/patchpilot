package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/phamtanminhtien/patchpilot/internal/agent"
	"github.com/phamtanminhtien/patchpilot/internal/auth"
	"github.com/phamtanminhtien/patchpilot/internal/database"
	"github.com/phamtanminhtien/patchpilot/internal/events"
	"github.com/phamtanminhtien/patchpilot/internal/filestore"
	"github.com/phamtanminhtien/patchpilot/internal/gitrepo"
	"github.com/phamtanminhtien/patchpilot/internal/ports"
	"github.com/phamtanminhtien/patchpilot/internal/runner"
	"github.com/phamtanminhtien/patchpilot/internal/workspace"
)

type Server struct {
	workspaces  *workspace.Manager
	files       *filestore.Service
	git         *gitrepo.Client
	runner      *runner.Runner
	store       *database.Store
	events      *events.Hub
	agent       *agent.Manager
	auth        *auth.Service
	health      HealthChecker
	ports       ports.ListenerScanner
	backendAddr string
}

type HealthChecker interface {
	Ping(context.Context) error
}

func NewServer(workspaces *workspace.Manager, files *filestore.Service, git *gitrepo.Client, runner *runner.Runner, store *database.Store, hub *events.Hub, agentManager *agent.Manager, health HealthChecker) *Server {
	return NewServerWithAuth(workspaces, files, git, runner, store, hub, agentManager, nil, health)
}

func NewServerWithAuth(workspaces *workspace.Manager, files *filestore.Service, git *gitrepo.Client, runner *runner.Runner, store *database.Store, hub *events.Hub, agentManager *agent.Manager, authService *auth.Service, health HealthChecker) *Server {
	if hub == nil {
		hub = events.NewHub()
	}
	return &Server{workspaces: workspaces, files: files, git: git, runner: runner, store: store, events: hub, agent: agentManager, auth: authService, health: health, ports: ports.NewListenerScanner()}
}

func (s *Server) SetBackendAddr(addr string) {
	s.backendAddr = strings.TrimSpace(addr)
}

func (s *Server) Routes() http.Handler {
	return s.RoutesWithStatic("")
}

func (s *Server) RoutesWithStatic(staticDir string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", s.healthCheck)
	mux.HandleFunc("POST /api/auth/login", s.login)
	mux.Handle("GET /api/auth/session", s.requireAuth(http.HandlerFunc(s.getSession)))
	mux.Handle("POST /api/auth/logout", s.requireAuth(http.HandlerFunc(s.logout)))
	mux.HandleFunc("POST /api/workspaces", s.createWorkspace)
	mux.HandleFunc("GET /api/workspaces", s.listWorkspaces)
	mux.HandleFunc("GET /api/workspaces/{workspaceId}", s.getWorkspace)
	mux.HandleFunc("DELETE /api/workspaces/{workspaceId}", s.deleteWorkspace)
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
	mux.HandleFunc("POST /api/workspaces/{workspaceId}/conversations", s.createConversation)
	mux.HandleFunc("GET /api/workspaces/{workspaceId}/conversations", s.listConversations)
	mux.HandleFunc("GET /api/workspaces/{workspaceId}/conversations/{conversationId}", s.getConversation)
	mux.HandleFunc("PATCH /api/workspaces/{workspaceId}/conversations/{conversationId}", s.updateConversation)
	mux.HandleFunc("POST /api/workspaces/{workspaceId}/conversations/{conversationId}/messages", s.createMessage)
	mux.HandleFunc("POST /api/workspaces/{workspaceId}/conversations/{conversationId}/runs/{runId}/cancel", s.cancelAgentRun)
	mux.HandleFunc("POST /api/workspaces/{workspaceId}/conversations/{conversationId}/runs/{runId}/tool-calls/{toolCallId}/approve", s.approveAgentToolCall)
	mux.HandleFunc("POST /api/workspaces/{workspaceId}/conversations/{conversationId}/runs/{runId}/tool-calls/{toolCallId}/reject", s.rejectAgentToolCall)
	mux.HandleFunc("POST /api/workspaces/{workspaceId}/commands", s.createCommand)
	mux.HandleFunc("GET /api/workspaces/{workspaceId}/processes", s.listProcesses)
	mux.HandleFunc("GET /api/workspaces/{workspaceId}/processes/{processId}", s.getProcess)
	mux.HandleFunc("POST /api/workspaces/{workspaceId}/processes/{processId}/stop", s.stopProcess)
	mux.HandleFunc("GET /api/workspaces/{workspaceId}/ports", s.listPorts)
	mux.HandleFunc("POST /api/workspaces/{workspaceId}/ports/{port}/expose", s.exposePort)
	mux.HandleFunc("GET /api/workspaces/{workspaceId}/events", s.workspaceEvents)
	mux.HandleFunc("GET /workspaces/{workspaceId}/ports/{port}/proxy/{path...}", s.proxyPort)
	if staticDir != "" {
		mux.HandleFunc("GET /", serveStatic(staticDir))
	}
	return s.authenticatedRoutes(mux)
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

func (s *Server) authenticatedRoutes(next http.Handler) http.Handler {
	if s.auth == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isPublicRoute(r) || (!strings.HasPrefix(r.URL.Path, "/api/") && !strings.HasPrefix(r.URL.Path, "/workspaces/")) {
			next.ServeHTTP(w, r)
			return
		}
		s.requireAuth(next).ServeHTTP(w, r)
	})
}

func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.auth == nil {
			next.ServeHTTP(w, r)
			return
		}
		if _, err := s.auth.ValidateRequest(r.Context(), r, time.Now().UTC()); err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication is required", nil)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func isPublicRoute(r *http.Request) bool {
	return (r.Method == http.MethodGet && r.URL.Path == "/api/health") ||
		(r.Method == http.MethodPost && r.URL.Path == "/api/auth/login")
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

type conversationRequest struct {
	Title string `json:"title"`
}

type createMessageRequest struct {
	Content         string `json:"content"`
	Model           string `json:"model"`
	ReasoningEffort string `json:"reasoningEffort"`
}

type loginRequest struct {
	Token string `json:"token"`
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	if s.auth == nil {
		writeError(w, http.StatusServiceUnavailable, "auth_unavailable", "Authentication is unavailable", nil)
		return
	}
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Request body must be valid JSON", nil)
		return
	}
	rawToken, session, err := s.auth.Login(r.Context(), req.Token, time.Now().UTC())
	if err != nil {
		if errors.Is(err, auth.ErrInvalidToken) {
			writeError(w, http.StatusUnauthorized, "invalid_auth_token", "Invalid admin token", nil)
			return
		}
		writeError(w, http.StatusInternalServerError, "login_failed", "Login failed", nil)
		return
	}
	auth.SetSessionCookie(w, rawToken, session.ExpiresAt, auth.SecureCookie(r))
	writeJSON(w, http.StatusOK, map[string]any{"session": session})
}

func (s *Server) getSession(w http.ResponseWriter, r *http.Request) {
	if s.auth == nil {
		writeError(w, http.StatusServiceUnavailable, "auth_unavailable", "Authentication is unavailable", nil)
		return
	}
	session, err := s.auth.ValidateRequest(r.Context(), r, time.Now().UTC())
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication is required", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"session": session})
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if s.auth == nil {
		writeError(w, http.StatusServiceUnavailable, "auth_unavailable", "Authentication is unavailable", nil)
		return
	}
	if err := s.auth.Logout(r.Context(), r); err != nil {
		writeError(w, http.StatusInternalServerError, "logout_failed", "Logout failed", nil)
		return
	}
	auth.ClearSessionCookie(w, auth.SecureCookie(r))
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
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
	s.publishWorkspaceState(ws.ID, "workspace.indexing", ws)
	if err := s.refreshWorkspaceIndex(r.Context(), ws); err != nil {
		writeIndexRefreshError(w, err)
		return
	}
	s.publishWorkspaceState(ws.ID, "workspace.ready", ws)
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
	s.publishWorkspaceState(ws.ID, "workspace.indexing", ws)
	if err := s.refreshWorkspaceIndex(r.Context(), ws); err != nil {
		writeIndexRefreshError(w, err)
		return
	}
	s.publishWorkspaceState(ws.ID, "workspace.ready", ws)
	writeJSON(w, http.StatusOK, ws)
}

func (s *Server) deleteWorkspace(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.workspaceFromRequest(w, r)
	if !ok {
		return
	}
	if err := s.workspaces.Delete(r.Context(), ws.ID); err != nil {
		writeWorkspaceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
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
	s.publishWorkspaceState(ws.ID, "workspace.indexing", ws)
	if err := s.refreshWorkspaceIndex(r.Context(), ws); err != nil {
		writeIndexRefreshError(w, err)
		return
	}
	s.publishWorkspaceState(ws.ID, "workspace.ready", ws)
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
	opts := gitrepo.StatusOptions{}
	if val := r.URL.Query().Get("ignored"); val != "" {
		if b, err := strconv.ParseBool(val); err == nil {
			opts.Ignored = b
		}
	}
	if val := r.URL.Query().Get("untracked"); val != "" {
		opts.Untracked = val
	}
	if val := r.URL.Query().Get("ignore_submodules"); val != "" {
		opts.IgnoreSubmodules = val
	}
	var paths []string
	for _, key := range []string{"paths", "path"} {
		if vals, exists := r.URL.Query()[key]; exists {
			for _, val := range vals {
				for _, p := range strings.Split(val, ",") {
					p = strings.TrimSpace(p)
					if p != "" {
						paths = append(paths, p)
					}
				}
			}
		}
	}
	opts.Paths = paths

	status, err := s.git.Status(r.Context(), ws.RootPath, opts)
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
	s.publishGitChanged(r.Context(), ws)
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
	s.publishGitChanged(r.Context(), ws)
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
	s.publishGitChanged(r.Context(), ws)
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
	s.publishGitChanged(r.Context(), ws)
	writeJSON(w, http.StatusOK, commit)
}

func (s *Server) createConversation(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.workspaceFromRequest(w, r)
	if !ok {
		return
	}
	var req conversationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Request body must be valid JSON", nil)
		return
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = "New conversation"
	}
	conversation, err := s.store.CreateConversation(r.Context(), database.ConversationRecord{
		WorkspaceID: ws.ID,
		Title:       title,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "conversation_create_failed", "Conversation could not be created", nil)
		return
	}
	writeJSON(w, http.StatusCreated, conversationResponseFromRecord(conversation))
}

func (s *Server) listConversations(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.workspaceFromRequest(w, r)
	if !ok {
		return
	}
	conversations, err := s.store.ListConversations(r.Context(), ws.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "conversation_list_failed", "Conversations could not be listed", nil)
		return
	}
	out := make([]conversationResponse, 0, len(conversations))
	for _, conversation := range conversations {
		out = append(out, conversationResponseFromRecord(conversation))
	}
	writeJSON(w, http.StatusOK, map[string]any{"conversations": out})
}

func (s *Server) getConversation(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.workspaceFromRequest(w, r)
	if !ok {
		return
	}
	detail, err := s.conversationDetail(r.Context(), ws.ID, r.PathValue("conversationId"))
	if err != nil {
		writeConversationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (s *Server) updateConversation(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.workspaceFromRequest(w, r)
	if !ok {
		return
	}
	var req conversationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Request body must be valid JSON", nil)
		return
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		writeError(w, http.StatusBadRequest, "invalid_conversation_title", "Conversation title is required", nil)
		return
	}
	conversation, err := s.store.UpdateConversation(r.Context(), ws.ID, r.PathValue("conversationId"), map[string]any{"title": title})
	if err != nil {
		writeConversationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, conversationResponseFromRecord(conversation))
}

func (s *Server) createMessage(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.workspaceFromRequest(w, r)
	if !ok {
		return
	}
	if s.agent == nil {
		writeError(w, http.StatusServiceUnavailable, "agent_unavailable", "Agent runtime is unavailable", nil)
		return
	}
	conversationID := r.PathValue("conversationId")
	if _, err := s.store.GetConversation(r.Context(), ws.ID, conversationID); err != nil {
		writeConversationError(w, err)
		return
	}
	var req createMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Request body must be valid JSON", nil)
		return
	}
	message, err := s.store.CreateMessage(r.Context(), database.MessageRecord{
		WorkspaceID:    ws.ID,
		ConversationID: conversationID,
		Role:           "user",
		Content:        strings.TrimSpace(req.Content),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "message_create_failed", "Message could not be created", nil)
		return
	}
	run, err := s.agent.Create(r.Context(), ws.ID, ws.RootPath, agent.CreateRunInput{
		Prompt:           message.Content,
		ConversationID:   conversationID,
		TriggerMessageID: message.ID,
		Model:            req.Model,
		ReasoningEffort:  req.ReasoningEffort,
	})
	if err != nil {
		writeAgentError(w, err)
		return
	}
	updatedMessage, err := s.store.UpdateMessageRun(r.Context(), ws.ID, conversationID, message.ID, run.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "message_update_failed", "Message could not be linked to the run", nil)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"message": messageResponseFromRecord(updatedMessage), "run": run})
}

func (s *Server) cancelAgentRun(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.workspaceFromRequest(w, r)
	if !ok {
		return
	}
	if s.agent == nil {
		writeError(w, http.StatusServiceUnavailable, "agent_unavailable", "Agent runtime is unavailable", nil)
		return
	}
	run, err := s.agent.Cancel(r.Context(), ws.ID, r.PathValue("conversationId"), r.PathValue("runId"))
	if err != nil {
		writeAgentError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, run)
}

func (s *Server) approveAgentToolCall(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.workspaceFromRequest(w, r)
	if !ok {
		return
	}
	if s.agent == nil {
		writeError(w, http.StatusServiceUnavailable, "agent_unavailable", "Agent runtime is unavailable", nil)
		return
	}
	call, err := s.agent.ApproveToolCall(r.Context(), ws.ID, r.PathValue("runId"), r.PathValue("toolCallId"))
	if err != nil {
		writeAgentError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"toolCall": call})
}

func (s *Server) rejectAgentToolCall(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.workspaceFromRequest(w, r)
	if !ok {
		return
	}
	if s.agent == nil {
		writeError(w, http.StatusServiceUnavailable, "agent_unavailable", "Agent runtime is unavailable", nil)
		return
	}
	call, err := s.agent.RejectToolCall(r.Context(), ws.ID, r.PathValue("runId"), r.PathValue("toolCallId"))
	if err != nil {
		writeAgentError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"toolCall": call})
}

func (s *Server) conversationDetail(ctx context.Context, workspaceID, conversationID string) (map[string]any, error) {
	conversation, err := s.store.GetConversation(ctx, workspaceID, conversationID)
	if err != nil {
		return nil, err
	}
	messages, err := s.store.ListMessages(ctx, workspaceID, conversationID)
	if err != nil {
		return nil, err
	}
	runs, err := s.agent.List(ctx, workspaceID, conversationID)
	if err != nil {
		return nil, err
	}
	messageResponses := make([]messageResponse, 0, len(messages))
	for _, message := range messages {
		messageResponses = append(messageResponses, messageResponseFromRecord(message))
	}
	toolCalls := make([]agent.ToolCall, 0)
	runEvents := make([]agent.RunEvent, 0)
	for _, run := range runs {
		records, err := s.store.ListAgentToolCalls(ctx, workspaceID, run.ID)
		if err != nil {
			return nil, err
		}
		toolCalls = append(toolCalls, agent.ToolCallsFromRecords(records)...)
		if run.Status == string(agent.StatusDone) || run.Status == string(agent.StatusFailed) {
			continue
		}
		eventRecords, err := s.store.ListAgentRunEvents(ctx, workspaceID, run.ID)
		if err != nil {
			return nil, err
		}
		runEvents = append(runEvents, agent.EventsFromRecords(eventRecords)...)
	}
	return map[string]any{
		"conversation": conversationResponseFromRecord(conversation),
		"events":       runEvents,
		"messages":     messageResponses,
		"runs":         runs,
		"toolCalls":    toolCalls,
	}, nil
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

func (s *Server) listPorts(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.workspaceFromRequest(w, r)
	if !ok {
		return
	}
	s.refreshPortStates(r.Context(), ws.ID)
	records, err := s.store.ListPorts(r.Context(), ws.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "port_list_failed", "Ports could not be listed", nil)
		return
	}
	response := make([]portResponse, 0, len(records))
	for _, record := range records {
		response = append(response, s.portResponseFromRecord(r, record))
	}
	writeJSON(w, http.StatusOK, map[string]any{"ports": response})
}

func (s *Server) exposePort(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.workspaceFromRequest(w, r)
	if !ok {
		return
	}
	port, ok := portFromRequest(w, r)
	if !ok {
		return
	}
	if _, err := s.store.GetPort(r.Context(), ws.ID, port); err != nil {
		writePortError(w, err)
		return
	}
	if !ports.Reachable(r.Context(), port) {
		s.markPortClosed(r.Context(), ws.ID, port)
		writeError(w, http.StatusBadGateway, "port_unreachable", "Port is not accepting local connections", nil)
		return
	}
	exposedPath := fmt.Sprintf("/workspaces/%s/ports/%d/proxy/", ws.ID, port)
	record, err := s.store.ExposePort(r.Context(), ws.ID, port, exposedPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "port_expose_failed", "Port could not be exposed", nil)
		return
	}
	response := s.portResponseFromRecord(r, record)
	s.events.Publish(events.Event{WorkspaceID: ws.ID, Type: "port.exposed", Payload: response})
	writeJSON(w, http.StatusOK, map[string]any{"port": response})
}

func (s *Server) proxyPort(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.workspaceFromRequest(w, r)
	if !ok {
		return
	}
	port, ok := portFromRequest(w, r)
	if !ok {
		return
	}
	record, err := s.store.GetPort(r.Context(), ws.ID, port)
	if err != nil {
		writePortError(w, err)
		return
	}
	if record.Status != "exposed" {
		writeError(w, http.StatusConflict, "port_not_exposed", "Port is not exposed", nil)
		return
	}
	host, reachable := ports.ReachableHost(r.Context(), port)
	if !reachable {
		s.markPortClosed(r.Context(), ws.ID, port)
		writeError(w, http.StatusBadGateway, "port_unreachable", "Port is not accepting local connections", nil)
		return
	}
	prefix := fmt.Sprintf("/workspaces/%s/ports/%d/proxy", ws.ID, port)
	http.StripPrefix(prefix, ports.NewProxyForHost(host, port)).ServeHTTP(w, r)
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
	scanCtx, stopScanning := context.WithCancel(context.Background())
	return runner.Hooks{
		OnStarted: func(pid int) {
			startedAt := time.Now().UTC()
			command, err := s.store.MarkCommandRunning(context.Background(), workspaceID, commandID, startedAt)
			if err == nil {
				s.publishProcessStarted(command)
			}
			go s.pollListeningPorts(scanCtx, workspaceID, commandID, pid)
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
			stopScanning()
			finishedAt := time.Now().UTC()
			command, err := s.store.FinishCommand(context.Background(), workspaceID, commandID, result.Status, result.ExitCode, finishedAt)
			if err == nil {
				s.publishProcessExited(command)
			}
		},
	}
}

func (s *Server) pollListeningPorts(ctx context.Context, workspaceID, commandID string, pid int) {
	s.detectListeningPorts(ctx, workspaceID, commandID, pid)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.detectListeningPorts(ctx, workspaceID, commandID, pid)
		}
	}
}

func (s *Server) detectListeningPorts(ctx context.Context, workspaceID, commandID string, pid int) {
	detectedPorts, err := s.ports.ListeningPorts(ctx, pid)
	if err != nil {
		return
	}
	for _, detectedPort := range detectedPorts {
		processID := commandID
		record, created, err := s.store.UpsertDetectedPort(ctx, database.PortRecord{
			WorkspaceID: workspaceID,
			ProcessID:   &processID,
			Port:        detectedPort,
			Status:      "detected",
		})
		if err != nil {
			continue
		}
		eventType := "port.opened"
		if !created && record.Status == "exposed" {
			eventType = "port.exposed"
		}
		s.events.Publish(events.Event{
			WorkspaceID: workspaceID,
			Type:        eventType,
			Payload:     s.portResponseFromRecord(nil, record),
		})
	}
}

func (s *Server) markPortClosed(ctx context.Context, workspaceID string, port int) {
	record, err := s.store.MarkPortClosed(ctx, workspaceID, port, time.Now().UTC())
	if err != nil {
		return
	}
	s.events.Publish(events.Event{
		WorkspaceID: workspaceID,
		Type:        "port.closed",
		Payload:     s.portResponseFromRecord(nil, record),
	})
}

func (s *Server) refreshPortStates(ctx context.Context, workspaceID string) {
	records, err := s.store.ListPorts(ctx, workspaceID)
	if err != nil {
		return
	}
	for _, record := range records {
		if ports.Reachable(ctx, record.Port) {
			if record.ExposedPath != nil {
				updated, err := s.store.ExposePort(ctx, workspaceID, record.Port, *record.ExposedPath)
				if err == nil && record.Status != updated.Status {
					s.events.Publish(events.Event{WorkspaceID: workspaceID, Type: "port.exposed", Payload: s.portResponseFromRecord(nil, updated)})
				}
				continue
			}
			updated, _, err := s.store.UpsertDetectedPort(ctx, database.PortRecord{
				WorkspaceID: workspaceID,
				ProcessID:   record.ProcessID,
				Port:        record.Port,
				Status:      "detected",
			})
			if err == nil && record.Status != updated.Status {
				s.events.Publish(events.Event{WorkspaceID: workspaceID, Type: "port.opened", Payload: s.portResponseFromRecord(nil, updated)})
			}
			continue
		}
		if record.Status == "exposed" || record.Status == "detected" {
			s.markPortClosed(ctx, workspaceID, record.Port)
		}
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

func (s *Server) publishGitChanged(ctx context.Context, ws workspace.Workspace) {
	status, err := s.git.Status(ctx, ws.RootPath, gitrepo.StatusOptions{})
	if err != nil {
		return
	}
	s.events.Publish(events.Event{
		WorkspaceID: ws.ID,
		Type:        "git.changed",
		Payload:     status,
	})
}

func (s *Server) publishWorkspaceState(workspaceID, eventType string, ws workspace.Workspace) {
	s.events.Publish(events.Event{
		WorkspaceID: workspaceID,
		Type:        eventType,
		Payload:     ws,
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

type conversationResponse struct {
	ID            string    `json:"id"`
	WorkspaceID   string    `json:"workspaceId"`
	Title         string    `json:"title"`
	LastMessageAt time.Time `json:"lastMessageAt"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

type messageResponse struct {
	ID             string    `json:"id"`
	WorkspaceID    string    `json:"workspaceId"`
	ConversationID string    `json:"conversationId"`
	Role           string    `json:"role"`
	Content        string    `json:"content"`
	RunID          *string   `json:"runId"`
	CreatedAt      time.Time `json:"createdAt"`
}

type commandOutputResponse struct {
	ID        string    `json:"id"`
	CommandID string    `json:"commandId"`
	Stream    string    `json:"stream"`
	Chunk     string    `json:"chunk"`
	CreatedAt time.Time `json:"createdAt"`
}

type portResponse struct {
	ID          string     `json:"id"`
	WorkspaceID string     `json:"workspaceId"`
	ProcessID   *string    `json:"processId"`
	Port        int        `json:"port"`
	Status      string     `json:"status"`
	ExposedPath *string    `json:"exposedPath"`
	ExposedURL  *string    `json:"exposedUrl"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
	ClosedAt    *time.Time `json:"closedAt"`
}

func conversationResponseFromRecord(record database.ConversationRecord) conversationResponse {
	return conversationResponse{
		ID:            record.ID,
		WorkspaceID:   record.WorkspaceID,
		Title:         record.Title,
		LastMessageAt: record.LastMessageAt,
		CreatedAt:     record.CreatedAt,
		UpdatedAt:     record.UpdatedAt,
	}
}

func messageResponseFromRecord(record database.MessageRecord) messageResponse {
	return messageResponse{
		ID:             record.ID,
		WorkspaceID:    record.WorkspaceID,
		ConversationID: record.ConversationID,
		Role:           record.Role,
		Content:        record.Content,
		RunID:          record.RunID,
		CreatedAt:      record.CreatedAt,
	}
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

func (s *Server) portResponseFromRecord(r *http.Request, record database.PortRecord) portResponse {
	var exposedURL *string
	if record.ExposedPath != nil {
		url := s.exposedURL(r, *record.ExposedPath)
		exposedURL = &url
	}
	return portResponse{
		ID:          record.ID,
		WorkspaceID: record.WorkspaceID,
		ProcessID:   record.ProcessID,
		Port:        record.Port,
		Status:      record.Status,
		ExposedPath: record.ExposedPath,
		ExposedURL:  exposedURL,
		CreatedAt:   record.CreatedAt,
		UpdatedAt:   record.UpdatedAt,
		ClosedAt:    record.ClosedAt,
	}
}

func (s *Server) exposedURL(r *http.Request, path string) string {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return s.backendOrigin(r) + path
}

func (s *Server) backendOrigin(r *http.Request) string {
	scheme := "http"
	requestHost := ""
	if r != nil {
		requestHost = r.Host
	}
	if r != nil && r.TLS != nil {
		scheme = "https"
	}
	if r != nil {
		if proto := strings.ToLower(strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-Proto"), ",")[0])); proto == "http" || proto == "https" {
			scheme = proto
		}
	}
	host := requestHost
	if s.backendAddr != "" {
		host = backendHostForRequest(requestHost, s.backendAddr)
	}
	if host == "" {
		host = "127.0.0.1"
	}
	return scheme + "://" + host
}

func backendHostForRequest(requestHost, backendAddr string) string {
	backendHost, backendPort := splitHostPort(backendAddr)
	if backendPort == "" {
		return requestHost
	}
	if backendHost == "" || backendHost == "0.0.0.0" || backendHost == "::" {
		backendHost = hostName(requestHost)
	}
	if backendHost == "" {
		backendHost = "127.0.0.1"
	}
	return net.JoinHostPort(backendHost, backendPort)
}

func splitHostPort(value string) (string, string) {
	host, port, err := net.SplitHostPort(value)
	if err == nil {
		return strings.Trim(host, "[]"), port
	}
	if strings.HasPrefix(value, ":") {
		return "", strings.TrimPrefix(value, ":")
	}
	return value, ""
}

func hostName(value string) string {
	host, _, err := net.SplitHostPort(value)
	if err == nil {
		return strings.Trim(host, "[]")
	}
	return strings.Trim(value, "[]")
}

func portFromRequest(w http.ResponseWriter, r *http.Request) (int, bool) {
	port, err := strconv.Atoi(r.PathValue("port"))
	if err != nil || port < 1 || port > 65535 {
		writeError(w, http.StatusBadRequest, "invalid_port", "Port must be between 1 and 65535", nil)
		return 0, false
	}
	return port, true
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
		writeError(w, http.StatusInternalServerError, fallbackCode, fallbackMessage, map[string]any{"error": err.Error()})
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

func writePortError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, database.ErrNotFound):
		writeError(w, http.StatusNotFound, "port_not_found", "Port not found", nil)
	default:
		writeError(w, http.StatusInternalServerError, "port_request_failed", "Port request failed", nil)
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
	case errors.Is(err, agent.ErrRunNotFound):
		writeError(w, http.StatusNotFound, "agent_run_not_found", "Agent run not found", nil)
	case errors.Is(err, agent.ErrToolCallNotFound):
		writeError(w, http.StatusNotFound, "agent_tool_call_not_found", "Agent tool call not found", nil)
	case errors.Is(err, agent.ErrToolNotApprovable):
		writeError(w, http.StatusConflict, "agent_tool_not_approvable", "Agent tool call is not waiting for approval", nil)
	case errors.Is(err, agent.ErrRunNotResumable):
		writeError(w, http.StatusConflict, "agent_run_not_resumable", "Agent run cannot resume after server restart", nil)
	default:
		writeError(w, http.StatusInternalServerError, "agent_run_failed", "Agent run request failed", nil)
	}
}

func writeConversationError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, database.ErrNotFound):
		writeError(w, http.StatusNotFound, "conversation_not_found", "Conversation not found", nil)
	default:
		writeError(w, http.StatusInternalServerError, "conversation_request_failed", "Conversation request failed", nil)
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
