package agent

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/phamtanminhtien/patchpilot/internal/database"
)

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
	m.cancelActiveToolCalls(ctx, workspaceID, conversationID, runID)
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

func (m *Manager) cancelActiveToolCalls(ctx context.Context, workspaceID, conversationID, runID string) {
	calls, err := m.store.ListAgentToolCalls(ctx, workspaceID, runID)
	if err != nil {
		return
	}
	finished := time.Now().UTC()
	output := openToolJSON(map[string]string{"status": "canceled"})
	run := Run{ID: runID, WorkspaceID: workspaceID, ConversationID: conversationID}
	for _, call := range calls {
		switch call.Status {
		case ToolStatusWaitingApproval:
			updated, err := m.store.UpdateAgentToolCall(ctx, workspaceID, runID, call.ID, map[string]any{
				"status":      ToolStatusRejected,
				"decision":    ToolStatusRejected,
				"output_json": output,
				"finished_at": finished,
			})
			if err == nil {
				_ = m.publish(ctx, run, "agent.tool.finished", ToolCallFromRecord(updated))
			}
		case ToolStatusPending, ToolStatusApproved, ToolStatusRunning:
			updated, err := m.store.FinishAgentToolCall(ctx, workspaceID, runID, call.ID, ToolStatusFailed, output, finished)
			if err == nil {
				_ = m.publish(ctx, run, "agent.tool.finished", ToolCallFromRecord(updated))
			}
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
