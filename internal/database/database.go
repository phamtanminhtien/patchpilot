package database

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

var ErrNotFound = errors.New("record not found")

type Store struct {
	db *gorm.DB
}

type Metadata struct {
	Key       string    `gorm:"primaryKey;column:key"`
	Value     string    `gorm:"column:value;not null"`
	CreatedAt time.Time `gorm:"column:created_at;not null"`
	UpdatedAt time.Time `gorm:"column:updated_at;not null"`
}

func (Metadata) TableName() string {
	return "app_metadata"
}

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

func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}

	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	store := &Store{db: db}
	if err := store.enableForeignKeys(); err != nil {
		_ = store.Close()
		return nil, err
	}
	if err := store.Migrate(); err != nil {
		_ = store.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

func (s *Store) Ping(ctx context.Context) error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.PingContext(ctx)
}

func (s *Store) Migrate() error {
	return s.db.AutoMigrate(&Metadata{}, &WorkspaceRecord{}, &FileIndexRecord{})
}

func (s *Store) enableForeignKeys() error {
	return s.db.Exec("PRAGMA foreign_keys = ON").Error
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
