package database

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

type AgentRunRecord struct {
	ID               string     `gorm:"primaryKey;column:id"`
	WorkspaceID      string     `gorm:"column:workspace_id;not null;index"`
	ConversationID   string     `gorm:"column:conversation_id;not null;index"`
	TriggerMessageID string     `gorm:"column:trigger_message_id;not null;index"`
	Model            string     `gorm:"column:model;not null"`
	ReasoningEffort  string     `gorm:"column:reasoning_effort;not null"`
	Status           string     `gorm:"column:status;not null;index"`
	Summary          string     `gorm:"column:summary;not null"`
	Error            *string    `gorm:"column:error"`
	StartedAt        *time.Time `gorm:"column:started_at"`
	FinishedAt       *time.Time `gorm:"column:finished_at"`
	CreatedAt        time.Time  `gorm:"column:created_at;not null;index"`
	UpdatedAt        time.Time  `gorm:"column:updated_at;not null;index"`
}

func (AgentRunRecord) TableName() string {
	return "agent_runs"
}

type AgentRunEventRecord struct {
	ID          string    `gorm:"primaryKey;column:id"`
	WorkspaceID string    `gorm:"column:workspace_id;not null;index"`
	RunID       string    `gorm:"column:run_id;not null;index"`
	Type        string    `gorm:"column:type;not null;index"`
	PayloadJSON string    `gorm:"column:payload_json;not null"`
	CreatedAt   time.Time `gorm:"column:created_at;not null;index"`
}

func (AgentRunEventRecord) TableName() string {
	return "agent_run_events"
}

type AgentToolCallRecord struct {
	ID               string     `gorm:"primaryKey;column:id"`
	WorkspaceID      string     `gorm:"column:workspace_id;not null;index"`
	RunID            string     `gorm:"column:run_id;not null;index"`
	BatchID          string     `gorm:"column:batch_id;not null;index"`
	Sequence         int        `gorm:"column:sequence;not null"`
	ProviderCallID   string     `gorm:"column:provider_call_id;not null"`
	Name             string     `gorm:"column:name;not null"`
	InputJSON        string     `gorm:"column:input_json;not null"`
	OutputJSON       string     `gorm:"column:output_json;not null"`
	Status           string     `gorm:"column:status;not null;index"`
	RequiresApproval bool       `gorm:"column:requires_approval;not null"`
	Decision         *string    `gorm:"column:decision"`
	StartedAt        *time.Time `gorm:"column:started_at"`
	FinishedAt       *time.Time `gorm:"column:finished_at"`
	CreatedAt        time.Time  `gorm:"column:created_at;not null;index"`
}

func (AgentToolCallRecord) TableName() string {
	return "agent_tool_calls"
}

func (s *Store) CreateAgentRun(ctx context.Context, run AgentRunRecord) (AgentRunRecord, error) {
	if run.ID == "" {
		id, err := newPrefixedID("run_")
		if err != nil {
			return AgentRunRecord{}, err
		}
		run.ID = id
	}
	now := time.Now().UTC()
	if run.CreatedAt.IsZero() {
		run.CreatedAt = now
	}
	if run.UpdatedAt.IsZero() {
		run.UpdatedAt = run.CreatedAt
	}
	if err := s.db.WithContext(ctx).Create(&run).Error; err != nil {
		return AgentRunRecord{}, err
	}
	return run, nil
}

func (s *Store) GetAgentRun(ctx context.Context, workspaceID, conversationID, runID string) (AgentRunRecord, error) {
	var run AgentRunRecord
	if err := s.db.WithContext(ctx).First(&run, "workspace_id = ? AND conversation_id = ? AND id = ?", workspaceID, conversationID, runID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return AgentRunRecord{}, ErrNotFound
		}
		return AgentRunRecord{}, err
	}
	return run, nil
}

func (s *Store) ListAgentRuns(ctx context.Context, workspaceID, conversationID string) ([]AgentRunRecord, error) {
	var runs []AgentRunRecord
	if err := s.db.WithContext(ctx).
		Where("workspace_id = ? AND conversation_id = ?", workspaceID, conversationID).
		Order("created_at ASC, id ASC").
		Find(&runs).Error; err != nil {
		return nil, err
	}
	return runs, nil
}

func (s *Store) UpdateAgentRun(ctx context.Context, workspaceID, conversationID, runID string, updates map[string]any) (AgentRunRecord, error) {
	updates["updated_at"] = time.Now().UTC()
	if err := s.db.WithContext(ctx).Model(&AgentRunRecord{}).
		Where("workspace_id = ? AND conversation_id = ? AND id = ?", workspaceID, conversationID, runID).
		Updates(updates).Error; err != nil {
		return AgentRunRecord{}, err
	}
	return s.GetAgentRun(ctx, workspaceID, conversationID, runID)
}

func (s *Store) CreateAgentRunEvent(ctx context.Context, event AgentRunEventRecord) (AgentRunEventRecord, error) {
	if event.ID == "" {
		id, err := newPrefixedID("evt_")
		if err != nil {
			return AgentRunEventRecord{}, err
		}
		event.ID = id
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}
	if err := s.db.WithContext(ctx).Create(&event).Error; err != nil {
		return AgentRunEventRecord{}, err
	}
	return event, nil
}

func (s *Store) ListAgentRunEvents(ctx context.Context, workspaceID, runID string) ([]AgentRunEventRecord, error) {
	var events []AgentRunEventRecord
	query := s.db.WithContext(ctx).Where("workspace_id = ?", workspaceID)
	if runID != "" {
		query = query.Where("run_id = ?", runID)
	}
	if err := query.Order("created_at ASC, id ASC").Find(&events).Error; err != nil {
		return nil, err
	}
	return events, nil
}

func (s *Store) CreateAgentToolCall(ctx context.Context, call AgentToolCallRecord) (AgentToolCallRecord, error) {
	if call.ID == "" {
		id, err := newPrefixedID("evt_")
		if err != nil {
			return AgentToolCallRecord{}, err
		}
		call.ID = id
	}
	if call.CreatedAt.IsZero() {
		call.CreatedAt = time.Now().UTC()
	}
	if err := s.db.WithContext(ctx).Create(&call).Error; err != nil {
		return AgentToolCallRecord{}, err
	}
	return call, nil
}

func (s *Store) FinishAgentToolCall(ctx context.Context, workspaceID, runID, callID, status, outputJSON string, finishedAt time.Time) (AgentToolCallRecord, error) {
	if err := s.db.WithContext(ctx).Model(&AgentToolCallRecord{}).
		Where("workspace_id = ? AND run_id = ? AND id = ?", workspaceID, runID, callID).
		Updates(map[string]any{"status": status, "output_json": outputJSON, "finished_at": finishedAt}).Error; err != nil {
		return AgentToolCallRecord{}, err
	}
	var call AgentToolCallRecord
	if err := s.db.WithContext(ctx).First(&call, "workspace_id = ? AND run_id = ? AND id = ?", workspaceID, runID, callID).Error; err != nil {
		return AgentToolCallRecord{}, err
	}
	return call, nil
}

func (s *Store) UpdateAgentToolCall(ctx context.Context, workspaceID, runID, callID string, updates map[string]any) (AgentToolCallRecord, error) {
	if err := s.db.WithContext(ctx).Model(&AgentToolCallRecord{}).
		Where("workspace_id = ? AND run_id = ? AND id = ?", workspaceID, runID, callID).
		Updates(updates).Error; err != nil {
		return AgentToolCallRecord{}, err
	}
	var call AgentToolCallRecord
	if err := s.db.WithContext(ctx).First(&call, "workspace_id = ? AND run_id = ? AND id = ?", workspaceID, runID, callID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return AgentToolCallRecord{}, ErrNotFound
		}
		return AgentToolCallRecord{}, err
	}
	return call, nil
}

func (s *Store) GetAgentToolCall(ctx context.Context, workspaceID, runID, callID string) (AgentToolCallRecord, error) {
	var call AgentToolCallRecord
	if err := s.db.WithContext(ctx).First(&call, "workspace_id = ? AND run_id = ? AND id = ?", workspaceID, runID, callID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return AgentToolCallRecord{}, ErrNotFound
		}
		return AgentToolCallRecord{}, err
	}
	return call, nil
}

func (s *Store) ListAgentToolCalls(ctx context.Context, workspaceID, runID string) ([]AgentToolCallRecord, error) {
	var calls []AgentToolCallRecord
	if err := s.db.WithContext(ctx).
		Where("workspace_id = ? AND run_id = ?", workspaceID, runID).
		Order("created_at ASC, batch_id ASC, sequence ASC, id ASC").
		Find(&calls).Error; err != nil {
		return nil, err
	}
	return calls, nil
}
