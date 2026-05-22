package database

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

type PortRecord struct {
	ID          string     `gorm:"primaryKey;column:id"`
	WorkspaceID string     `gorm:"column:workspace_id;not null;index"`
	ProcessID   *string    `gorm:"column:process_id;index"`
	Port        int        `gorm:"column:port;not null;index"`
	Status      string     `gorm:"column:status;not null;index"`
	ExposedPath *string    `gorm:"column:exposed_path"`
	CreatedAt   time.Time  `gorm:"column:created_at;not null"`
	UpdatedAt   time.Time  `gorm:"column:updated_at;not null;index"`
	ClosedAt    *time.Time `gorm:"column:closed_at"`
}

func (PortRecord) TableName() string {
	return "ports"
}

func (s *Store) UpsertDetectedPort(ctx context.Context, port PortRecord) (PortRecord, bool, error) {
	now := time.Now().UTC()
	if port.CreatedAt.IsZero() {
		port.CreatedAt = now
	}
	port.UpdatedAt = now
	var existing PortRecord
	err := s.db.WithContext(ctx).First(&existing, "workspace_id = ? AND port = ?", port.WorkspaceID, port.Port).Error
	if err == nil {
		status := port.Status
		if port.Status == "detected" && (existing.Status == "exposed" || existing.ExposedPath != nil) {
			status = existing.Status
			if status == "closed" {
				status = "exposed"
			}
		}
		updates := map[string]any{
			"status":     status,
			"updated_at": port.UpdatedAt,
			"closed_at":  nil,
		}
		if port.ProcessID != nil {
			updates["process_id"] = port.ProcessID
		}
		if err := s.db.WithContext(ctx).Model(&PortRecord{}).
			Where("id = ?", existing.ID).
			Updates(updates).Error; err != nil {
			return PortRecord{}, false, err
		}
		updated, err := s.GetPort(ctx, port.WorkspaceID, port.Port)
		return updated, false, err
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return PortRecord{}, false, err
	}
	if port.ID == "" {
		id, err := newPrefixedID("port_")
		if err != nil {
			return PortRecord{}, false, err
		}
		port.ID = id
	}
	if err := s.db.WithContext(ctx).Create(&port).Error; err != nil {
		return PortRecord{}, false, err
	}
	return port, true, nil
}

func (s *Store) ListPorts(ctx context.Context, workspaceID string) ([]PortRecord, error) {
	var ports []PortRecord
	if err := s.db.WithContext(ctx).
		Where("workspace_id = ?", workspaceID).
		Order("port ASC").
		Find(&ports).Error; err != nil {
		return nil, err
	}
	return ports, nil
}

func (s *Store) GetPort(ctx context.Context, workspaceID string, port int) (PortRecord, error) {
	var record PortRecord
	if err := s.db.WithContext(ctx).First(&record, "workspace_id = ? AND port = ?", workspaceID, port).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return PortRecord{}, ErrNotFound
		}
		return PortRecord{}, err
	}
	return record, nil
}

func (s *Store) ExposePort(ctx context.Context, workspaceID string, port int, exposedPath string) (PortRecord, error) {
	if err := s.db.WithContext(ctx).Model(&PortRecord{}).
		Where("workspace_id = ? AND port = ?", workspaceID, port).
		Updates(map[string]any{"status": "exposed", "exposed_path": exposedPath, "updated_at": time.Now().UTC()}).Error; err != nil {
		return PortRecord{}, err
	}
	return s.GetPort(ctx, workspaceID, port)
}

func (s *Store) MarkPortClosed(ctx context.Context, workspaceID string, port int, closedAt time.Time) (PortRecord, error) {
	if err := s.db.WithContext(ctx).Model(&PortRecord{}).
		Where("workspace_id = ? AND port = ?", workspaceID, port).
		Updates(map[string]any{"status": "closed", "closed_at": closedAt.UTC(), "updated_at": closedAt.UTC()}).Error; err != nil {
		return PortRecord{}, err
	}
	return s.GetPort(ctx, workspaceID, port)
}
