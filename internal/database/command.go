package database

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

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
