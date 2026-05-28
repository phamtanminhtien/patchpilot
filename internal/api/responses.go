package api

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/phamtanminhtien/patchpilot/internal/agent"
	"github.com/phamtanminhtien/patchpilot/internal/database"
	"github.com/phamtanminhtien/patchpilot/internal/filestore"
	"github.com/phamtanminhtien/patchpilot/internal/gitrepo"
	"github.com/phamtanminhtien/patchpilot/internal/workspace"
)

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
	HasRunningRun bool      `json:"hasRunningRun"`
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

type terminalSessionResponse struct {
	ID          string     `json:"id"`
	WorkspaceID string     `json:"workspaceId"`
	Title       string     `json:"title"`
	Cwd         string     `json:"cwd"`
	Status      string     `json:"status"`
	PID         *int       `json:"pid"`
	Rows        int        `json:"rows"`
	Cols        int        `json:"cols"`
	ExitCode    *int       `json:"exitCode"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
	ClosedAt    *time.Time `json:"closedAt"`
}

func conversationResponseFromRecord(record database.ConversationRecord) conversationResponse {
	return conversationResponse{
		ID:            record.ID,
		WorkspaceID:   record.WorkspaceID,
		Title:         record.Title,
		HasRunningRun: record.HasRunningRun,
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

func terminalSessionResponseFromRecord(record database.TerminalSessionRecord) terminalSessionResponse {
	return terminalSessionResponse{
		ID:          record.ID,
		WorkspaceID: record.WorkspaceID,
		Title:       record.Title,
		Cwd:         record.Cwd,
		Status:      record.Status,
		PID:         record.PID,
		Rows:        record.Rows,
		Cols:        record.Cols,
		ExitCode:    record.ExitCode,
		CreatedAt:   record.CreatedAt,
		UpdatedAt:   record.UpdatedAt,
		ClosedAt:    record.ClosedAt,
	}
}

func terminalSessionResponsesFromRecords(records []database.TerminalSessionRecord) []terminalSessionResponse {
	sessions := make([]terminalSessionResponse, 0, len(records))
	for _, record := range records {
		sessions = append(sessions, terminalSessionResponseFromRecord(record))
	}
	return sessions
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
	case errors.Is(err, gitrepo.ErrEmptyBranchName):
		writeError(w, http.StatusBadRequest, "empty_branch_name", "Branch name is required", nil)
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
	case errors.Is(err, filestore.ErrSecretPath):
		writeError(w, http.StatusBadRequest, "secret_path", "Secret-like paths cannot be written", nil)
	case errors.Is(err, filestore.ErrSymlinkPath):
		writeError(w, http.StatusBadRequest, "symlink_path", "Symlink paths cannot be written", nil)
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

const (
	defaultPageLimit = 50
	maxPageLimit     = 100
)

type paginationRequest struct {
	Limit  int
	Cursor string
}

func paginationFromRequest(w http.ResponseWriter, r *http.Request) (paginationRequest, bool) {
	limit := defaultPageLimit
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > maxPageLimit {
			writeError(w, http.StatusBadRequest, "invalid_limit", "Limit must be between 1 and 100", nil)
			return paginationRequest{}, false
		}
		limit = parsed
	}
	return paginationRequest{Limit: limit, Cursor: strings.TrimSpace(r.URL.Query().Get("cursor"))}, true
}

func paginateItems[T any](w http.ResponseWriter, items []T, pagination paginationRequest, key func(T) string) ([]T, *string, bool) {
	start := 0
	if pagination.Cursor != "" {
		cursorKey, err := decodeCursor(pagination.Cursor)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_cursor", "Cursor is invalid", nil)
			return nil, nil, false
		}
		found := false
		for index, item := range items {
			if key(item) == cursorKey {
				start = index + 1
				found = true
				break
			}
		}
		if !found {
			writeError(w, http.StatusBadRequest, "invalid_cursor", "Cursor is invalid", nil)
			return nil, nil, false
		}
	}
	if start >= len(items) {
		return []T{}, nil, true
	}
	end := start + pagination.Limit
	if end > len(items) {
		end = len(items)
	}
	var nextCursor *string
	if end < len(items) {
		cursor := encodeCursor(key(items[end-1]))
		nextCursor = &cursor
	}
	return items[start:end], nextCursor, true
}

func encodeCursor(value string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(value))
}

func decodeCursor(value string) (string, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return "", err
	}
	if len(decoded) == 0 {
		return "", errors.New("empty cursor")
	}
	return string(decoded), nil
}
