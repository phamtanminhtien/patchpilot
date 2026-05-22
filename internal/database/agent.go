package database

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

type AgentTaskRecord struct {
	ID              string     `gorm:"primaryKey;column:id"`
	WorkspaceID     string     `gorm:"column:workspace_id;not null;index"`
	SessionID       *string    `gorm:"column:session_id"`
	Prompt          string     `gorm:"column:prompt;not null"`
	Model           string     `gorm:"column:model;not null"`
	ReasoningEffort string     `gorm:"column:reasoning_effort;not null"`
	Status          string     `gorm:"column:status;not null;index"`
	Summary         string     `gorm:"column:summary;not null"`
	Error           *string    `gorm:"column:error"`
	StartedAt       *time.Time `gorm:"column:started_at"`
	FinishedAt      *time.Time `gorm:"column:finished_at"`
	CreatedAt       time.Time  `gorm:"column:created_at;not null;index"`
	UpdatedAt       time.Time  `gorm:"column:updated_at;not null;index"`
}

func (AgentTaskRecord) TableName() string {
	return "agent_tasks"
}

type AgentTaskEventRecord struct {
	ID          string    `gorm:"primaryKey;column:id"`
	WorkspaceID string    `gorm:"column:workspace_id;not null;index"`
	TaskID      string    `gorm:"column:task_id;not null;index"`
	Type        string    `gorm:"column:type;not null;index"`
	PayloadJSON string    `gorm:"column:payload_json;not null"`
	CreatedAt   time.Time `gorm:"column:created_at;not null;index"`
}

func (AgentTaskEventRecord) TableName() string {
	return "agent_task_events"
}

type AgentToolCallRecord struct {
	ID               string     `gorm:"primaryKey;column:id"`
	WorkspaceID      string     `gorm:"column:workspace_id;not null;index"`
	TaskID           string     `gorm:"column:task_id;not null;index"`
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

func (s *Store) CreateAgentTask(ctx context.Context, task AgentTaskRecord) (AgentTaskRecord, error) {
	if task.ID == "" {
		id, err := newPrefixedID("task_")
		if err != nil {
			return AgentTaskRecord{}, err
		}
		task.ID = id
	}
	now := time.Now().UTC()
	if task.CreatedAt.IsZero() {
		task.CreatedAt = now
	}
	if task.UpdatedAt.IsZero() {
		task.UpdatedAt = task.CreatedAt
	}
	if err := s.db.WithContext(ctx).Create(&task).Error; err != nil {
		return AgentTaskRecord{}, err
	}
	return task, nil
}

func (s *Store) GetAgentTask(ctx context.Context, workspaceID, taskID string) (AgentTaskRecord, error) {
	var task AgentTaskRecord
	if err := s.db.WithContext(ctx).First(&task, "workspace_id = ? AND id = ?", workspaceID, taskID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return AgentTaskRecord{}, ErrNotFound
		}
		return AgentTaskRecord{}, err
	}
	return task, nil
}

func (s *Store) ListAgentTasks(ctx context.Context, workspaceID string) ([]AgentTaskRecord, error) {
	var tasks []AgentTaskRecord
	if err := s.db.WithContext(ctx).
		Where("workspace_id = ?", workspaceID).
		Order("created_at DESC, id DESC").
		Find(&tasks).Error; err != nil {
		return nil, err
	}
	return tasks, nil
}

func (s *Store) UpdateAgentTask(ctx context.Context, workspaceID, taskID string, updates map[string]any) (AgentTaskRecord, error) {
	updates["updated_at"] = time.Now().UTC()
	if err := s.db.WithContext(ctx).Model(&AgentTaskRecord{}).
		Where("workspace_id = ? AND id = ?", workspaceID, taskID).
		Updates(updates).Error; err != nil {
		return AgentTaskRecord{}, err
	}
	return s.GetAgentTask(ctx, workspaceID, taskID)
}

func (s *Store) CreateAgentTaskEvent(ctx context.Context, event AgentTaskEventRecord) (AgentTaskEventRecord, error) {
	if event.ID == "" {
		id, err := newPrefixedID("evt_")
		if err != nil {
			return AgentTaskEventRecord{}, err
		}
		event.ID = id
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}
	if err := s.db.WithContext(ctx).Create(&event).Error; err != nil {
		return AgentTaskEventRecord{}, err
	}
	return event, nil
}

func (s *Store) ListAgentTaskEvents(ctx context.Context, workspaceID, taskID string) ([]AgentTaskEventRecord, error) {
	var events []AgentTaskEventRecord
	query := s.db.WithContext(ctx).Where("workspace_id = ?", workspaceID)
	if taskID != "" {
		query = query.Where("task_id = ?", taskID)
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

func (s *Store) FinishAgentToolCall(ctx context.Context, workspaceID, taskID, callID, status, outputJSON string, finishedAt time.Time) (AgentToolCallRecord, error) {
	if err := s.db.WithContext(ctx).Model(&AgentToolCallRecord{}).
		Where("workspace_id = ? AND task_id = ? AND id = ?", workspaceID, taskID, callID).
		Updates(map[string]any{"status": status, "output_json": outputJSON, "finished_at": finishedAt}).Error; err != nil {
		return AgentToolCallRecord{}, err
	}
	var call AgentToolCallRecord
	if err := s.db.WithContext(ctx).First(&call, "workspace_id = ? AND task_id = ? AND id = ?", workspaceID, taskID, callID).Error; err != nil {
		return AgentToolCallRecord{}, err
	}
	return call, nil
}

func (s *Store) UpdateAgentToolCall(ctx context.Context, workspaceID, taskID, callID string, updates map[string]any) (AgentToolCallRecord, error) {
	if err := s.db.WithContext(ctx).Model(&AgentToolCallRecord{}).
		Where("workspace_id = ? AND task_id = ? AND id = ?", workspaceID, taskID, callID).
		Updates(updates).Error; err != nil {
		return AgentToolCallRecord{}, err
	}
	var call AgentToolCallRecord
	if err := s.db.WithContext(ctx).First(&call, "workspace_id = ? AND task_id = ? AND id = ?", workspaceID, taskID, callID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return AgentToolCallRecord{}, ErrNotFound
		}
		return AgentToolCallRecord{}, err
	}
	return call, nil
}

func (s *Store) GetAgentToolCall(ctx context.Context, workspaceID, taskID, callID string) (AgentToolCallRecord, error) {
	var call AgentToolCallRecord
	if err := s.db.WithContext(ctx).First(&call, "workspace_id = ? AND task_id = ? AND id = ?", workspaceID, taskID, callID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return AgentToolCallRecord{}, ErrNotFound
		}
		return AgentToolCallRecord{}, err
	}
	return call, nil
}

func (s *Store) ListAgentToolCalls(ctx context.Context, workspaceID, taskID string) ([]AgentToolCallRecord, error) {
	var calls []AgentToolCallRecord
	if err := s.db.WithContext(ctx).
		Where("workspace_id = ? AND task_id = ?", workspaceID, taskID).
		Order("created_at ASC, batch_id ASC, sequence ASC, id ASC").
		Find(&calls).Error; err != nil {
		return nil, err
	}
	return calls, nil
}
