package database

import (
	"context"
	"time"

	"gorm.io/gorm"
)

type FileIndexRecord struct {
	WorkspaceID string    `gorm:"primaryKey;column:workspace_id"`
	Path        string    `gorm:"primaryKey;column:path"`
	Size        int64     `gorm:"column:size;not null"`
	ModifiedAt  time.Time `gorm:"column:modified_at;not null"`
	IndexedAt   time.Time `gorm:"column:indexed_at;not null;index"`
}

func (FileIndexRecord) TableName() string {
	return "file_index"
}

func (s *Store) ReplaceFileIndex(ctx context.Context, workspaceID string, entries []FileIndexRecord) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("workspace_id = ?", workspaceID).Delete(&FileIndexRecord{}).Error; err != nil {
			return err
		}
		if len(entries) == 0 {
			return nil
		}
		return tx.CreateInBatches(entries, 500).Error
	})
}

func (s *Store) ListFileIndex(ctx context.Context, workspaceID string) ([]FileIndexRecord, error) {
	var entries []FileIndexRecord
	if err := s.db.WithContext(ctx).
		Where("workspace_id = ?", workspaceID).
		Order("path ASC").
		Find(&entries).Error; err != nil {
		return nil, err
	}
	return entries, nil
}
