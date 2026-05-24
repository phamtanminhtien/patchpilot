package database

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
)

type ConversationRecord struct {
	ID                             string     `gorm:"primaryKey;column:id"`
	WorkspaceID                    string     `gorm:"column:workspace_id;not null;index"`
	Title                          string     `gorm:"column:title;not null"`
	HasRunningRun                  bool       `gorm:"column:has_running_run;not null"`
	LastMessageAt                  time.Time  `gorm:"column:last_message_at;not null;index"`
	ContextSummary                 string     `gorm:"column:context_summary;not null"`
	ContextSummaryThroughMessageID *string    `gorm:"column:context_summary_through_message_id"`
	ContextSummaryUpdatedAt        *time.Time `gorm:"column:context_summary_updated_at"`
	CreatedAt                      time.Time  `gorm:"column:created_at;not null"`
	UpdatedAt                      time.Time  `gorm:"column:updated_at;not null;index"`
}

func (ConversationRecord) TableName() string {
	return "conversations"
}

type MessageRecord struct {
	ID             string    `gorm:"primaryKey;column:id"`
	WorkspaceID    string    `gorm:"column:workspace_id;not null;index"`
	ConversationID string    `gorm:"column:conversation_id;not null;index"`
	Role           string    `gorm:"column:role;not null;index"`
	Content        string    `gorm:"column:content;not null"`
	RunID          *string   `gorm:"column:run_id;index"`
	CreatedAt      time.Time `gorm:"column:created_at;not null;index"`
}

func (MessageRecord) TableName() string {
	return "messages"
}

func (s *Store) CreateConversation(ctx context.Context, conversation ConversationRecord) (ConversationRecord, error) {
	if conversation.ID == "" {
		id, err := newPrefixedID("conv_")
		if err != nil {
			return ConversationRecord{}, err
		}
		conversation.ID = id
	}
	now := time.Now().UTC()
	if conversation.CreatedAt.IsZero() {
		conversation.CreatedAt = now
	}
	if conversation.UpdatedAt.IsZero() {
		conversation.UpdatedAt = conversation.CreatedAt
	}
	if conversation.LastMessageAt.IsZero() {
		conversation.LastMessageAt = conversation.CreatedAt
	}
	if err := s.db.WithContext(ctx).Create(&conversation).Error; err != nil {
		return ConversationRecord{}, err
	}
	return conversation, nil
}

func (s *Store) GetConversation(ctx context.Context, workspaceID, conversationID string) (ConversationRecord, error) {
	var conversation ConversationRecord
	if err := s.db.WithContext(ctx).First(&conversation, "workspace_id = ? AND id = ?", workspaceID, conversationID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ConversationRecord{}, ErrNotFound
		}
		return ConversationRecord{}, err
	}
	return conversation, nil
}

func (s *Store) ListConversations(ctx context.Context, workspaceID, query string) ([]ConversationRecord, error) {
	var conversations []ConversationRecord
	db := s.db.WithContext(ctx).
		Where("workspace_id = ?", workspaceID)
	if trimmed := strings.TrimSpace(query); trimmed != "" {
		db = db.Where("LOWER(title) LIKE ?", "%"+strings.ToLower(trimmed)+"%")
	}
	if err := db.
		Order("last_message_at DESC, updated_at DESC, id DESC").
		Find(&conversations).Error; err != nil {
		return nil, err
	}
	return conversations, nil
}

func (s *Store) UpdateConversation(ctx context.Context, workspaceID, conversationID string, updates map[string]any) (ConversationRecord, error) {
	updates["updated_at"] = time.Now().UTC()
	if err := s.db.WithContext(ctx).Model(&ConversationRecord{}).
		Where("workspace_id = ? AND id = ?", workspaceID, conversationID).
		Updates(updates).Error; err != nil {
		return ConversationRecord{}, err
	}
	return s.GetConversation(ctx, workspaceID, conversationID)
}

func (s *Store) UpdateConversationContextSummary(ctx context.Context, workspaceID, conversationID, summary, throughMessageID string, updatedAt time.Time) (ConversationRecord, error) {
	if updatedAt.IsZero() {
		updatedAt = time.Now().UTC()
	}
	updates := map[string]any{
		"context_summary":                    summary,
		"context_summary_through_message_id": throughMessageID,
		"context_summary_updated_at":         updatedAt,
		"updated_at":                         updatedAt,
	}
	if err := s.db.WithContext(ctx).Model(&ConversationRecord{}).
		Where("workspace_id = ? AND id = ?", workspaceID, conversationID).
		Updates(updates).Error; err != nil {
		return ConversationRecord{}, err
	}
	return s.GetConversation(ctx, workspaceID, conversationID)
}

func (s *Store) RefreshConversationHasRunningRun(ctx context.Context, workspaceID, conversationID string) (ConversationRecord, error) {
	activeStatuses := []string{"queued", "running", "waiting_tool_approval"}
	var count int64
	if err := s.db.WithContext(ctx).Model(&AgentRunRecord{}).
		Where("workspace_id = ? AND conversation_id = ? AND status IN ?", workspaceID, conversationID, activeStatuses).
		Count(&count).Error; err != nil {
		return ConversationRecord{}, err
	}
	if err := s.db.WithContext(ctx).Model(&ConversationRecord{}).
		Where("workspace_id = ? AND id = ?", workspaceID, conversationID).
		Update("has_running_run", count > 0).Error; err != nil {
		return ConversationRecord{}, err
	}
	return s.GetConversation(ctx, workspaceID, conversationID)
}

func (s *Store) CreateMessage(ctx context.Context, message MessageRecord) (MessageRecord, error) {
	if message.ID == "" {
		id, err := newPrefixedID("msg_")
		if err != nil {
			return MessageRecord{}, err
		}
		message.ID = id
	}
	if message.CreatedAt.IsZero() {
		message.CreatedAt = time.Now().UTC()
	}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&message).Error; err != nil {
			return err
		}
		return tx.Model(&ConversationRecord{}).
			Where("workspace_id = ? AND id = ?", message.WorkspaceID, message.ConversationID).
			Updates(map[string]any{"last_message_at": message.CreatedAt, "updated_at": message.CreatedAt}).Error
	})
	if err != nil {
		return MessageRecord{}, err
	}
	return message, nil
}

func (s *Store) UpdateMessageRun(ctx context.Context, workspaceID, conversationID, messageID, runID string) (MessageRecord, error) {
	if err := s.db.WithContext(ctx).Model(&MessageRecord{}).
		Where("workspace_id = ? AND conversation_id = ? AND id = ?", workspaceID, conversationID, messageID).
		Update("run_id", runID).Error; err != nil {
		return MessageRecord{}, err
	}
	var message MessageRecord
	if err := s.db.WithContext(ctx).First(&message, "workspace_id = ? AND conversation_id = ? AND id = ?", workspaceID, conversationID, messageID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return MessageRecord{}, ErrNotFound
		}
		return MessageRecord{}, err
	}
	return message, nil
}

func (s *Store) ListMessages(ctx context.Context, workspaceID, conversationID string) ([]MessageRecord, error) {
	var messages []MessageRecord
	if err := s.db.WithContext(ctx).
		Where("workspace_id = ? AND conversation_id = ?", workspaceID, conversationID).
		Order("created_at ASC, id ASC").
		Find(&messages).Error; err != nil {
		return nil, err
	}
	return messages, nil
}

func (s *Store) ListMessagesAfter(ctx context.Context, workspaceID, conversationID, afterMessageID string) ([]MessageRecord, error) {
	if afterMessageID == "" {
		return s.ListMessages(ctx, workspaceID, conversationID)
	}
	var after MessageRecord
	if err := s.db.WithContext(ctx).
		First(&after, "workspace_id = ? AND conversation_id = ? AND id = ?", workspaceID, conversationID, afterMessageID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	var messages []MessageRecord
	if err := s.db.WithContext(ctx).
		Where("workspace_id = ? AND conversation_id = ? AND (created_at > ? OR (created_at = ? AND id > ?))", workspaceID, conversationID, after.CreatedAt, after.CreatedAt, after.ID).
		Order("created_at ASC, id ASC").
		Find(&messages).Error; err != nil {
		return nil, err
	}
	return messages, nil
}
