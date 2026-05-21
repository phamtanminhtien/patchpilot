package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"

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
	health     HealthChecker
}

type HealthChecker interface {
	Ping(context.Context) error
}

func NewServer(workspaces *workspace.Manager, files *filestore.Service, git *gitrepo.Client, runner *runner.Runner, health HealthChecker) *Server {
	return &Server{workspaces: workspaces, files: files, git: git, runner: runner, health: health}
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
	mux.HandleFunc("POST /api/workspaces/{workspaceId}/commands", s.createCommand)
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
	Command string `json:"command"`
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

func (s *Server) createCommand(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.workspaceFromRequest(w, r); !ok {
		return
	}
	var req createCommandRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Request body must be valid JSON", nil)
		return
	}
	command, err := s.runner.Queue(req.Command)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_command", "Command is required", nil)
		return
	}
	writeJSON(w, http.StatusAccepted, command)
}

func (s *Server) workspaceFromRequest(w http.ResponseWriter, r *http.Request) (workspace.Workspace, bool) {
	ws, err := s.workspaces.Get(r.Context(), r.PathValue("workspaceId"))
	if err != nil {
		writeError(w, http.StatusNotFound, "workspace_not_found", "Workspace not found", nil)
		return workspace.Workspace{}, false
	}
	return ws, true
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
