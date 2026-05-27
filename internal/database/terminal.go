package database

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

type TerminalSessionRecord struct {
	ID          string     `gorm:"primaryKey;column:id"`
	WorkspaceID string     `gorm:"column:workspace_id;not null;index"`
	Title       string     `gorm:"column:title;not null"`
	Cwd         string     `gorm:"column:cwd;not null"`
	Status      string     `gorm:"column:status;not null;index"`
	PID         *int       `gorm:"column:pid"`
	Rows        int        `gorm:"column:rows;not null"`
	Cols        int        `gorm:"column:cols;not null"`
	ExitCode    *int       `gorm:"column:exit_code"`
	CreatedAt   time.Time  `gorm:"column:created_at;not null"`
	UpdatedAt   time.Time  `gorm:"column:updated_at;not null;index"`
	ClosedAt    *time.Time `gorm:"column:closed_at"`
}

func (TerminalSessionRecord) TableName() string {
	return "terminal_sessions"
}

func (s *Store) CreateTerminalSession(ctx context.Context, session TerminalSessionRecord) (TerminalSessionRecord, error) {
	now := time.Now().UTC()
	if session.ID == "" {
		id, err := newPrefixedID("term_")
		if err != nil {
			return TerminalSessionRecord{}, err
		}
		session.ID = id
	}
	if session.CreatedAt.IsZero() {
		session.CreatedAt = now
	}
	if session.UpdatedAt.IsZero() {
		session.UpdatedAt = now
	}
	if err := s.db.WithContext(ctx).Create(&session).Error; err != nil {
		return TerminalSessionRecord{}, err
	}
	return session, nil
}

func (s *Store) ListTerminalSessions(ctx context.Context, workspaceID string) ([]TerminalSessionRecord, error) {
	var sessions []TerminalSessionRecord
	if err := s.db.WithContext(ctx).
		Where("workspace_id = ?", workspaceID).
		Order("updated_at DESC, id DESC").
		Find(&sessions).Error; err != nil {
		return nil, err
	}
	return sessions, nil
}

func (s *Store) ListOpenTerminalSessions(ctx context.Context) ([]TerminalSessionRecord, error) {
	var sessions []TerminalSessionRecord
	if err := s.db.WithContext(ctx).
		Where("status = ?", "open").
		Order("created_at ASC, id ASC").
		Find(&sessions).Error; err != nil {
		return nil, err
	}
	return sessions, nil
}

func (s *Store) GetTerminalSession(ctx context.Context, workspaceID, sessionID string) (TerminalSessionRecord, error) {
	var session TerminalSessionRecord
	if err := s.db.WithContext(ctx).First(&session, "workspace_id = ? AND id = ?", workspaceID, sessionID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return TerminalSessionRecord{}, ErrNotFound
		}
		return TerminalSessionRecord{}, err
	}
	return session, nil
}

func (s *Store) UpdateTerminalSession(ctx context.Context, workspaceID, sessionID string, updates map[string]any) (TerminalSessionRecord, error) {
	updates["updated_at"] = time.Now().UTC()
	if err := s.db.WithContext(ctx).Model(&TerminalSessionRecord{}).
		Where("workspace_id = ? AND id = ?", workspaceID, sessionID).
		Updates(updates).Error; err != nil {
		return TerminalSessionRecord{}, err
	}
	return s.GetTerminalSession(ctx, workspaceID, sessionID)
}

func (s *Store) CloseTerminalSession(ctx context.Context, workspaceID, sessionID, status string, exitCode *int, closedAt time.Time) (TerminalSessionRecord, error) {
	if status == "" {
		status = "closed"
	}
	updates := map[string]any{
		"status":     status,
		"exit_code":  exitCode,
		"closed_at":  closedAt.UTC(),
		"updated_at": closedAt.UTC(),
	}
	if err := s.db.WithContext(ctx).Model(&TerminalSessionRecord{}).
		Where("workspace_id = ? AND id = ?", workspaceID, sessionID).
		Updates(updates).Error; err != nil {
		return TerminalSessionRecord{}, err
	}
	return s.GetTerminalSession(ctx, workspaceID, sessionID)
}
