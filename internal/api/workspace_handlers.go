package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/phamtanminhtien/patchpilot/internal/filestore"
	"github.com/phamtanminhtien/patchpilot/internal/workspace"
)

type createWorkspaceRequest struct {
	RootPath string `json:"rootPath"`
}

type writeFileRequest struct {
	Path    string `json:"path"`
	Content string `json:"content"`
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
	pagination, ok := paginationFromRequest(w, r)
	if !ok {
		return
	}
	workspaces, err := s.workspaces.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "workspace_list_failed", "Workspaces could not be listed", nil)
		return
	}
	page, nextCursor, ok := paginateItems(w, workspaces, pagination, func(ws workspace.Workspace) string {
		return ws.ID
	})
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"workspaces": page, "nextCursor": nextCursor})
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

func (s *Server) writeFile(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.workspaceFromRequest(w, r)
	if !ok {
		return
	}
	var req writeFileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Request body must be valid JSON", nil)
		return
	}
	file, err := s.files.Write(ws.RootPath, req.Path, req.Content)
	if err != nil {
		writeFileError(w, err)
		return
	}
	if err := s.refreshWorkspaceIndex(r.Context(), ws); err != nil {
		writeIndexRefreshError(w, err)
		return
	}
	s.publishGitChanged(r.Context(), ws)
	writeJSON(w, http.StatusOK, file)
}

func (s *Server) listFileIndex(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.workspaceFromRequest(w, r)
	if !ok {
		return
	}
	pagination, ok := paginationFromRequest(w, r)
	if !ok {
		return
	}
	entries, err := s.workspaces.FileIndex(r.Context(), ws.ID)
	if err != nil {
		writeWorkspaceError(w, err)
		return
	}
	page, nextCursor, ok := paginateItems(w, entries, pagination, func(entry workspace.FileIndexEntry) string {
		return entry.Path
	})
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"entries": page, "nextCursor": nextCursor})
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
	pagination, ok := paginationFromRequest(w, r)
	if !ok {
		return
	}
	results, err := s.files.Search(ws.RootPath, r.URL.Query().Get("q"))
	if err != nil {
		writeFileError(w, err)
		return
	}
	page, nextCursor, ok := paginateItems(w, results, pagination, func(result filestore.SearchResult) string {
		return fmt.Sprintf("%s\x00%s\x00%d", result.Path, result.Kind, result.Line)
	})
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": page, "nextCursor": nextCursor})
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

func (s *Server) workspaceFromRequest(w http.ResponseWriter, r *http.Request) (workspace.Workspace, bool) {
	ws, err := s.workspaces.Get(r.Context(), r.PathValue("workspaceId"))
	if err != nil {
		writeError(w, http.StatusNotFound, "workspace_not_found", "Workspace not found", nil)
		return workspace.Workspace{}, false
	}
	return ws, true
}
