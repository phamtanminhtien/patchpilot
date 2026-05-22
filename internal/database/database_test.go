package database

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenCreatesSQLiteDatabaseAndEnablesForeignKeys(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "nested", "patchpilot.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()

	if err := store.Ping(context.Background()); err != nil {
		t.Fatalf("Ping returned error: %v", err)
	}

	var enabled int
	if err := store.db.Raw("PRAGMA foreign_keys").Scan(&enabled).Error; err != nil {
		t.Fatalf("read foreign_keys pragma: %v", err)
	}
	if enabled != 1 {
		t.Fatalf("expected foreign keys enabled, got %d", enabled)
	}

	if !store.db.Migrator().HasTable(&Metadata{}) {
		t.Fatal("expected app metadata table to be migrated")
	}
	if !store.db.Migrator().HasTable(&WorkspaceRecord{}) {
		t.Fatal("expected workspaces table to be migrated")
	}
	if !store.db.Migrator().HasTable(&FileIndexRecord{}) {
		t.Fatal("expected file_index table to be migrated")
	}
	if !store.db.Migrator().HasTable(&CommandRecord{}) {
		t.Fatal("expected commands table to be migrated")
	}
	if !store.db.Migrator().HasTable(&CommandOutputRecord{}) {
		t.Fatal("expected command_output table to be migrated")
	}
	if !store.db.Migrator().HasTable(&AgentTaskRecord{}) {
		t.Fatal("expected agent_tasks table to be migrated")
	}
	if !store.db.Migrator().HasTable(&AgentTaskEventRecord{}) {
		t.Fatal("expected agent_task_events table to be migrated")
	}
	if !store.db.Migrator().HasTable(&AgentToolCallRecord{}) {
		t.Fatal("expected agent_tool_calls table to be migrated")
	}
	if !store.db.Migrator().HasTable(&AuthSessionRecord{}) {
		t.Fatal("expected auth_sessions table to be migrated")
	}
	if !store.db.Migrator().HasTable(&SessionRecord{}) {
		t.Fatal("expected sessions table to be migrated")
	}
	if !store.db.Migrator().HasTable(&PortRecord{}) {
		t.Fatal("expected ports table to be migrated")
	}
	if !store.db.Migrator().HasTable(&GitSnapshotRecord{}) {
		t.Fatal("expected git_snapshots table to be migrated")
	}

	var version Metadata
	if err := store.db.First(&version, "key = ?", schemaVersionKey).Error; err != nil {
		t.Fatalf("expected schema version metadata: %v", err)
	}
	if version.Value != "1" {
		t.Fatalf("expected schema version 1, got %q", version.Value)
	}
}

func TestMigrateIsIdempotent(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "patchpilot.db")
	store, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}

	store, err = Open(dbPath)
	if err != nil {
		t.Fatalf("second Open returned error: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()

	var count int64
	if err := store.db.Model(&Metadata{}).Where("key = ?", schemaVersionKey).Count(&count).Error; err != nil {
		t.Fatalf("count schema version metadata: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected one schema version metadata row, got %d", count)
	}
}

func TestMigrateFailsOnInvalidSchemaVersion(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "patchpilot.db")
	store, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	if err := store.db.Model(&Metadata{}).
		Where("key = ?", schemaVersionKey).
		Update("value", "not-a-number").Error; err != nil {
		t.Fatalf("corrupt schema version metadata: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}

	store, err = Open(dbPath)
	if err == nil {
		_ = store.Close()
		t.Fatal("expected Open to fail on invalid schema version")
	}
	if !strings.Contains(err.Error(), "parse schema version") {
		t.Fatalf("expected parse schema version error, got %v", err)
	}
}
