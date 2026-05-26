package agent

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

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

type ProviderRequest struct {
	Run                 Run
	Prompt              string
	WorkspaceRoot       string
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

func (m *Manager) Cancel(ctx context.Context, workspaceID, conversationID, runID string) (Run, error) {
	current, err := m.store.GetAgentRun(ctx, workspaceID, conversationID, runID)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return Run{}, ErrRunNotFound
		}
		return Run{}, err
	}
	if isTerminalStatus(current.Status) {
		return RunFromRecord(current), nil
	}
	if runtime := m.runtime(runID); runtime != nil {
		runtime.cancel()
		m.stopRuntimeCommands(runtime)
	}
	m.cancelActiveToolCalls(ctx, workspaceID, runID)
	now := time.Now().UTC()
	run, err := m.store.UpdateAgentRun(ctx, workspaceID, conversationID, runID, map[string]any{
		"status":      string(StatusCanceled),
		"finished_at": now,
	})
	if err != nil {
		return Run{}, err
	}
	m.deleteRuntime(runID)
	publicRun := RunFromRecord(run)
	_ = m.publish(ctx, publicRun, "agent.run.status_changed", publicRun)
	return publicRun, nil
}

func (m *Manager) Shutdown(ctx context.Context, reason string) error {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = shutdownFailureMessage
	}
	runs, err := m.store.ListActiveAgentRuns(ctx)
	if err != nil {
		return err
	}
	for _, record := range runs {
		runtime := m.runtime(record.ID)
		if runtime != nil {
			runtime.cancel()
			m.stopRuntimeCommands(runtime)
		}
		m.failRunForShutdown(ctx, record, reason)
		if runtime != nil {
			m.deleteRuntime(record.ID)
		}
	}
	return nil
}

func (m *Manager) cancelActiveToolCalls(ctx context.Context, workspaceID, runID string) {
	calls, err := m.store.ListAgentToolCalls(ctx, workspaceID, runID)
	if err != nil {
		return
	}
	finished := time.Now().UTC()
	output := openToolJSON(map[string]string{"status": "canceled"})
	for _, call := range calls {
		switch call.Status {
		case ToolStatusPending, ToolStatusApproved, ToolStatusRunning:
			_, _ = m.store.FinishAgentToolCall(ctx, workspaceID, runID, call.ID, ToolStatusFailed, output, finished)
		}
	}
}

func (m *Manager) failRunForShutdown(ctx context.Context, record database.AgentRunRecord, reason string) {
	m.failToolCallsForShutdown(ctx, record.WorkspaceID, record.ID, reason)
	now := time.Now().UTC()
	updated, err := m.store.UpdateAgentRun(ctx, record.WorkspaceID, record.ConversationID, record.ID, map[string]any{
		"status":      string(StatusFailed),
		"error":       reason,
		"finished_at": now,
	})
	if err != nil {
		return
	}
	publicRun := RunFromRecord(updated)
	_ = m.publish(ctx, publicRun, "agent.run.status_changed", publicRun)
}

func (m *Manager) failToolCallsForShutdown(ctx context.Context, workspaceID, runID, reason string) {
	calls, err := m.store.ListAgentToolCalls(ctx, workspaceID, runID)
	if err != nil {
		return
	}
	finished := time.Now().UTC()
	output := openToolError(errors.New(reason))
	for _, call := range calls {
		switch call.Status {
		case ToolStatusPending, ToolStatusWaitingApproval, ToolStatusApproved, ToolStatusRunning:
			_, _ = m.store.FinishAgentToolCall(ctx, workspaceID, runID, call.ID, ToolStatusFailed, output, finished)
		}
	}
}

func (m *Manager) ApproveToolCall(ctx context.Context, workspaceID, runID, toolCallID string) (ToolCall, error) {
	return m.decideToolCall(ctx, workspaceID, runID, toolCallID, "approved")
}

func (m *Manager) RejectToolCall(ctx context.Context, workspaceID, runID, toolCallID string) (ToolCall, error) {
	return m.decideToolCall(ctx, workspaceID, runID, toolCallID, "rejected")
}

func (m *Manager) decideToolCall(ctx context.Context, workspaceID, runID, toolCallID, decision string) (ToolCall, error) {
	runtime := m.runtime(runID)
	if runtime == nil {
		return ToolCall{}, ErrRunNotResumable
	}
	call, err := m.store.GetAgentToolCall(ctx, workspaceID, runID, toolCallID)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return ToolCall{}, ErrToolCallNotFound
		}
		return ToolCall{}, err
	}
	if !call.RequiresApproval || call.Status != ToolStatusWaitingApproval {
		return ToolCall{}, ErrToolNotApprovable
	}
	call, err = m.store.UpdateAgentToolCall(ctx, workspaceID, runID, toolCallID, map[string]any{
		"status":   decision,
		"decision": decision,
	})
	if err != nil {
		return ToolCall{}, err
	}
	publicCall := ToolCallFromRecord(call)
	run := Run{ID: runID, WorkspaceID: workspaceID, ConversationID: runtime.conversationID}
	_ = m.publish(ctx, run, "agent.tool.finished", publicCall)
	if m.batchDecided(ctx, workspaceID, runID, call.BatchID) {
		go m.resume(runtime)
	}
	return publicCall, nil
}

func (m *Manager) run(runtime *runRuntime) {
	ctx := runtime.ctx
	startedAt := time.Now().UTC()
	record, err := m.store.UpdateAgentRun(ctx, runtime.workspaceID, runtime.conversationID, runtime.runID, map[string]any{
		"status":     string(StatusRunning),
		"started_at": startedAt,
	})
	if err != nil {
		return
	}
	run := RunFromRecord(record)
	_ = m.publish(ctx, run, "agent.run.status_changed", run)

	if err := m.prepareConversationContext(ctx, run, runtime); err != nil {
		m.fail(ctx, run, err)
		return
	}
	m.loop(ctx, runtime)
}

func (m *Manager) resume(runtime *runRuntime) {
	ctx := runtime.ctx
	record, err := m.store.UpdateAgentRun(ctx, runtime.workspaceID, runtime.conversationID, runtime.runID, map[string]any{
		"status": string(StatusRunning),
	})
	if err != nil {
		return
	}
	run := RunFromRecord(record)
	_ = m.publish(ctx, run, "agent.run.status_changed", run)
	calls, err := m.store.ListAgentToolCalls(ctx, runtime.workspaceID, runtime.runID)
	if err != nil {
		m.fail(ctx, run, err)
		return
	}
	batch := callsForBatch(calls, runtime.pendingBatch)
	results := m.executeBatch(ctx, run, runtime, batch)
	runtime.history = append(runtime.history, results...)
	runtime.pendingBatch = ""
	m.loop(ctx, runtime)
}

func (m *Manager) prepareConversationContext(ctx context.Context, run Run, runtime *runRuntime) error {
	snapshot, err := m.RefreshContext(ctx, runtime.workspaceRoot)
	if err != nil {
		return err
	}
	runtime.repoInstructions = snapshot.InstructionSources
	runtime.selectedSkills = skills.EnabledContext(skills.Registry{Skills: snapshot.Skills})
	runtime.contextWarnings = append(snapshot.SkippedSources, snapshot.ContextWarnings...)
	conversation, err := m.store.GetConversation(ctx, runtime.workspaceID, runtime.conversationID)
	if err != nil {
		return err
	}
	afterMessageID := ""
	if conversation.ContextSummaryThroughMessageID != nil {
		afterMessageID = *conversation.ContextSummaryThroughMessageID
	}
	messages, err := m.store.ListMessagesAfter(ctx, runtime.workspaceID, runtime.conversationID, afterMessageID)
	if err != nil {
		return err
	}
	context := buildConversationContext(conversation, messages, runtime.triggerMessageID)
	if len(context.SummarizeRecords) > 0 {
		m.events.Publish(events.Event{
			WorkspaceID: run.WorkspaceID,
			Type:        "agent.delta",
			Payload:     map[string]string{"runId": run.ID, "text": "Summarizing earlier conversation context."},
		})
		summary, err := m.provider.Summarize(ctx, SummaryRequest{
			Run:             run,
			ExistingSummary: context.Summary,
			Messages:        providerMessagesFromRecords(context.SummarizeRecords),
		})
		if err != nil {
			return err
		}
		summary = strings.TrimSpace(summary)
		throughMessageID := context.SummarizeRecords[len(context.SummarizeRecords)-1].ID
		conversation, err = m.store.UpdateConversationContextSummary(ctx, runtime.workspaceID, runtime.conversationID, summary, throughMessageID, time.Now().UTC())
		if err != nil {
			return err
		}
		messages, err = m.store.ListMessagesAfter(ctx, runtime.workspaceID, runtime.conversationID, throughMessageID)
		if err != nil {
			return err
		}
		context = buildConversationContext(conversation, messages, runtime.triggerMessageID)
	}
	runtime.contextSummary = context.Summary
	runtime.conversationContext = context.Messages
	return nil
}

func (m *Manager) loop(ctx context.Context, runtime *runRuntime) {
	for {
		record, err := m.store.GetAgentRun(ctx, runtime.workspaceID, runtime.conversationID, runtime.runID)
		if err != nil {
			return
		}
		run := RunFromRecord(record)
		m.resetDraftText(runtime)
		result, err := m.provider.Generate(ctx, ProviderRequest{
			Run:                 run,
			Prompt:              runtime.prompt,
			WorkspaceRoot:       runtime.workspaceRoot,
			RepoInstructions:    runtime.repoInstructions,
			SelectedSkills:      runtime.selectedSkills,
			ContextWarnings:     runtime.contextWarnings,
			ContextSummary:      runtime.contextSummary,
			ConversationContext: runtime.conversationContext,
			History:             runtime.history,
		}, m.stream(run))
		if err != nil {
			if errors.Is(err, context.Canceled) || ctx.Err() != nil {
				return
			}
			m.fail(ctx, run, err)
			return
		}
		if strings.TrimSpace(result.Text) != "" {
			runtime.history = append(runtime.history, ProviderHistoryItem{Type: "text", Text: result.Text})
		}
		if len(result.ToolCalls) == 0 {
			now := time.Now().UTC()
			summary := strings.TrimSpace(result.Text)
			if summary != "" {
				runID := runtime.runID
				message, _ := m.store.CreateMessage(ctx, database.MessageRecord{
					WorkspaceID:    runtime.workspaceID,
					ConversationID: runtime.conversationID,
					Role:           "assistant",
					Content:        summary,
					RunID:          &runID,
					CreatedAt:      now,
				})
				if message.ID != "" {
					_ = m.publish(ctx, run, "conversation.message.created", messageEventPayload(message))
				}
			}
			m.resetDraftText(runtime)
			record, err = m.store.UpdateAgentRun(ctx, runtime.workspaceID, runtime.conversationID, runtime.runID, map[string]any{
				"status":      string(StatusDone),
				"summary":     summary,
				"finished_at": now,
			})
			if err == nil {
				_ = m.publish(ctx, RunFromRecord(record), "agent.run.status_changed", RunFromRecord(record))
			}
			m.deleteRuntime(runtime.runID)
			return
		}
		if strings.TrimSpace(result.Text) != "" {
			runID := runtime.runID
			message, _ := m.store.CreateMessage(ctx, database.MessageRecord{
				WorkspaceID:    runtime.workspaceID,
				ConversationID: runtime.conversationID,
				Role:           "assistant",
				Content:        strings.TrimSpace(result.Text),
				RunID:          &runID,
			})
			if message.ID != "" {
				_ = m.publish(ctx, run, "conversation.message.created", messageEventPayload(message))
			}
			m.resetDraftText(runtime)
		}
		waiting, results, err := m.prepareOrExecuteBatch(ctx, run, runtime, result.ToolCalls)
		if err != nil {
			m.fail(ctx, run, err)
			return
		}
		runtime.history = append(runtime.history, results...)
		if waiting {
			return
		}
	}
}

func (m *Manager) prepareOrExecuteBatch(ctx context.Context, run Run, runtime *runRuntime, requests []ToolRequest) (bool, []ProviderHistoryItem, error) {
	batchID, err := randomID("batch_")
	if err != nil {
		return false, nil, err
	}
	records := make([]database.AgentToolCallRecord, 0, len(requests))
	requiresApproval := false
	for i, request := range requests {
		input := request.Arguments
		if input == "" {
			input = "{}"
		}
		callRequiresApproval, initialStatus, initialOutput := m.prepareToolRequest(ctx, runtime.workspaceRoot, request)
		source, sourceRef, policyReason := toolSourceMetadata(request.Name, request.Arguments, callRequiresApproval)
		if callRequiresApproval {
			requiresApproval = true
		}
		record, err := m.store.CreateAgentToolCall(ctx, database.AgentToolCallRecord{
			WorkspaceID:      run.WorkspaceID,
			RunID:            run.ID,
			BatchID:          batchID,
			Sequence:         i,
			ProviderCallID:   request.CallID,
			Name:             request.Name,
			InputJSON:        input,
			OutputJSON:       initialOutput,
			Status:           initialStatus,
			RequiresApproval: callRequiresApproval,
			Source:           source,
			SourceRef:        sourceRef,
			PolicyReason:     policyReason,
		})
		if err != nil {
			return false, nil, err
		}
		records = append(records, record)
		runtime.history = append(runtime.history, ProviderHistoryItem{Type: "tool_call", ToolCall: request})
		if callRequiresApproval {
			_ = m.publish(ctx, run, "agent.approval_required", ToolCallFromRecord(record))
		}
	}
	if requiresApproval {
		runtime.pendingBatch = batchID
		record, err := m.store.UpdateAgentRun(ctx, run.WorkspaceID, run.ConversationID, run.ID, map[string]any{"status": string(StatusWaitingToolApproval)})
		if err == nil {
			_ = m.publish(ctx, RunFromRecord(record), "agent.run.status_changed", RunFromRecord(record))
		}
		return true, nil, nil
	}
	return false, m.executeBatch(ctx, run, runtime, records), nil
}

func (m *Manager) executeBatch(ctx context.Context, run Run, runtime *runRuntime, records []database.AgentToolCallRecord) []ProviderHistoryItem {
	results := make([]ProviderHistoryItem, 0, len(records))
	for _, record := range records {
		if record.Status == ToolStatusFailed || record.Status == ToolStatusFinished {
			results = append(results, ProviderHistoryItem{Type: "tool_result", ToolResult: ToolResponse{CallID: record.ProviderCallID, Output: record.OutputJSON}})
			continue
		}
		if record.RequiresApproval && record.Status == ToolStatusRejected {
			output := openToolJSON(map[string]string{"status": "rejected"})
			finished := time.Now().UTC()
			updated, err := m.store.FinishAgentToolCall(ctx, run.WorkspaceID, run.ID, record.ID, ToolStatusRejected, output, finished)
			if err == nil {
				_ = m.publish(ctx, run, "agent.tool.finished", ToolCallFromRecord(updated))
			}
			results = append(results, ProviderHistoryItem{Type: "tool_result", ToolResult: ToolResponse{CallID: record.ProviderCallID, Output: output}})
			continue
		}
		started := time.Now().UTC()
		updated, err := m.store.UpdateAgentToolCall(ctx, run.WorkspaceID, run.ID, record.ID, map[string]any{
			"status":     ToolStatusRunning,
			"started_at": started,
		})
		if err == nil {
			_ = m.publish(ctx, run, "agent.tool.started", ToolCallFromRecord(updated))
		}
		output, execErr := m.executeTool(ctx, runtime, record)
		if errors.Is(execErr, context.Canceled) || ctx.Err() != nil {
			return results
		}
		status := ToolStatusFinished
		if execErr != nil {
			status = ToolStatusFailed
			output = openToolError(execErr)
		}
		finished := time.Now().UTC()
		updated, err = m.store.FinishAgentToolCall(ctx, run.WorkspaceID, run.ID, record.ID, status, output, finished)
		if err == nil {
			_ = m.publish(ctx, run, "agent.tool.finished", ToolCallFromRecord(updated))
		}
		if record.Name == "apply_patch" && execErr == nil {
			m.events.Publish(events.Event{WorkspaceID: run.WorkspaceID, Type: "git.changed", Payload: map[string]string{"workspaceId": run.WorkspaceID}})
		}
		results = append(results, ProviderHistoryItem{Type: "tool_result", ToolResult: ToolResponse{CallID: record.ProviderCallID, Output: output}})
	}
	return results
}

func (m *Manager) prepareToolRequest(ctx context.Context, workspaceRoot string, request ToolRequest) (bool, string, string) {
	switch request.Name {
	case "apply_patch":
		var args struct {
			Diff string `json:"diff"`
		}
		if err := json.Unmarshal([]byte(request.Arguments), &args); err != nil {
			return false, ToolStatusFailed, openToolError(err)
		}
		if err := m.git.CheckPatch(ctx, workspaceRoot, normalizeProviderPatch(args.Diff)); err != nil {
			return false, ToolStatusFailed, openToolError(fmt.Errorf("patch failed git apply check: %w", err))
		}
		if policy, ok := staticToolPolicy(request.Name); ok && policy == toolRequiresApproval {
			return true, ToolStatusWaitingApproval, "{}"
		}
		return false, ToolStatusPending, "{}"
	case "run_command":
		var args struct {
			Command string `json:"command"`
		}
		if err := json.Unmarshal([]byte(request.Arguments), &args); err != nil {
			return false, ToolStatusFailed, openToolError(err)
		}
		decision, err := runner.Classify(args.Command)
		if err != nil {
			return false, ToolStatusFailed, openToolError(err)
		}
		if decision.Level == runner.SafetyNeedsConfirmation {
			return true, ToolStatusWaitingApproval, "{}"
		}
		return false, ToolStatusPending, "{}"
	default:
		if strings.HasPrefix(request.Name, "mcp.") {
			return true, ToolStatusWaitingApproval, "{}"
		}
		if policy, ok := staticToolPolicy(request.Name); ok && policy == toolRequiresApproval {
			return true, ToolStatusWaitingApproval, "{}"
		}
		return false, ToolStatusPending, "{}"
	}
}

func (m *Manager) executeTool(ctx context.Context, runtime *runRuntime, record database.AgentToolCallRecord) (string, error) {
	workspaceRoot := runtime.workspaceRoot
	switch record.Name {
	case "list_files":
		var args struct {
			Path string `json:"path"`
		}
		if err := json.Unmarshal([]byte(record.InputJSON), &args); err != nil {
			return "", err
		}
		entries, err := m.files.List(workspaceRoot, args.Path)
		if err != nil {
			return "", err
		}
		return openToolJSON(map[string]any{"entries": entries}), nil
	case "search_files":
		var args struct {
			Query string `json:"query"`
			Path  string `json:"path"`
		}
		if err := json.Unmarshal([]byte(record.InputJSON), &args); err != nil {
			return "", err
		}
		results, err := m.files.SearchWithOptions(workspaceRoot, args.Query, filestore.SearchOptions{Path: args.Path})
		if err != nil {
			return "", err
		}
		if len(results) > 25 {
			results = results[:25]
		}
		return openToolJSON(map[string]any{"results": results}), nil
	case "run_command":
		return m.executeCommandTool(ctx, runtime, record)
	case "apply_patch":
		var args struct {
			Diff    string `json:"diff"`
			Summary string `json:"summary"`
		}
		if err := json.Unmarshal([]byte(record.InputJSON), &args); err != nil {
			return "", err
		}
		diff := normalizeProviderPatch(args.Diff)
		if err := m.git.ApplyPatch(ctx, workspaceRoot, diff, gitrepo.ApplyForward); err != nil {
			return "", err
		}
		return openToolJSON(map[string]string{"status": "applied", "summary": args.Summary}), nil
	default:
		if strings.HasPrefix(record.Name, "mcp.") {
			return "", fmt.Errorf("MCP tool execution is not connected for %s", record.Name)
		}
		return "", fmt.Errorf("unknown tool: %s", record.Name)
	}
}

func toolSourceMetadata(name, inputJSON string, requiresApproval bool) (string, *string, string) {
	if strings.HasPrefix(name, "mcp.") {
		ref := strings.TrimPrefix(name, "mcp.")
		return "mcp", &ref, "MCP tools require approval unless configured read-only and safe."
	}
	if strings.HasPrefix(name, "skill.") {
		ref := strings.TrimPrefix(name, "skill.")
		return "skill", &ref, "Skill tool call."
	}
	if requiresApproval {
		return "builtin", nil, "Built-in tool requires approval."
	}
	return "builtin", nil, ""
}

func (m *Manager) executeCommandTool(ctx context.Context, runtime *runRuntime, record database.AgentToolCallRecord) (string, error) {
	var args struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal([]byte(record.InputJSON), &args); err != nil {
		return "", err
	}
	decision, err := runner.Classify(args.Command)
	if err != nil {
		return "", err
	}
	if decision.Level == runner.SafetyBlocked {
		return openToolJSON(map[string]any{"status": "blocked", "decision": decision}), nil
	}
	if decision.Level == runner.SafetyNeedsConfirmation && record.Decision == nil {
		return "", fmt.Errorf("command requires approval: %s", decision.Reason)
	}
	done := make(chan runner.FinishResult, 1)
	var output strings.Builder
	err = m.runner.Start(runner.RunSpec{
		ID:          record.ID,
		WorkspaceID: record.WorkspaceID,
		Command:     args.Command,
		Cwd:         runtime.workspaceRoot,
	}, runner.Hooks{
		OnOutput: func(_, chunk string) {
			if output.Len() < 1024*1024 {
				output.WriteString(chunk)
			}
		},
		OnFinished: func(result runner.FinishResult) {
			done <- result
		},
	})
	if err != nil {
		return "", err
	}
	m.addRuntimeCommand(runtime, record.ID)
	defer m.removeRuntimeCommand(runtime, record.ID)
	var result runner.FinishResult
	select {
	case <-ctx.Done():
		m.runner.Stop(record.ID)
		return "", context.Canceled
	case result = <-done:
	}
	return openToolJSON(map[string]any{"status": result.Status, "exitCode": result.ExitCode, "output": output.String()}), nil
}

func (m *Manager) batchDecided(ctx context.Context, workspaceID, runID, batchID string) bool {
	calls, err := m.store.ListAgentToolCalls(ctx, workspaceID, runID)
	if err != nil {
		return false
	}
	for _, call := range calls {
		if call.BatchID == batchID && call.RequiresApproval && call.Decision == nil {
			return false
		}
	}
	return true
}

func callsForBatch(calls []database.AgentToolCallRecord, batchID string) []database.AgentToolCallRecord {
	out := make([]database.AgentToolCallRecord, 0)
	for _, call := range calls {
		if call.BatchID == batchID {
			out = append(out, call)
		}
	}
	return out
}

func (m *Manager) fail(ctx context.Context, run Run, failure error) {
	if errors.Is(failure, context.Canceled) || ctx.Err() != nil {
		return
	}
	message := failure.Error()
	now := time.Now().UTC()
	record, err := m.store.UpdateAgentRun(ctx, run.WorkspaceID, run.ConversationID, run.ID, map[string]any{
		"status":      string(StatusFailed),
		"error":       message,
		"finished_at": now,
	})
	if err != nil {
		return
	}
	m.deleteRuntime(run.ID)
	publicRun := RunFromRecord(record)
	_ = m.publish(ctx, publicRun, "agent.run.status_changed", publicRun)
}

func (m *Manager) publish(ctx context.Context, run Run, eventType string, payload any) error {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	record, err := m.store.CreateAgentRunEvent(ctx, database.AgentRunEventRecord{
		WorkspaceID: run.WorkspaceID,
		RunID:       run.ID,
		Type:        eventType,
		PayloadJSON: string(payloadBytes),
	})
	if err != nil {
		return err
	}
	m.events.Publish(events.Event{
		ID:          record.ID,
		WorkspaceID: run.WorkspaceID,
		Type:        eventType,
		CreatedAt:   record.CreatedAt,
		Payload:     payload,
	})
	return nil
}

func (m *Manager) stream(run Run) Stream {
	return streamFunc(func(ctx context.Context, text string) {
		if text == "" {
			return
		}
		m.appendDraftText(run.ID, text)
		m.events.Publish(events.Event{
			WorkspaceID: run.WorkspaceID,
			Type:        "agent.delta",
			Payload:     map[string]string{"runId": run.ID, "text": text},
		})
	})
}

type streamFunc func(context.Context, string)

func (s streamFunc) Delta(ctx context.Context, text string) {
	s(ctx, text)
}

func (m *Manager) setRuntime(runtime *runRuntime) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.runtimes[runtime.runID] = runtime
}

func (m *Manager) runtime(runID string) *runRuntime {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.runtimes[runID]
}

func (m *Manager) deleteRuntime(runID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.runtimes, runID)
}

func (m *Manager) appendDraftText(runID, text string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	runtime := m.runtimes[runID]
	if runtime == nil {
		return
	}
	runtime.draftText.WriteString(text)
}

func (m *Manager) resetDraftText(runtime *runRuntime) {
	m.mu.Lock()
	defer m.mu.Unlock()
	runtime.draftText.Reset()
}

func (m *Manager) addRuntimeCommand(runtime *runRuntime, commandID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	runtime.activeCommands[commandID] = struct{}{}
}

func (m *Manager) removeRuntimeCommand(runtime *runRuntime, commandID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(runtime.activeCommands, commandID)
}

func (m *Manager) stopRuntimeCommands(runtime *runRuntime) {
	m.mu.Lock()
	commandIDs := make([]string, 0, len(runtime.activeCommands))
	for commandID := range runtime.activeCommands {
		commandIDs = append(commandIDs, commandID)
	}
	m.mu.Unlock()
	for _, commandID := range commandIDs {
		m.runner.Stop(commandID)
	}
}

func isTerminalStatus(status string) bool {
	return status == string(StatusDone) || status == string(StatusFailed) || status == string(StatusCanceled)
}

func messageEventPayload(message database.MessageRecord) map[string]any {
	payload := map[string]any{
		"id":             message.ID,
		"workspaceId":    message.WorkspaceID,
		"conversationId": message.ConversationID,
		"role":           message.Role,
		"content":        message.Content,
		"createdAt":      message.CreatedAt,
	}
	if message.RunID != nil {
		payload["runId"] = *message.RunID
	}
	return payload
}

func openToolError(err error) string {
	return openToolJSON(map[string]string{"error": err.Error()})
}

func openToolJSON(value any) string {
	payload, err := json.Marshal(value)
	if err != nil {
		return `{"error":"failed to encode tool output"}`
	}
	return string(payload)
}

func randomID(prefix string) (string, error) {
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return prefix + hex.EncodeToString(bytes), nil
}

func RunFromRecord(record database.AgentRunRecord) Run {
	return Run{
		ID:               record.ID,
		WorkspaceID:      record.WorkspaceID,
		ConversationID:   record.ConversationID,
		TriggerMessageID: record.TriggerMessageID,
		Model:            record.Model,
		ReasoningEffort:  record.ReasoningEffort,
		Status:           record.Status,
		Summary:          record.Summary,
		Error:            record.Error,
		StartedAt:        record.StartedAt,
		FinishedAt:       record.FinishedAt,
		CreatedAt:        record.CreatedAt,
		UpdatedAt:        record.UpdatedAt,
	}
}

func EventsFromRecords(records []database.AgentRunEventRecord) []RunEvent {
	out := make([]RunEvent, 0, len(records))
	for _, record := range records {
		out = append(out, RunEventFromRecord(record))
	}
	return out
}

func RunEventFromRecord(record database.AgentRunEventRecord) RunEvent {
	return RunEvent{
		ID:          record.ID,
		WorkspaceID: record.WorkspaceID,
		RunID:       record.RunID,
		Type:        record.Type,
		Payload:     json.RawMessage(record.PayloadJSON),
		CreatedAt:   record.CreatedAt,
	}
}

func ToolCallsFromRecords(records []database.AgentToolCallRecord) []ToolCall {
	out := make([]ToolCall, 0, len(records))
	for _, record := range records {
		out = append(out, ToolCallFromRecord(record))
	}
	return out
}

func ToolCallFromRecord(record database.AgentToolCallRecord) ToolCall {
	return ToolCall{
		ID:               record.ID,
		WorkspaceID:      record.WorkspaceID,
		RunID:            record.RunID,
		BatchID:          record.BatchID,
		Sequence:         record.Sequence,
		ProviderCallID:   record.ProviderCallID,
		Name:             record.Name,
		Input:            record.InputJSON,
		Output:           record.OutputJSON,
		Status:           record.Status,
		RequiresApproval: record.RequiresApproval,
		Source:           record.Source,
		SourceRef:        record.SourceRef,
		PolicyReason:     record.PolicyReason,
		Decision:         record.Decision,
		StartedAt:        record.StartedAt,
		FinishedAt:       record.FinishedAt,
		CreatedAt:        record.CreatedAt,
	}
}
