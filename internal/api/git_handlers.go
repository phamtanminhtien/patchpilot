package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/phamtanminhtien/patchpilot/internal/gitrepo"
)

type gitStageRequest struct {
	Paths []string `json:"paths"`
}

type gitStagePatchRequest struct {
	Direction gitrepo.ApplyDirection `json:"direction"`
	Patch     string                 `json:"patch"`
}

type gitCommitRequest struct {
	Message string   `json:"message"`
	Paths   []string `json:"paths"`
}

type gitSwitchBranchRequest struct {
	Branch string `json:"branch"`
}

func (s *Server) gitBranches(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.workspaceFromRequest(w, r)
	if !ok {
		return
	}
	branches, err := s.git.Branches(r.Context(), ws.RootPath)
	if err != nil {
		writeGitError(w, err, "git_branch_list_failed", "Git branches could not be listed")
		return
	}
	writeJSON(w, http.StatusOK, branches)
}

func (s *Server) gitSwitchBranch(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.workspaceFromRequest(w, r)
	if !ok {
		return
	}
	var req gitSwitchBranchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Request body must be valid JSON", nil)
		return
	}
	status, err := s.git.SwitchBranch(r.Context(), ws.RootPath, req.Branch)
	if err != nil {
		writeGitError(w, err, "git_branch_switch_failed", "Git branch switch failed")
		return
	}
	s.publishGitChanged(r.Context(), ws)
	writeJSON(w, http.StatusOK, status)
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
		writeGitError(w, err, "git_state_failed", "Git status failed")
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
		writeGitError(w, err, "git_comparison_failed", "Git diff failed")
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

func (s *Server) gitStagePatch(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.workspaceFromRequest(w, r)
	if !ok {
		return
	}
	var req gitStagePatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Request body must be valid JSON", nil)
		return
	}
	direction := req.Direction
	if direction == "" {
		direction = gitrepo.ApplyForward
	}
	if direction != gitrepo.ApplyForward && direction != gitrepo.ApplyReverse {
		writeError(w, http.StatusBadRequest, "invalid_patch_direction", "Patch direction must be forward or reverse", nil)
		return
	}
	status, err := s.git.StagePatch(r.Context(), ws.RootPath, req.Patch, direction)
	if err != nil {
		writeGitError(w, err, "git_stage_patch_failed", "Git patch staging failed")
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
