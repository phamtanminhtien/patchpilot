package agent

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/phamtanminhtien/patchpilot/internal/config"
	"github.com/phamtanminhtien/patchpilot/internal/database"
	"github.com/phamtanminhtien/patchpilot/internal/events"
	"github.com/phamtanminhtien/patchpilot/internal/filestore"
	"github.com/phamtanminhtien/patchpilot/internal/gitrepo"
	"github.com/phamtanminhtien/patchpilot/internal/runner"
	"github.com/phamtanminhtien/patchpilot/internal/skills"
)

var (
	ErrEmptyPrompt         = errors.New("prompt is required")
	ErrInvalidModel        = errors.New("invalid model")
	ErrInvalidEffort       = errors.New("invalid reasoning effort")
	ErrProviderUnavailable = errors.New("agent provider is unavailable")
	ErrRunNotFound         = errors.New("agent run not found")
	ErrToolCallNotFound    = errors.New("agent tool call not found")
	ErrToolNotApprovable   = errors.New("agent tool call is not waiting for approval")
	ErrRunNotResumable     = errors.New("agent run is not resumable after server restart")
)

var supportedModels = map[string]struct{}{
	"gpt-5.5":      {},
	"gpt-5.4":      {},
	"gpt-5.4-mini": {},
}

var supportedEfforts = map[string]struct{}{
	"low":    {},
	"medium": {},
	"high":   {},
	"xhigh":  {},
}

type RunStatus string

const (
	StatusQueued              RunStatus = "queued"
	StatusRunning             RunStatus = "running"
	StatusWaitingToolApproval RunStatus = "waiting_tool_approval"
	StatusDone                RunStatus = "done"
	StatusFailed              RunStatus = "failed"
	StatusCanceled            RunStatus = "canceled"
)

const (
	ToolStatusPending         = "pending"
	ToolStatusWaitingApproval = "waiting_approval"
	ToolStatusApproved        = "approved"
	ToolStatusRejected        = "rejected"
	ToolStatusRunning         = "running"
	ToolStatusFinished        = "finished"
	ToolStatusFailed          = "failed"
)

type CreateRunInput struct {
	Prompt           string
	ConversationID   string
	TriggerMessageID string
	Model            string
	ReasoningEffort  string
	Permissions      config.WorkspacePermission
}

type Run struct {
	ID               string     `json:"id"`
	WorkspaceID      string     `json:"workspaceId"`
	ConversationID   string     `json:"conversationId"`
	TriggerMessageID string     `json:"triggerMessageId"`
	Model            string     `json:"model"`
	ReasoningEffort  string     `json:"reasoningEffort"`
	Status           string     `json:"status"`
	Summary          string     `json:"summary"`
	Error            *string    `json:"error"`
	StartedAt        *time.Time `json:"startedAt"`
	FinishedAt       *time.Time `json:"finishedAt"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
}

type RunEvent struct {
	ID          string          `json:"id"`
	WorkspaceID string          `json:"workspaceId"`
	RunID       string          `json:"runId"`
	Type        string          `json:"type"`
	Payload     json.RawMessage `json:"payload"`
	CreatedAt   time.Time       `json:"createdAt"`
}

type ToolCall struct {
	ID               string     `json:"id"`
	WorkspaceID      string     `json:"workspaceId"`
	RunID            string     `json:"runId"`
	BatchID          string     `json:"batchId"`
	Sequence         int        `json:"sequence"`
	ProviderCallID   string     `json:"providerCallId"`
	Name             string     `json:"name"`
	Input            string     `json:"input"`
	Output           string     `json:"output"`
	Status           string     `json:"status"`
	RequiresApproval bool       `json:"requiresApproval"`
	Source           string     `json:"source"`
	SourceRef        *string    `json:"sourceRef,omitempty"`
	PolicyReason     string     `json:"policyReason,omitempty"`
	Decision         *string    `json:"decision"`
	StartedAt        *time.Time `json:"startedAt"`
	FinishedAt       *time.Time `json:"finishedAt"`
	CreatedAt        time.Time  `json:"createdAt"`
}

type Detail struct {
	Run       Run        `json:"run"`
	Events    []RunEvent `json:"events"`
	ToolCalls []ToolCall `json:"toolCalls"`
}

type Provider interface {
	Generate(context.Context, ProviderRequest, Stream) (ProviderResult, error)
	Summarize(context.Context, SummaryRequest) (string, error)
	Configured() bool
}

type titleProvider interface {
	GenerateTitle(context.Context, string, string) (string, error)
}

type ProviderRequest struct {
	Run                 Run
	Prompt              string
	WorkspaceRoot       string
	Shell               string
	CurrentDate         string
	Timezone            string
	RepoInstructions    []InstructionSource
	SelectedSkills      []skills.Skill
	ContextWarnings     []ContextWarning
	ContextSummary      string
	ConversationContext []ProviderMessage
	History             []ProviderHistoryItem
}

type ProviderHistoryItem struct {
	Type       string       `json:"type"`
	Text       string       `json:"text,omitempty"`
	ToolCall   ToolRequest  `json:"toolCall,omitempty"`
	ToolResult ToolResponse `json:"toolResult,omitempty"`
}

type ToolRequest struct {
	CallID    string `json:"callId"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type ToolResponse struct {
	CallID string `json:"callId"`
	Output string `json:"output"`
}

type ProviderResult struct {
	Text      string
	ToolCalls []ToolRequest
	Done      bool
}

type Stream interface {
	Delta(context.Context, string)
}

type Manager struct {
	store    *database.Store
	files    *filestore.Service
	git      *gitrepo.Client
	runner   *runner.Runner
	events   *events.Hub
	provider Provider
	homeDir  string

	mu       sync.Mutex
	runtimes map[string]*runRuntime
}

type runRuntime struct {
	workspaceID         string
	workspaceRoot       string
	conversationID      string
	runID               string
	triggerMessageID    string
	prompt              string
	ctx                 context.Context
	cancel              context.CancelFunc
	contextSummary      string
	repoInstructions    []InstructionSource
	selectedSkills      []skills.Skill
	contextWarnings     []ContextWarning
	conversationContext []ProviderMessage
	history             []ProviderHistoryItem
	pendingBatch        string
	activeCommands      map[string]struct{}
	permissions         config.WorkspacePermission
	draftText           strings.Builder
}

const shutdownFailureMessage = "backend shutdown"

func NewManager(store *database.Store, files *filestore.Service, git *gitrepo.Client, run *runner.Runner, hub *events.Hub, provider Provider) *Manager {
	home, _ := os.UserHomeDir()
	return NewManagerWithHome(store, files, git, run, hub, provider, home)
}

func NewManagerWithHome(store *database.Store, files *filestore.Service, git *gitrepo.Client, run *runner.Runner, hub *events.Hub, provider Provider, homeDir string) *Manager {
	return &Manager{store: store, files: files, git: git, runner: run, events: hub, provider: provider, homeDir: homeDir, runtimes: map[string]*runRuntime{}}
}

func (m *Manager) Create(ctx context.Context, workspaceID, workspaceRoot string, input CreateRunInput) (Run, error) {
	input.Prompt = strings.TrimSpace(input.Prompt)
	input.ConversationID = strings.TrimSpace(input.ConversationID)
	input.TriggerMessageID = strings.TrimSpace(input.TriggerMessageID)
	input.Model = strings.TrimSpace(input.Model)
	input.ReasoningEffort = strings.TrimSpace(input.ReasoningEffort)
	if input.Prompt == "" || input.ConversationID == "" || input.TriggerMessageID == "" {
		return Run{}, ErrEmptyPrompt
	}
	if _, ok := supportedModels[input.Model]; !ok {
		return Run{}, ErrInvalidModel
	}
	if _, ok := supportedEfforts[input.ReasoningEffort]; !ok {
		return Run{}, ErrInvalidEffort
	}
	if input.Permissions == (config.WorkspacePermission{}) {
		input.Permissions = config.DefaultWorkspacePermission()
	} else {
		input.Permissions = config.NormalizeWorkspacePermission(input.Permissions)
	}
	if m.provider == nil || !m.provider.Configured() {
		return Run{}, ErrProviderUnavailable
	}

	run, err := m.store.CreateAgentRun(ctx, database.AgentRunRecord{
		WorkspaceID:      workspaceID,
		ConversationID:   input.ConversationID,
		TriggerMessageID: input.TriggerMessageID,
		Model:            input.Model,
		ReasoningEffort:  input.ReasoningEffort,
		Status:           string(StatusQueued),
	})
	if err != nil {
		return Run{}, err
	}
	publicRun := RunFromRecord(run)
	runCtx, cancel := context.WithCancel(context.Background())
	runtime := &runRuntime{
		workspaceID:      workspaceID,
		workspaceRoot:    workspaceRoot,
		conversationID:   input.ConversationID,
		runID:            run.ID,
		triggerMessageID: input.TriggerMessageID,
		prompt:           input.Prompt,
		permissions:      input.Permissions,
		ctx:              runCtx,
		cancel:           cancel,
		activeCommands:   map[string]struct{}{},
	}
	m.setRuntime(runtime)
	go m.run(runtime)
	return publicRun, nil
}

func (m *Manager) List(ctx context.Context, workspaceID, conversationID string) ([]Run, error) {
	records, err := m.store.ListAgentRuns(ctx, workspaceID, conversationID)
	if err != nil {
		return nil, err
	}
	runs := make([]Run, 0, len(records))
	for _, record := range records {
		runs = append(runs, RunFromRecord(record))
	}
	return runs, nil
}

func (m *Manager) Detail(ctx context.Context, workspaceID, conversationID, runID string) (Detail, error) {
	run, err := m.store.GetAgentRun(ctx, workspaceID, conversationID, runID)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return Detail{}, ErrRunNotFound
		}
		return Detail{}, err
	}
	eventRecords, err := m.store.ListAgentRunEvents(ctx, workspaceID, runID)
	if err != nil {
		return Detail{}, err
	}
	toolRecords, err := m.store.ListAgentToolCalls(ctx, workspaceID, runID)
	if err != nil {
		return Detail{}, err
	}
	return Detail{
		Run:       RunFromRecord(run),
		Events:    EventsFromRecords(eventRecords),
		ToolCalls: ToolCallsFromRecords(toolRecords),
	}, nil
}

func (m *Manager) SetSkillEnabled(_ context.Context, key string, enabled bool) (skills.Skill, error) {
	home := m.homeDir
	if strings.TrimSpace(home) == "" {
		home, _ = os.UserHomeDir()
	}
	skill, _, err := skills.SetEnabled(home, key, enabled)
	return skill, err
}

func (m *Manager) DraftText(runID string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	runtime := m.runtimes[runID]
	if runtime == nil {
		return ""
	}
	return runtime.draftText.String()
}

func (m *Manager) GenerateTitle(ctx context.Context, prompt, model string) (string, error) {
	prompt = strings.TrimSpace(prompt)
	model = strings.TrimSpace(model)
	if prompt == "" {
		return "", ErrEmptyPrompt
	}
	if m.provider == nil || !m.provider.Configured() {
		return "", ErrProviderUnavailable
	}
	provider, ok := m.provider.(titleProvider)
	if !ok {
		return "", ErrProviderUnavailable
	}
	return provider.GenerateTitle(ctx, prompt, model)
}
