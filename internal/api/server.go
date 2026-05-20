package api

import (
	"encoding/json"
	"errors"
	"net/http"

	fileapi "github.com/phamtanminhtien/patchpilot/internal/files"
	gitapi "github.com/phamtanminhtien/patchpilot/internal/git"
	"github.com/phamtanminhtien/patchpilot/internal/runner"
	"github.com/phamtanminhtien/patchpilot/internal/workspace"
)

type Server struct {
	workspaces *workspace.Manager
	files      *fileapi.Service
	git        *gitapi.Client
	runner     *runner.Runner
}

func NewServer(workspaces *workspace.Manager, files *fileapi.Service, git *gitapi.Client, runner *runner.Runner) *Server {
	return &Server{workspaces: workspaces, files: files, git: git, runner: runner}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/workspaces", s.createWorkspace)
	mux.HandleFunc("GET /api/workspaces", s.listWorkspaces)
	mux.HandleFunc("GET /api/workspaces/{workspaceId}", s.getWorkspace)
	mux.HandleFunc("GET /api/workspaces/{workspaceId}/files", s.listFiles)
	mux.HandleFunc("GET /api/workspaces/{workspaceId}/file", s.readFile)
	mux.HandleFunc("GET /api/workspaces/{workspaceId}/git/status", s.gitStatus)
	mux.HandleFunc("GET /api/workspaces/{workspaceId}/git/diff", s.gitDiff)
	mux.HandleFunc("POST /api/workspaces/{workspaceId}/commands", s.createCommand)
	return mux
}

type createWorkspaceRequest struct {
	RootPath string `json:"rootPath"`
}

type createCommandRequest struct {
	Command string `json:"command"`
}

func (s *Server) createWorkspace(w http.ResponseWriter, r *http.Request) {
	var req createWorkspaceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Request body must be valid JSON", nil)
		return
	}
	ws, err := s.workspaces.Create(req.RootPath)
	if err != nil {
		writeWorkspaceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, ws)
}

func (s *Server) listWorkspaces(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"workspaces": s.workspaces.List()})
}

func (s *Server) getWorkspace(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.workspaceFromRequest(w, r)
	if !ok {
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

func (s *Server) gitStatus(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.workspaceFromRequest(w, r)
	if !ok {
		return
	}
	status, err := s.git.Status(r.Context(), ws.RootPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "git_status_failed", "Git status failed", nil)
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
		if errors.Is(err, gitapi.ErrInvalidPath) {
			writeError(w, http.StatusBadRequest, "invalid_path", "Path must be workspace-relative", nil)
			return
		}
		writeError(w, http.StatusInternalServerError, "git_diff_failed", "Git diff failed", nil)
		return
	}
	writeJSON(w, http.StatusOK, diff)
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
	ws, err := s.workspaces.Get(r.PathValue("workspaceId"))
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

func writeFileError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, fileapi.ErrInvalidPath):
		writeError(w, http.StatusBadRequest, "invalid_path", "Path must be workspace-relative", nil)
	case errors.Is(err, fileapi.ErrOutsideRoot):
		writeError(w, http.StatusBadRequest, "path_outside_workspace", "Path escapes workspace root", nil)
	case errors.Is(err, fileapi.ErrNotTextFile):
		writeError(w, http.StatusBadRequest, "not_text_file", "File is not a readable text file", nil)
	default:
		writeError(w, http.StatusNotFound, "path_not_found", "Path not found", nil)
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
