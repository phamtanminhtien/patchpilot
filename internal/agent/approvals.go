package agent

import (
	"context"
	"errors"
	"time"

	"github.com/phamtanminhtien/patchpilot/internal/database"
	"github.com/phamtanminhtien/patchpilot/internal/events"
)

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
		callRequiresApproval, initialStatus, initialOutput := m.prepareToolRequestWithPermissions(ctx, runtime.workspaceRoot, runtime.permissions, request)
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
