package database

import (
	"context"
	"crypto/rand"
	"encoding/hex"
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

type CommandRecord struct {
	ID          string     `gorm:"primaryKey;column:id"`
	WorkspaceID string     `gorm:"column:workspace_id;not null;index"`
	TaskID      *string    `gorm:"column:task_id"`
	Command     string     `gorm:"column:command;not null"`
	Cwd         string     `gorm:"column:cwd;not null"`
	Status      string     `gorm:"column:status;not null;index"`
	ExitCode    *int       `gorm:"column:exit_code"`
	StartedAt   *time.Time `gorm:"column:started_at"`
	FinishedAt  *time.Time `gorm:"column:finished_at"`
	CreatedAt   time.Time  `gorm:"column:created_at;not null;index"`
}

func (CommandRecord) TableName() string {
	return "commands"
}

type CommandOutputRecord struct {
	ID        string    `gorm:"primaryKey;column:id"`
	CommandID string    `gorm:"column:command_id;not null;index"`
	Stream    string    `gorm:"column:stream;not null"`
	Chunk     string    `gorm:"column:chunk;not null"`
	CreatedAt time.Time `gorm:"column:created_at;not null;index"`
}

func (CommandOutputRecord) TableName() string {
	return "command_output"
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
	return s.db.AutoMigrate(&Metadata{}, &WorkspaceRecord{}, &FileIndexRecord{}, &CommandRecord{}, &CommandOutputRecord{})
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

func (s *Store) CreateCommand(ctx context.Context, command CommandRecord) (CommandRecord, error) {
	if command.ID == "" {
		id, err := newPrefixedID("cmd_")
		if err != nil {
			return CommandRecord{}, err
		}
		command.ID = id
	}
	if command.CreatedAt.IsZero() {
		command.CreatedAt = time.Now().UTC()
	}
	if err := s.db.WithContext(ctx).Create(&command).Error; err != nil {
		return CommandRecord{}, err
	}
	return command, nil
}

func (s *Store) GetCommand(ctx context.Context, workspaceID, commandID string) (CommandRecord, error) {
	var command CommandRecord
	if err := s.db.WithContext(ctx).First(&command, "workspace_id = ? AND id = ?", workspaceID, commandID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return CommandRecord{}, ErrNotFound
		}
		return CommandRecord{}, err
	}
	return command, nil
}

func (s *Store) ListCommands(ctx context.Context, workspaceID string) ([]CommandRecord, error) {
	var commands []CommandRecord
	if err := s.db.WithContext(ctx).
		Where("workspace_id = ?", workspaceID).
		Order("created_at DESC, id DESC").
		Find(&commands).Error; err != nil {
		return nil, err
	}
	return commands, nil
}

func (s *Store) MarkCommandRunning(ctx context.Context, workspaceID, commandID string, startedAt time.Time) (CommandRecord, error) {
	if err := s.db.WithContext(ctx).Model(&CommandRecord{}).
		Where("workspace_id = ? AND id = ?", workspaceID, commandID).
		Updates(map[string]any{"status": "running", "started_at": startedAt}).Error; err != nil {
		return CommandRecord{}, err
	}
	return s.GetCommand(ctx, workspaceID, commandID)
}

func (s *Store) FinishCommand(ctx context.Context, workspaceID, commandID, status string, exitCode *int, finishedAt time.Time) (CommandRecord, error) {
	if err := s.db.WithContext(ctx).Model(&CommandRecord{}).
		Where("workspace_id = ? AND id = ?", workspaceID, commandID).
		Updates(map[string]any{"status": status, "exit_code": exitCode, "finished_at": finishedAt}).Error; err != nil {
		return CommandRecord{}, err
	}
	return s.GetCommand(ctx, workspaceID, commandID)
}

func (s *Store) AppendCommandOutput(ctx context.Context, output CommandOutputRecord, maxBytes int) (CommandOutputRecord, error) {
	if output.ID == "" {
		id, err := newPrefixedID("out_")
		if err != nil {
			return CommandOutputRecord{}, err
		}
		output.ID = id
	}
	if output.CreatedAt.IsZero() {
		output.CreatedAt = time.Now().UTC()
	}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&output).Error; err != nil {
			return err
		}
		if maxBytes <= 0 {
			return nil
		}
		var records []CommandOutputRecord
		if err := tx.Where("command_id = ?", output.CommandID).
			Order("created_at ASC, id ASC").
			Find(&records).Error; err != nil {
			return err
		}
		total := 0
		for _, record := range records {
			total += len([]byte(record.Chunk))
		}
		var deleteIDs []string
		for _, record := range records {
			if total <= maxBytes {
				break
			}
			deleteIDs = append(deleteIDs, record.ID)
			total -= len([]byte(record.Chunk))
		}
		if len(deleteIDs) == 0 {
			return nil
		}
		return tx.Where("id IN ?", deleteIDs).Delete(&CommandOutputRecord{}).Error
	})
	if err != nil {
		return CommandOutputRecord{}, err
	}
	return output, nil
}

func (s *Store) ListCommandOutput(ctx context.Context, commandID string) ([]CommandOutputRecord, error) {
	var output []CommandOutputRecord
	if err := s.db.WithContext(ctx).
		Where("command_id = ?", commandID).
		Order("created_at ASC, id ASC").
		Find(&output).Error; err != nil {
		return nil, err
	}
	return output, nil
}

func newPrefixedID(prefix string) (string, error) {
	var random [8]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", err
	}
	return prefix + hex.EncodeToString(random[:]), nil
}
