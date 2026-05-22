package database

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

type SessionRecord struct {
	ID          string    `gorm:"primaryKey;column:id"`
	WorkspaceID string    `gorm:"column:workspace_id;not null;index"`
	Title       string    `gorm:"column:title;not null"`
	Mode        string    `gorm:"column:mode;not null"`
	CreatedAt   time.Time `gorm:"column:created_at;not null"`
	UpdatedAt   time.Time `gorm:"column:updated_at;not null;index"`
}

func (SessionRecord) TableName() string {
	return "sessions"
}

func (s *Store) UpsertSession(ctx context.Context, session SessionRecord) (SessionRecord, error) {
	if session.ID == "" {
		id, err := newPrefixedID("sess_")
		if err != nil {
			return SessionRecord{}, err
		}
		session.ID = id
	}
	now := time.Now().UTC()
	if session.CreatedAt.IsZero() {
		session.CreatedAt = now
	}
	session.UpdatedAt = now
	var existing SessionRecord
	err := s.db.WithContext(ctx).First(&existing, "workspace_id = ?", session.WorkspaceID).Error
	if err == nil {
		if err := s.db.WithContext(ctx).Model(&SessionRecord{}).
			Where("id = ?", existing.ID).
			Updates(map[string]any{"title": session.Title, "mode": session.Mode, "updated_at": session.UpdatedAt}).Error; err != nil {
			return SessionRecord{}, err
		}
		return s.GetSession(ctx, existing.ID)
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return SessionRecord{}, err
	}
	if err := s.db.WithContext(ctx).Create(&session).Error; err != nil {
		return SessionRecord{}, err
	}
	return session, nil
}

func (s *Store) GetSession(ctx context.Context, sessionID string) (SessionRecord, error) {
	var session SessionRecord
	if err := s.db.WithContext(ctx).First(&session, "id = ?", sessionID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return SessionRecord{}, ErrNotFound
		}
		return SessionRecord{}, err
	}
	return session, nil
}
