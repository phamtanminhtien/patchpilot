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
	if !store.db.Migrator().HasTable(&ConversationRecord{}) {
		t.Fatal("expected conversations table to be migrated")
	}
	if !store.db.Migrator().HasColumn(&ConversationRecord{}, "has_running_run") {
		t.Fatal("expected has_running_run column on conversations table")
	}
	if !store.db.Migrator().HasTable(&MessageRecord{}) {
		t.Fatal("expected messages table to be migrated")
	}
	if !store.db.Migrator().HasTable(&AgentRunRecord{}) {
		t.Fatal("expected agent_runs table to be migrated")
	}
	if !store.db.Migrator().HasTable(&AgentRunEventRecord{}) {
		t.Fatal("expected agent_run_events table to be migrated")
	}
	if !store.db.Migrator().HasTable(&AgentToolCallRecord{}) {
		t.Fatal("expected agent_tool_calls table to be migrated")
	}
	if !store.db.Migrator().HasTable(&AuthSessionRecord{}) {
		t.Fatal("expected auth_sessions table to be migrated")
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
	if version.Value != "4" {
		t.Fatalf("expected schema version 4, got %q", version.Value)
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

func TestConversationContextSummaryAndListMessagesAfter(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "patchpilot.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()

	conversation, err := store.CreateConversation(context.Background(), ConversationRecord{
		WorkspaceID: "ws_1",
		Title:       "Context",
	})
	if err != nil {
		t.Fatalf("CreateConversation returned error: %v", err)
	}
	first, err := store.CreateMessage(context.Background(), MessageRecord{
		WorkspaceID:    "ws_1",
		ConversationID: conversation.ID,
		Role:           "user",
		Content:        "first",
	})
	if err != nil {
		t.Fatalf("CreateMessage first returned error: %v", err)
	}
	second, err := store.CreateMessage(context.Background(), MessageRecord{
		WorkspaceID:    "ws_1",
		ConversationID: conversation.ID,
		Role:           "assistant",
		Content:        "second",
	})
	if err != nil {
		t.Fatalf("CreateMessage second returned error: %v", err)
	}

	updated, err := store.UpdateConversationContextSummary(context.Background(), "ws_1", conversation.ID, "summary", first.ID, first.CreatedAt)
	if err != nil {
		t.Fatalf("UpdateConversationContextSummary returned error: %v", err)
	}
	if updated.ContextSummary != "summary" || updated.ContextSummaryThroughMessageID == nil || *updated.ContextSummaryThroughMessageID != first.ID {
		t.Fatalf("unexpected summary fields: %+v", updated)
	}

	messages, err := store.ListMessagesAfter(context.Background(), "ws_1", conversation.ID, first.ID)
	if err != nil {
		t.Fatalf("ListMessagesAfter returned error: %v", err)
	}
	if len(messages) != 1 || messages[0].ID != second.ID {
		t.Fatalf("expected only second message, got %+v", messages)
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
