package database

import (
	"embed"
	"errors"
	"fmt"
	"strconv"
	"time"

	"gorm.io/gorm"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

type migration struct {
	version int
	name    string
	path    string
}

const schemaVersionKey = "schema_version"

var migrations = []migration{
	{version: 1, name: "create_initial_schema", path: "migrations/001_create_initial_schema.sql"},
	{version: 2, name: "align_conversation_run_model", path: "migrations/002_align_conversation_run_model.sql"},
	{version: 3, name: "conversation_context_summary", path: "migrations/003_conversation_context_summary.sql"},
	{version: 4, name: "conversation_active_run_flag", path: "migrations/004_conversation_active_run_flag.sql"},
	{version: 5, name: "agent_tool_call_sources", path: "migrations/005_agent_tool_call_sources.sql"},
}

func (s *Store) Migrate() error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		currentVersion, err := migrationVersion(tx)
		if err != nil {
			return err
		}
		for _, m := range migrations {
			if m.version <= currentVersion {
				continue
			}
			sql, err := migrationFiles.ReadFile(m.path)
			if err != nil {
				return fmt.Errorf("read migration %d %s: %w", m.version, m.name, err)
			}
			if err := tx.Exec(string(sql)).Error; err != nil {
				return fmt.Errorf("apply migration %d %s: %w", m.version, m.name, err)
			}
			if err := setMigrationVersion(tx, m.version); err != nil {
				return fmt.Errorf("record migration %d %s: %w", m.version, m.name, err)
			}
		}
		return nil
	})
}

func migrationVersion(tx *gorm.DB) (int, error) {
	var count int64
	if err := tx.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?", "app_metadata").Scan(&count).Error; err != nil {
		return 0, err
	}
	if count == 0 {
		return 0, nil
	}

	var metadata Metadata
	if err := tx.First(&metadata, "key = ?", schemaVersionKey).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, nil
		}
		return 0, err
	}
	version, err := strconv.Atoi(metadata.Value)
	if err != nil {
		return 0, fmt.Errorf("parse schema version %q: %w", metadata.Value, err)
	}
	return version, nil
}

func setMigrationVersion(tx *gorm.DB, version int) error {
	now := time.Now().UTC()
	value := strconv.Itoa(version)
	result := tx.Model(&Metadata{}).
		Where("key = ?", schemaVersionKey).
		Updates(map[string]any{"value": value, "updated_at": now})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected > 0 {
		return nil
	}
	return tx.Create(&Metadata{
		Key:       schemaVersionKey,
		Value:     value,
		CreatedAt: now,
		UpdatedAt: now,
	}).Error
}
