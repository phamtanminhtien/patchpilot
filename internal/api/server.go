package api

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/phamtanminhtien/patchpilot/internal/agent"
	"github.com/phamtanminhtien/patchpilot/internal/auth"
	"github.com/phamtanminhtien/patchpilot/internal/database"
	"github.com/phamtanminhtien/patchpilot/internal/events"
	"github.com/phamtanminhtien/patchpilot/internal/filestore"
	"github.com/phamtanminhtien/patchpilot/internal/gitrepo"
	"github.com/phamtanminhtien/patchpilot/internal/ports"
	"github.com/phamtanminhtien/patchpilot/internal/runner"
	terminalsvc "github.com/phamtanminhtien/patchpilot/internal/terminal"
	"github.com/phamtanminhtien/patchpilot/internal/workspace"
)

type Server struct {
	workspaces       *workspace.Manager
	files            *filestore.Service
	git              *gitrepo.Client
	runner           *runner.Runner
	terminal         *terminalsvc.Manager
	store            *database.Store
	events           *events.Hub
	agent            *agent.Manager
	auth             *auth.Service
	health           HealthChecker
	ports            ports.ListenerScanner
	backendAddr      string
	lightModel       string
	settingsHome     string
	providerReady    bool
	openAIBaseURL    string
	allowedRootCount int
	logFormat        string
	staticDirReady   bool
	terminalScansMu  sync.Mutex
	terminalScans    map[string]context.CancelFunc
}

const (
	defaultConversationTitle = "New conversation"
	defaultLightModel        = "gpt-5.4-mini"
	titleGenerationTimeout   = 20 * time.Second
)

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
	s := &Server{workspaces: workspaces, files: files, git: git, runner: runner, store: store, events: hub, agent: agentManager, auth: authService, health: health, ports: ports.NewListenerScanner(), terminalScans: map[string]context.CancelFunc{}}
	s.terminal = terminalsvc.NewManager(store, s.onTerminalClosed)
	return s
}

func (s *Server) SetBackendAddr(addr string) {
	s.backendAddr = strings.TrimSpace(addr)
}

func (s *Server) SetLightModel(model string) {
	s.lightModel = strings.TrimSpace(model)
}

func (s *Server) SetSettingsHome(home string) {
	s.settingsHome = strings.TrimSpace(home)
}

func (s *Server) SetRuntimeConfigStatus(providerReady bool, openAIBaseURL string, allowedRootCount int, logFormat string, staticDirReady bool) {
	s.providerReady = providerReady
	s.openAIBaseURL = strings.TrimSpace(openAIBaseURL)
	s.allowedRootCount = allowedRootCount
	s.logFormat = strings.TrimSpace(logFormat)
	s.staticDirReady = staticDirReady
}

func (s *Server) Shutdown(ctx context.Context, reason string) error {
	if s.agent != nil {
		if err := s.agent.Shutdown(ctx, reason); err != nil {
			return err
		}
	}
	if s.terminal != nil {
		if err := s.terminal.CloseAll(ctx); err != nil {
			return err
		}
	}
	commands, err := s.store.ListActiveCommands(ctx)
	if err != nil {
		return err
	}
	for _, command := range commands {
		if err := s.stopCommandForShutdown(ctx, command); err != nil {
			return err
		}
	}
	return nil
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
	mux.HandleFunc("GET /api/settings", s.getSettings)
	mux.HandleFunc("PATCH /api/settings/preferences", s.patchSettingsPreferences)
	mux.HandleFunc("GET /api/settings/fonts", s.listSettingsFonts)
	mux.HandleFunc("POST /api/settings/fonts", s.createSettingsFont)
	mux.HandleFunc("GET /api/settings/fonts/{fontId}/file", s.getSettingsFontFile)
	mux.HandleFunc("DELETE /api/settings/fonts/{fontId}", s.deleteSettingsFont)
	mux.HandleFunc("POST /api/workspaces", s.createWorkspace)
	mux.HandleFunc("GET /api/workspaces", s.listWorkspaces)
	mux.HandleFunc("GET /api/workspaces/{workspaceId}", s.getWorkspace)
	mux.HandleFunc("GET /api/workspaces/{workspaceId}/permissions", s.getWorkspacePermissions)
	mux.HandleFunc("PATCH /api/workspaces/{workspaceId}/permissions", s.patchWorkspacePermissions)
	mux.HandleFunc("DELETE /api/workspaces/{workspaceId}", s.deleteWorkspace)
	mux.HandleFunc("GET /api/workspaces/{workspaceId}/files", s.listFiles)
	mux.HandleFunc("GET /api/workspaces/{workspaceId}/files/index", s.listFileIndex)
	mux.HandleFunc("POST /api/workspaces/{workspaceId}/files/index/refresh", s.refreshFileIndex)
	mux.HandleFunc("GET /api/workspaces/{workspaceId}/file", s.readFile)
	mux.HandleFunc("PUT /api/workspaces/{workspaceId}/file", s.writeFile)
	mux.HandleFunc("GET /api/workspaces/{workspaceId}/search", s.searchFiles)
	mux.HandleFunc("GET /api/workspaces/{workspaceId}/git/branches", s.gitBranches)
	mux.HandleFunc("POST /api/workspaces/{workspaceId}/git/branch", s.gitSwitchBranch)
	mux.HandleFunc("GET /api/workspaces/{workspaceId}/git/status", s.gitStatus)
	mux.HandleFunc("GET /api/workspaces/{workspaceId}/git/diff", s.gitDiff)
	mux.HandleFunc("POST /api/workspaces/{workspaceId}/git/stage", s.gitStage)
	mux.HandleFunc("POST /api/workspaces/{workspaceId}/git/stage-patch", s.gitStagePatch)
	mux.HandleFunc("POST /api/workspaces/{workspaceId}/git/unstage", s.gitUnstage)
	mux.HandleFunc("POST /api/workspaces/{workspaceId}/git/discard", s.gitDiscard)
	mux.HandleFunc("POST /api/workspaces/{workspaceId}/git/commit", s.gitCommit)
	mux.HandleFunc("POST /api/workspaces/{workspaceId}/conversations", s.createConversation)
	mux.HandleFunc("GET /api/workspaces/{workspaceId}/conversations", s.listConversations)
	mux.HandleFunc("GET /api/workspaces/{workspaceId}/conversations/{conversationId}", s.getConversation)
	mux.HandleFunc("PATCH /api/workspaces/{workspaceId}/conversations/{conversationId}", s.updateConversation)
	mux.HandleFunc("POST /api/workspaces/{workspaceId}/conversations/{conversationId}/messages", s.createMessage)
	mux.HandleFunc("POST /api/workspaces/{workspaceId}/conversations/{conversationId}/runs/{runId}/cancel", s.cancelAgentRun)
	mux.HandleFunc("GET /api/workspaces/{workspaceId}/conversations/{conversationId}/runs/{runId}/events", s.agentRunEvents)
	mux.HandleFunc("POST /api/workspaces/{workspaceId}/conversations/{conversationId}/runs/{runId}/tool-calls/{toolCallId}/approve", s.approveAgentToolCall)
	mux.HandleFunc("POST /api/workspaces/{workspaceId}/conversations/{conversationId}/runs/{runId}/tool-calls/{toolCallId}/reject", s.rejectAgentToolCall)
	mux.HandleFunc("GET /api/workspaces/{workspaceId}/agent/context", s.getAgentContext)
	mux.HandleFunc("POST /api/workspaces/{workspaceId}/agent/context/refresh", s.getAgentContext)
	mux.HandleFunc("GET /api/workspaces/{workspaceId}/skills", s.listAgentSkills)
	mux.HandleFunc("PATCH /api/workspaces/{workspaceId}/skills/{skillKey}", s.patchAgentSkill)
	mux.HandleFunc("POST /api/workspaces/{workspaceId}/skills/refresh", s.listAgentSkills)
	mux.HandleFunc("GET /api/workspaces/{workspaceId}/mcp/servers", s.listMCPServers)
	mux.HandleFunc("GET /api/workspaces/{workspaceId}/mcp/servers/{serverId}/tools", s.listMCPServerTools)
	mux.HandleFunc("GET /api/workspaces/{workspaceId}/terminal/sessions", s.listTerminalSessions)
	mux.HandleFunc("POST /api/workspaces/{workspaceId}/terminal/sessions", s.createTerminalSession)
	mux.HandleFunc("PATCH /api/workspaces/{workspaceId}/terminal/sessions/{sessionId}", s.patchTerminalSession)
	mux.HandleFunc("POST /api/workspaces/{workspaceId}/terminal/sessions/{sessionId}/close", s.closeTerminalSession)
	mux.HandleFunc("GET /api/workspaces/{workspaceId}/terminal/sessions/{sessionId}/socket", s.terminalSocket)
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
