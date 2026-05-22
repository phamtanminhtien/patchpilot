package database

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

type AuthSessionRecord struct {
	ID          string    `gorm:"primaryKey;column:id"`
	SessionHash string    `gorm:"column:session_hash;not null;uniqueIndex"`
	CreatedAt   time.Time `gorm:"column:created_at;not null"`
	LastSeenAt  time.Time `gorm:"column:last_seen_at;not null;index"`
	ExpiresAt   time.Time `gorm:"column:expires_at;not null;index"`
}

func (AuthSessionRecord) TableName() string {
	return "auth_sessions"
}

func (s *Store) CreateAuthSession(ctx context.Context, session AuthSessionRecord) (AuthSessionRecord, error) {
	if session.ID == "" {
		id, err := newPrefixedID("auth_")
		if err != nil {
			return AuthSessionRecord{}, err
		}
		session.ID = id
	}
	now := time.Now().UTC()
	if session.CreatedAt.IsZero() {
		session.CreatedAt = now
	}
	if session.LastSeenAt.IsZero() {
		session.LastSeenAt = session.CreatedAt
	}
	if err := s.db.WithContext(ctx).Create(&session).Error; err != nil {
		return AuthSessionRecord{}, err
	}
	return session, nil
}

func (s *Store) GetAuthSessionByHash(ctx context.Context, sessionHash string, now time.Time) (AuthSessionRecord, error) {
	var session AuthSessionRecord
	if err := s.db.WithContext(ctx).First(&session, "session_hash = ? AND expires_at > ?", sessionHash, now.UTC()).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return AuthSessionRecord{}, ErrNotFound
		}
		return AuthSessionRecord{}, err
	}
	if err := s.db.WithContext(ctx).Model(&AuthSessionRecord{}).
		Where("id = ?", session.ID).
		Update("last_seen_at", now.UTC()).Error; err != nil {
		return AuthSessionRecord{}, err
	}
	session.LastSeenAt = now.UTC()
	return session, nil
}

func (s *Store) DeleteAuthSessionByHash(ctx context.Context, sessionHash string) error {
	return s.db.WithContext(ctx).Where("session_hash = ?", sessionHash).Delete(&AuthSessionRecord{}).Error
}
