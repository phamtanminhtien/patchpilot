package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
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
	ErrTaskNotFound        = errors.New("agent task not found")
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

type TaskStatus string

const (
	StatusQueued          TaskStatus = "queued"
	StatusRunning         TaskStatus = "running"
	StatusWaitingApproval TaskStatus = "waiting_approval"
	StatusDone            TaskStatus = "done"
	StatusFailed          TaskStatus = "failed"
)

type CreateTaskInput struct {
	Prompt          string
	Model           string
	ReasoningEffort string
}

type Task struct {
	ID              string     `json:"id"`
	WorkspaceID     string     `json:"workspaceId"`
	Prompt          string     `json:"prompt"`
	Model           string     `json:"model"`
	ReasoningEffort string     `json:"reasoningEffort"`
	Status          string     `json:"status"`
	Plan            string     `json:"plan"`
	Summary         string     `json:"summary"`
	Error           *string    `json:"error"`
	GeneratedPatch  string     `json:"generatedPatch"`
	StartedAt       *time.Time `json:"startedAt"`
	FinishedAt      *time.Time `json:"finishedAt"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
}

type TaskEvent struct {
	ID          string          `json:"id"`
	WorkspaceID string          `json:"workspaceId"`
	TaskID      string          `json:"taskId"`
	Type        string          `json:"type"`
	Payload     json.RawMessage `json:"payload"`
	CreatedAt   time.Time       `json:"createdAt"`
}

type ToolCall struct {
	ID          string     `json:"id"`
	WorkspaceID string     `json:"workspaceId"`
	TaskID      string     `json:"taskId"`
	Name        string     `json:"name"`
	Input       string     `json:"input"`
	Output      string     `json:"output"`
	Status      string     `json:"status"`
	StartedAt   *time.Time `json:"startedAt"`
	FinishedAt  *time.Time `json:"finishedAt"`
	CreatedAt   time.Time  `json:"createdAt"`
}

type Patch struct {
	ID          string     `json:"id"`
	WorkspaceID string     `json:"workspaceId"`
	TaskID      string     `json:"taskId"`
	BaseCommit  *string    `json:"baseCommit"`
	Diff        string     `json:"diff"`
	Summary     string     `json:"summary"`
	Status      string     `json:"status"`
	AppliedAt   *time.Time `json:"appliedAt"`
	CreatedAt   time.Time  `json:"createdAt"`
}

type Detail struct {
	Task      Task        `json:"task"`
	Events    []TaskEvent `json:"events"`
	ToolCalls []ToolCall  `json:"toolCalls"`
	Patches   []Patch     `json:"patches"`
}

type Provider interface {
	Generate(context.Context, ProviderRequest, *Tools, Stream) (ProviderResult, error)
	Configured() bool
}

type ProviderRequest struct {
	Task           Task
	WorkspaceRoot  string
	GitStatus      string
	WorkspaceFiles []string
}

type ProviderResult struct {
	Plan    string
	Summary string
	Patch   string
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
}

func NewManager(store *database.Store, files *filestore.Service, git *gitrepo.Client, run *runner.Runner, hub *events.Hub, provider Provider) *Manager {
	return &Manager{store: store, files: files, git: git, runner: run, events: hub, provider: provider}
}

func (m *Manager) Create(ctx context.Context, workspaceID, workspaceRoot string, input CreateTaskInput) (Task, error) {
	input.Prompt = strings.TrimSpace(input.Prompt)
	input.Model = strings.TrimSpace(input.Model)
	input.ReasoningEffort = strings.TrimSpace(input.ReasoningEffort)
	if input.Prompt == "" {
		return Task{}, ErrEmptyPrompt
	}
	if _, ok := supportedModels[input.Model]; !ok {
		return Task{}, ErrInvalidModel
	}
	if _, ok := supportedEfforts[input.ReasoningEffort]; !ok {
		return Task{}, ErrInvalidEffort
	}
	if m.provider == nil || !m.provider.Configured() {
		return Task{}, ErrProviderUnavailable
	}

	task, err := m.store.CreateAgentTask(ctx, database.AgentTaskRecord{
		WorkspaceID:     workspaceID,
		Prompt:          input.Prompt,
		Model:           input.Model,
		ReasoningEffort: input.ReasoningEffort,
		Status:          string(StatusQueued),
	})
	if err != nil {
		return Task{}, err
	}
	publicTask := TaskFromRecord(task)
	go m.run(task.WorkspaceID, workspaceRoot, task.ID)
	return publicTask, nil
}

func (m *Manager) List(ctx context.Context, workspaceID string) ([]Task, error) {
	records, err := m.store.ListAgentTasks(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	tasks := make([]Task, 0, len(records))
	for _, record := range records {
		tasks = append(tasks, TaskFromRecord(record))
	}
	return tasks, nil
}

func (m *Manager) Detail(ctx context.Context, workspaceID, taskID string) (Detail, error) {
	task, err := m.store.GetAgentTask(ctx, workspaceID, taskID)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return Detail{}, ErrTaskNotFound
		}
		return Detail{}, err
	}
	eventRecords, err := m.store.ListAgentTaskEvents(ctx, workspaceID, taskID)
	if err != nil {
		return Detail{}, err
	}
	toolRecords, err := m.store.ListAgentToolCalls(ctx, workspaceID, taskID)
	if err != nil {
		return Detail{}, err
	}
	patchRecords, err := m.store.ListPatches(ctx, workspaceID, taskID)
	if err != nil {
		return Detail{}, err
	}
	return Detail{
		Task:      TaskFromRecord(task),
		Events:    EventsFromRecords(eventRecords),
		ToolCalls: ToolCallsFromRecords(toolRecords),
		Patches:   PatchesFromRecords(patchRecords),
	}, nil
}

func (m *Manager) Cancel(ctx context.Context, workspaceID, taskID string) (Task, error) {
	now := time.Now().UTC()
	task, err := m.store.UpdateAgentTask(ctx, workspaceID, taskID, map[string]any{
		"status":      string(StatusFailed),
		"error":       "Task was cancelled",
		"finished_at": now,
	})
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return Task{}, ErrTaskNotFound
		}
		return Task{}, err
	}
	publicTask := TaskFromRecord(task)
	_ = m.publish(ctx, publicTask, "agent.task.status_changed", publicTask)
	return publicTask, nil
}

func (m *Manager) run(workspaceID, workspaceRoot, taskID string) {
	ctx := context.Background()
	startedAt := time.Now().UTC()
	record, err := m.store.UpdateAgentTask(ctx, workspaceID, taskID, map[string]any{
		"status":     string(StatusRunning),
		"started_at": startedAt,
	})
	if err != nil {
		return
	}
	task := TaskFromRecord(record)
	_ = m.publish(ctx, task, "agent.task.status_changed", task)
	_ = m.publish(ctx, task, "agent.delta", map[string]string{"taskId": task.ID, "text": "Preparing workspace context."})

	status, err := m.captureGitStatus(ctx, task, workspaceRoot)
	if err != nil {
		m.fail(ctx, task, err)
		return
	}

	workspaceFiles := m.captureWorkspaceContext(ctx, task, workspaceRoot)
	result, err := m.provider.Generate(ctx, ProviderRequest{
		Task:           task,
		WorkspaceRoot:  workspaceRoot,
		GitStatus:      status.Porcelain,
		WorkspaceFiles: workspaceFiles,
	}, m.tools(task, workspaceRoot), m.stream(task))
	if err != nil {
		m.fail(ctx, task, err)
		return
	}
	if strings.TrimSpace(result.Patch) != "" {
		if err := m.git.CheckPatch(ctx, workspaceRoot, result.Patch); err != nil {
			_, _ = m.store.UpdateAgentTask(ctx, workspaceID, taskID, map[string]any{
				"plan":            strings.TrimSpace(result.Plan),
				"summary":         strings.TrimSpace(result.Summary),
				"generated_patch": result.Patch,
			})
			m.fail(ctx, task, fmt.Errorf("generated patch failed git apply check: %w", err))
			return
		}
	}

	updates := map[string]any{
		"plan":            strings.TrimSpace(result.Plan),
		"summary":         strings.TrimSpace(result.Summary),
		"generated_patch": result.Patch,
	}
	if strings.TrimSpace(result.Patch) != "" {
		updates["status"] = string(StatusWaitingApproval)
	} else {
		now := time.Now().UTC()
		updates["status"] = string(StatusDone)
		updates["finished_at"] = now
	}
	record, err = m.store.UpdateAgentTask(ctx, workspaceID, taskID, updates)
	if err != nil {
		return
	}
	task = TaskFromRecord(record)
	_ = m.publish(ctx, task, "agent.task.status_changed", task)

	if task.GeneratedPatch != "" {
		patch, err := m.store.CreatePatch(ctx, database.PatchRecord{
			WorkspaceID: workspaceID,
			TaskID:      task.ID,
			Diff:        task.GeneratedPatch,
			Summary:     task.Summary,
			Status:      "proposed",
		})
		if err == nil {
			publicPatch := PatchFromRecord(patch)
			_ = m.publish(ctx, task, "patch.created", publicPatch)
			_ = m.publish(ctx, task, "agent.approval_required", map[string]any{"taskId": task.ID, "patchId": publicPatch.ID})
		}
	}
}

func (m *Manager) captureGitStatus(ctx context.Context, task Task, workspaceRoot string) (gitrepo.Status, error) {
	started := time.Now().UTC()
	call, err := m.store.CreateAgentToolCall(ctx, database.AgentToolCallRecord{
		WorkspaceID: task.WorkspaceID,
		TaskID:      task.ID,
		Name:        "git_status",
		InputJSON:   "{}",
		OutputJSON:  "{}",
		Status:      "running",
		StartedAt:   &started,
	})
	if err == nil {
		_ = m.publish(ctx, task, "agent.tool.started", ToolCallFromRecord(call))
	}
	status, statusErr := m.git.Status(ctx, workspaceRoot)
	output, _ := json.Marshal(status)
	finished := time.Now().UTC()
	if err == nil {
		state := "finished"
		if statusErr != nil {
			state = "failed"
			output, _ = json.Marshal(map[string]string{"error": statusErr.Error()})
		}
		finishedCall, finishErr := m.store.FinishAgentToolCall(ctx, task.WorkspaceID, task.ID, call.ID, state, string(output), finished)
		if finishErr == nil {
			_ = m.publish(ctx, task, "agent.tool.finished", ToolCallFromRecord(finishedCall))
		}
	}
	return status, statusErr
}

func (m *Manager) captureWorkspaceContext(ctx context.Context, task Task, workspaceRoot string) []string {
	started := time.Now().UTC()
	call, err := m.store.CreateAgentToolCall(ctx, database.AgentToolCallRecord{
		WorkspaceID: task.WorkspaceID,
		TaskID:      task.ID,
		Name:        "workspace_context",
		InputJSON:   "{}",
		OutputJSON:  "{}",
		Status:      "running",
		StartedAt:   &started,
	})
	if err == nil {
		_ = m.publish(ctx, task, "agent.tool.started", ToolCallFromRecord(call))
	}

	entries, indexErr := m.files.Index(workspaceRoot)
	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		paths = append(paths, entry.Path)
	}
	output, _ := json.Marshal(map[string]any{
		"fileCount":        len(paths),
		"contextFileCount": 0,
	})
	finished := time.Now().UTC()
	if err == nil {
		state := "finished"
		if indexErr != nil {
			state = "failed"
			output, _ = json.Marshal(map[string]string{"error": indexErr.Error()})
		}
		finishedCall, finishErr := m.store.FinishAgentToolCall(ctx, task.WorkspaceID, task.ID, call.ID, state, string(output), finished)
		if finishErr == nil {
			_ = m.publish(ctx, task, "agent.tool.finished", ToolCallFromRecord(finishedCall))
		}
	}
	if indexErr != nil {
		return nil
	}
	if len(paths) > 200 {
		paths = paths[:200]
	}
	return paths
}

func (m *Manager) fail(ctx context.Context, task Task, failure error) {
	message := failure.Error()
	now := time.Now().UTC()
	record, err := m.store.UpdateAgentTask(ctx, task.WorkspaceID, task.ID, map[string]any{
		"status":      string(StatusFailed),
		"error":       message,
		"finished_at": now,
	})
	if err != nil {
		return
	}
	publicTask := TaskFromRecord(record)
	_ = m.publish(ctx, publicTask, "agent.task.status_changed", publicTask)
}

func (m *Manager) publish(ctx context.Context, task Task, eventType string, payload any) error {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	record, err := m.store.CreateAgentTaskEvent(ctx, database.AgentTaskEventRecord{
		WorkspaceID: task.WorkspaceID,
		TaskID:      task.ID,
		Type:        eventType,
		PayloadJSON: string(payloadBytes),
	})
	if err != nil {
		return err
	}
	m.events.Publish(events.Event{
		ID:          record.ID,
		WorkspaceID: task.WorkspaceID,
		Type:        eventType,
		CreatedAt:   record.CreatedAt,
		Payload:     payload,
	})
	return nil
}

func (m *Manager) stream(task Task) Stream {
	return streamFunc(func(ctx context.Context, text string) {
		if strings.TrimSpace(text) == "" {
			return
		}
		_ = m.publish(ctx, task, "agent.delta", map[string]string{"taskId": task.ID, "text": text})
	})
}

type streamFunc func(context.Context, string)

func (s streamFunc) Delta(ctx context.Context, text string) {
	s(ctx, text)
}

func (m *Manager) tools(task Task, workspaceRoot string) *Tools {
	return &Tools{task: task, workspaceRoot: workspaceRoot, store: m.store, files: m.files, git: m.git, runner: m.runner, publish: m.publish}
}

type Tools struct {
	task          Task
	workspaceRoot string
	store         *database.Store
	files         *filestore.Service
	git           *gitrepo.Client
	runner        *runner.Runner
	publish       func(context.Context, Task, string, any) error
}

func (t *Tools) SearchFiles(ctx context.Context, query string) ([]filestore.SearchResult, error) {
	return t.files.Search(t.workspaceRoot, query)
}

func (t *Tools) ReadFile(root, relPath string) (filestore.File, error) {
	_ = root
	if isSecretPath(relPath) {
		return filestore.File{}, ErrSecretPath
	}
	return t.files.Read(t.workspaceRoot, relPath)
}

func (t *Tools) GitDiff(ctx context.Context, relPath string) (gitrepo.Diff, error) {
	return t.git.Diff(ctx, t.workspaceRoot, relPath)
}

func (t *Tools) GitStatus(ctx context.Context) (gitrepo.Status, error) {
	return t.git.Status(ctx, t.workspaceRoot)
}

func (t *Tools) RunCommand(ctx context.Context, command string) (runner.SafetyDecision, error) {
	decision, err := runner.Classify(command)
	if err != nil {
		return runner.SafetyDecision{}, err
	}
	if decision.Level != runner.SafetyAllowed {
		_ = t.publish(ctx, t.task, "agent.approval_required", map[string]any{"taskId": t.task.ID, "command": command, "decision": decision})
		return decision, fmt.Errorf("command requires approval: %s", decision.Reason)
	}
	return decision, nil
}

func isSecretPath(relPath string) bool {
	clean := filepath.ToSlash(filepath.Clean(relPath))
	name := filepath.Base(clean)
	if name == ".env" || strings.HasPrefix(name, ".env.") || name == ".npmrc" || name == ".pypirc" || name == ".netrc" || name == "id_rsa" || name == "id_ed25519" {
		return true
	}
	return strings.HasSuffix(name, ".pem") || strings.HasSuffix(name, ".key")
}

func TaskFromRecord(record database.AgentTaskRecord) Task {
	return Task{
		ID:              record.ID,
		WorkspaceID:     record.WorkspaceID,
		Prompt:          record.Prompt,
		Model:           record.Model,
		ReasoningEffort: record.ReasoningEffort,
		Status:          record.Status,
		Plan:            record.Plan,
		Summary:         record.Summary,
		Error:           record.Error,
		GeneratedPatch:  record.GeneratedPatch,
		StartedAt:       record.StartedAt,
		FinishedAt:      record.FinishedAt,
		CreatedAt:       record.CreatedAt,
		UpdatedAt:       record.UpdatedAt,
	}
}

func EventsFromRecords(records []database.AgentTaskEventRecord) []TaskEvent {
	out := make([]TaskEvent, 0, len(records))
	for _, record := range records {
		out = append(out, TaskEventFromRecord(record))
	}
	return out
}

func TaskEventFromRecord(record database.AgentTaskEventRecord) TaskEvent {
	return TaskEvent{
		ID:          record.ID,
		WorkspaceID: record.WorkspaceID,
		TaskID:      record.TaskID,
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
		ID:          record.ID,
		WorkspaceID: record.WorkspaceID,
		TaskID:      record.TaskID,
		Name:        record.Name,
		Input:       record.InputJSON,
		Output:      record.OutputJSON,
		Status:      record.Status,
		StartedAt:   record.StartedAt,
		FinishedAt:  record.FinishedAt,
		CreatedAt:   record.CreatedAt,
	}
}

func PatchesFromRecords(records []database.PatchRecord) []Patch {
	out := make([]Patch, 0, len(records))
	for _, record := range records {
		out = append(out, PatchFromRecord(record))
	}
	return out
}

func PatchFromRecord(record database.PatchRecord) Patch {
	return Patch{
		ID:          record.ID,
		WorkspaceID: record.WorkspaceID,
		TaskID:      record.TaskID,
		BaseCommit:  record.BaseCommit,
		Diff:        record.Diff,
		Summary:     record.Summary,
		Status:      record.Status,
		AppliedAt:   record.AppliedAt,
		CreatedAt:   record.CreatedAt,
	}
}
