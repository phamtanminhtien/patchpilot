package agent

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/phamtanminhtien/patchpilot/internal/database"
	"github.com/phamtanminhtien/patchpilot/internal/events"
	"github.com/phamtanminhtien/patchpilot/internal/filestore"
	"github.com/phamtanminhtien/patchpilot/internal/gitrepo"
	"github.com/phamtanminhtien/patchpilot/internal/runner"
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
	ErrSecretPath          = errors.New("secret paths cannot enter agent context")
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
	Configured() bool
}

type ProviderRequest struct {
	Run           Run
	Prompt        string
	WorkspaceRoot string
	GitStatus     string
	History       []ProviderHistoryItem
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

	mu       sync.Mutex
	runtimes map[string]*runRuntime
}

type runRuntime struct {
	workspaceID    string
	workspaceRoot  string
	conversationID string
	runID          string
	prompt         string
	gitStatus      string
	history        []ProviderHistoryItem
	pendingBatch   string
}

func NewManager(store *database.Store, files *filestore.Service, git *gitrepo.Client, run *runner.Runner, hub *events.Hub, provider Provider) *Manager {
	return &Manager{store: store, files: files, git: git, runner: run, events: hub, provider: provider, runtimes: map[string]*runRuntime{}}
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
	runtime := &runRuntime{workspaceID: workspaceID, workspaceRoot: workspaceRoot, conversationID: input.ConversationID, runID: run.ID, prompt: input.Prompt}
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

func (m *Manager) Cancel(ctx context.Context, workspaceID, conversationID, runID string) (Run, error) {
	now := time.Now().UTC()
	run, err := m.store.UpdateAgentRun(ctx, workspaceID, conversationID, runID, map[string]any{
		"status":      string(StatusFailed),
		"error":       "Run was cancelled",
		"finished_at": now,
	})
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return Run{}, ErrRunNotFound
		}
		return Run{}, err
	}
	m.deleteRuntime(runID)
	publicRun := RunFromRecord(run)
	_ = m.publish(ctx, publicRun, "agent.run.status_changed", publicRun)
	return publicRun, nil
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
	ctx := context.Background()
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
	_ = m.publish(ctx, run, "agent.delta", map[string]string{"runId": run.ID, "text": "Preparing workspace context."})

	status, err := m.git.Status(ctx, runtime.workspaceRoot)
	if err != nil {
		m.fail(ctx, run, err)
		return
	}
	runtime.gitStatus = status.Porcelain
	m.loop(ctx, runtime)
}

func (m *Manager) resume(runtime *runRuntime) {
	ctx := context.Background()
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

func (m *Manager) loop(ctx context.Context, runtime *runRuntime) {
	for {
		record, err := m.store.GetAgentRun(ctx, runtime.workspaceID, runtime.conversationID, runtime.runID)
		if err != nil {
			return
		}
		run := RunFromRecord(record)
		result, err := m.provider.Generate(ctx, ProviderRequest{
			Run:           run,
			Prompt:        runtime.prompt,
			WorkspaceRoot: runtime.workspaceRoot,
			GitStatus:     runtime.gitStatus,
			History:       runtime.history,
		}, m.stream(run))
		if err != nil {
			m.fail(ctx, run, err)
			return
		}
		if strings.TrimSpace(result.Text) != "" {
			runtime.history = append(runtime.history, ProviderHistoryItem{Type: "text", Text: result.Text})
			_ = m.publish(ctx, run, "agent.delta", map[string]string{"runId": run.ID, "text": result.Text})
		}
		if len(result.ToolCalls) == 0 {
			now := time.Now().UTC()
			summary := strings.TrimSpace(result.Text)
			record, err = m.store.UpdateAgentRun(ctx, runtime.workspaceID, runtime.conversationID, runtime.runID, map[string]any{
				"status":      string(StatusDone),
				"summary":     summary,
				"finished_at": now,
			})
			if err == nil && summary != "" {
				runID := runtime.runID
				_, _ = m.store.CreateMessage(ctx, database.MessageRecord{
					WorkspaceID:    runtime.workspaceID,
					ConversationID: runtime.conversationID,
					Role:           "assistant",
					Content:        summary,
					RunID:          &runID,
					CreatedAt:      now,
				})
			}
			if err == nil {
				_ = m.publish(ctx, RunFromRecord(record), "agent.run.status_changed", RunFromRecord(record))
			}
			m.deleteRuntime(runtime.runID)
			return
		}
		if strings.TrimSpace(result.Text) != "" {
			runID := runtime.runID
			_, _ = m.store.CreateMessage(ctx, database.MessageRecord{
				WorkspaceID:    runtime.workspaceID,
				ConversationID: runtime.conversationID,
				Role:           "assistant",
				Content:        strings.TrimSpace(result.Text),
				RunID:          &runID,
			})
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
		output, execErr := m.executeTool(ctx, runtime.workspaceRoot, record)
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
		if policy, ok := staticToolPolicy(request.Name); ok && policy == toolRequiresApproval {
			return true, ToolStatusWaitingApproval, "{}"
		}
		return false, ToolStatusPending, "{}"
	}
}

func (m *Manager) executeTool(ctx context.Context, workspaceRoot string, record database.AgentToolCallRecord) (string, error) {
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
		}
		if err := json.Unmarshal([]byte(record.InputJSON), &args); err != nil {
			return "", err
		}
		results, err := m.files.Search(workspaceRoot, args.Query)
		if err != nil {
			return "", err
		}
		if len(results) > 25 {
			results = results[:25]
		}
		return openToolJSON(map[string]any{"results": results}), nil
	case "read_file":
		var args struct {
			Path string `json:"path"`
		}
		if err := json.Unmarshal([]byte(record.InputJSON), &args); err != nil {
			return "", err
		}
		if isSecretPath(args.Path) {
			return "", ErrSecretPath
		}
		file, err := m.files.Read(workspaceRoot, args.Path)
		if err != nil {
			return "", err
		}
		return openToolJSON(file), nil
	case "git_status":
		status, err := m.git.Status(ctx, workspaceRoot)
		if err != nil {
			return "", err
		}
		return openToolJSON(status), nil
	case "git_diff":
		var args struct {
			Path string `json:"path"`
		}
		if err := json.Unmarshal([]byte(record.InputJSON), &args); err != nil {
			return "", err
		}
		diff, err := m.git.Diff(ctx, workspaceRoot, args.Path)
		if err != nil {
			return "", err
		}
		return openToolJSON(diff), nil
	case "run_command":
		return m.executeCommandTool(record, workspaceRoot)
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
		return "", fmt.Errorf("unknown tool: %s", record.Name)
	}
}

func (m *Manager) executeCommandTool(record database.AgentToolCallRecord, workspaceRoot string) (string, error) {
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
		Cwd:         workspaceRoot,
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
	result := <-done
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
		if strings.TrimSpace(text) == "" {
			return
		}
		_ = m.publish(ctx, run, "agent.delta", map[string]string{"runId": run.ID, "text": text})
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

func isSecretPath(relPath string) bool {
	clean := filepath.ToSlash(filepath.Clean(relPath))
	name := filepath.Base(clean)
	if name == ".env" || strings.HasPrefix(name, ".env.") || name == ".npmrc" || name == ".pypirc" || name == ".netrc" || name == "id_rsa" || name == "id_ed25519" {
		return true
	}
	return strings.HasSuffix(name, ".pem") || strings.HasSuffix(name, ".key")
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
		Decision:         record.Decision,
		StartedAt:        record.StartedAt,
		FinishedAt:       record.FinishedAt,
		CreatedAt:        record.CreatedAt,
	}
}
