package agent

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"

	"github.com/phamtanminhtien/patchpilot/internal/database"
)

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
