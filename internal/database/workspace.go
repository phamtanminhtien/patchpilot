package database

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

type WorkspaceRecord struct {
	ID            string    `gorm:"primaryKey;column:id"`
	Name          string    `gorm:"column:name;not null"`
	RootPath      string    `gorm:"column:root_path;not null;uniqueIndex"`
	GitRemote     *string   `gorm:"column:git_remote"`
	DefaultBranch *string   `gorm:"column:default_branch"`
	Status        string    `gorm:"column:status;not null"`
	CreatedAt     time.Time `gorm:"column:created_at;not null"`
	UpdatedAt     time.Time `gorm:"column:updated_at;not null;index"`
}

func (WorkspaceRecord) TableName() string {
	return "workspaces"
}

func (s *Store) CreateWorkspace(ctx context.Context, workspace WorkspaceRecord) (WorkspaceRecord, error) {
	if err := s.db.WithContext(ctx).Create(&workspace).Error; err != nil {
		return WorkspaceRecord{}, err
	}
	return workspace, nil
}

func (s *Store) GetWorkspace(ctx context.Context, id string) (WorkspaceRecord, error) {
	var workspace WorkspaceRecord
	if err := s.db.WithContext(ctx).First(&workspace, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return WorkspaceRecord{}, ErrNotFound
		}
		return WorkspaceRecord{}, err
	}
	return workspace, nil
}

func (s *Store) FindWorkspaceByRoot(ctx context.Context, rootPath string) (WorkspaceRecord, error) {
	var workspace WorkspaceRecord
	if err := s.db.WithContext(ctx).First(&workspace, "root_path = ?", rootPath).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return WorkspaceRecord{}, ErrNotFound
		}
		return WorkspaceRecord{}, err
	}
	return workspace, nil
}

func (s *Store) ListWorkspaces(ctx context.Context) ([]WorkspaceRecord, error) {
	var workspaces []WorkspaceRecord
	if err := s.db.WithContext(ctx).Order("updated_at DESC, id DESC").Find(&workspaces).Error; err != nil {
		return nil, err
	}
	return workspaces, nil
}

func (s *Store) TouchWorkspace(ctx context.Context, id string, updatedAt time.Time) (WorkspaceRecord, error) {
	if err := s.db.WithContext(ctx).Model(&WorkspaceRecord{}).
		Where("id = ?", id).
		Update("updated_at", updatedAt).Error; err != nil {
		return WorkspaceRecord{}, err
	}
	return s.GetWorkspace(ctx, id)
}

func (s *Store) DeleteWorkspaceMetadata(ctx context.Context, workspaceID string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("workspace_id = ?", workspaceID).Delete(&FileIndexRecord{}).Error; err != nil {
			return err
		}
		if err := tx.Where("workspace_id = ?", workspaceID).Delete(&AgentRunEventRecord{}).Error; err != nil {
			return err
		}
		if err := tx.Where("workspace_id = ?", workspaceID).Delete(&AgentToolCallRecord{}).Error; err != nil {
			return err
		}
		var commands []CommandRecord
		if err := tx.Where("workspace_id = ?", workspaceID).Find(&commands).Error; err != nil {
			return err
		}
		commandIDs := make([]string, 0, len(commands))
		for _, command := range commands {
			commandIDs = append(commandIDs, command.ID)
		}
		if len(commandIDs) > 0 {
			if err := tx.Where("command_id IN ?", commandIDs).Delete(&CommandOutputRecord{}).Error; err != nil {
				return err
			}
		}
		if err := tx.Where("workspace_id = ?", workspaceID).Delete(&CommandRecord{}).Error; err != nil {
			return err
		}
		if err := tx.Where("workspace_id = ?", workspaceID).Delete(&PortRecord{}).Error; err != nil {
			return err
		}
		if err := tx.Where("workspace_id = ?", workspaceID).Delete(&GitSnapshotRecord{}).Error; err != nil {
			return err
		}
		if err := tx.Where("workspace_id = ?", workspaceID).Delete(&MessageRecord{}).Error; err != nil {
			return err
		}
		if err := tx.Where("workspace_id = ?", workspaceID).Delete(&AgentRunRecord{}).Error; err != nil {
			return err
		}
		if err := tx.Where("workspace_id = ?", workspaceID).Delete(&ConversationRecord{}).Error; err != nil {
			return err
		}
		return tx.Where("id = ?", workspaceID).Delete(&WorkspaceRecord{}).Error
	})
}
