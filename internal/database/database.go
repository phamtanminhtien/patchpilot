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

type SessionRecord struct {
	ID          string    `gorm:"primaryKey;column:id"`
	WorkspaceID string    `gorm:"column:workspace_id;not null;index"`
	Title       string    `gorm:"column:title;not null"`
	Mode        string    `gorm:"column:mode;not null"`
	CreatedAt   time.Time `gorm:"column:created_at;not null"`
	UpdatedAt   time.Time `gorm:"column:updated_at;not null;index"`
}

func (SessionRecord) TableName() string {
	return "sessions"
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

type AgentTaskRecord struct {
	ID              string     `gorm:"primaryKey;column:id"`
	WorkspaceID     string     `gorm:"column:workspace_id;not null;index"`
	SessionID       *string    `gorm:"column:session_id"`
	Prompt          string     `gorm:"column:prompt;not null"`
	Model           string     `gorm:"column:model;not null"`
	ReasoningEffort string     `gorm:"column:reasoning_effort;not null"`
	Status          string     `gorm:"column:status;not null;index"`
	Summary         string     `gorm:"column:summary;not null"`
	Error           *string    `gorm:"column:error"`
	StartedAt       *time.Time `gorm:"column:started_at"`
	FinishedAt      *time.Time `gorm:"column:finished_at"`
	CreatedAt       time.Time  `gorm:"column:created_at;not null;index"`
	UpdatedAt       time.Time  `gorm:"column:updated_at;not null;index"`
}

func (AgentTaskRecord) TableName() string {
	return "agent_tasks"
}

type AgentTaskEventRecord struct {
	ID          string    `gorm:"primaryKey;column:id"`
	WorkspaceID string    `gorm:"column:workspace_id;not null;index"`
	TaskID      string    `gorm:"column:task_id;not null;index"`
	Type        string    `gorm:"column:type;not null;index"`
	PayloadJSON string    `gorm:"column:payload_json;not null"`
	CreatedAt   time.Time `gorm:"column:created_at;not null;index"`
}

func (AgentTaskEventRecord) TableName() string {
	return "agent_task_events"
}

type AgentToolCallRecord struct {
	ID               string     `gorm:"primaryKey;column:id"`
	WorkspaceID      string     `gorm:"column:workspace_id;not null;index"`
	TaskID           string     `gorm:"column:task_id;not null;index"`
	BatchID          string     `gorm:"column:batch_id;not null;index"`
	Sequence         int        `gorm:"column:sequence;not null"`
	ProviderCallID   string     `gorm:"column:provider_call_id;not null"`
	Name             string     `gorm:"column:name;not null"`
	InputJSON        string     `gorm:"column:input_json;not null"`
	OutputJSON       string     `gorm:"column:output_json;not null"`
	Status           string     `gorm:"column:status;not null;index"`
	RequiresApproval bool       `gorm:"column:requires_approval;not null"`
	Decision         *string    `gorm:"column:decision"`
	StartedAt        *time.Time `gorm:"column:started_at"`
	FinishedAt       *time.Time `gorm:"column:finished_at"`
	CreatedAt        time.Time  `gorm:"column:created_at;not null;index"`
}

func (AgentToolCallRecord) TableName() string {
	return "agent_tool_calls"
}

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

type GitSnapshotRecord struct {
	ID          string    `gorm:"primaryKey;column:id"`
	WorkspaceID string    `gorm:"column:workspace_id;not null;index"`
	CommitSHA   *string   `gorm:"column:commit_sha;index"`
	StatusJSON  string    `gorm:"column:status_json;not null"`
	CreatedAt   time.Time `gorm:"column:created_at;not null;index"`
}

func (GitSnapshotRecord) TableName() string {
	return "git_snapshots"
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
	return s.db.AutoMigrate(&Metadata{}, &WorkspaceRecord{}, &FileIndexRecord{}, &AuthSessionRecord{}, &SessionRecord{}, &AgentTaskRecord{}, &AgentTaskEventRecord{}, &AgentToolCallRecord{}, &CommandRecord{}, &CommandOutputRecord{}, &PortRecord{}, &GitSnapshotRecord{})
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

func (s *Store) DeleteWorkspaceMetadata(ctx context.Context, workspaceID string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("workspace_id = ?", workspaceID).Delete(&FileIndexRecord{}).Error; err != nil {
			return err
		}
		if err := tx.Where("workspace_id = ?", workspaceID).Delete(&AgentTaskEventRecord{}).Error; err != nil {
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
		if err := tx.Where("workspace_id = ?", workspaceID).Delete(&SessionRecord{}).Error; err != nil {
			return err
		}
		if err := tx.Where("workspace_id = ?", workspaceID).Delete(&AgentTaskRecord{}).Error; err != nil {
			return err
		}
		return tx.Where("id = ?", workspaceID).Delete(&WorkspaceRecord{}).Error
	})
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

func (s *Store) UpsertSession(ctx context.Context, session SessionRecord) (SessionRecord, error) {
	if session.ID == "" {
		id, err := newPrefixedID("sess_")
		if err != nil {
			return SessionRecord{}, err
		}
		session.ID = id
	}
	now := time.Now().UTC()
	if session.CreatedAt.IsZero() {
		session.CreatedAt = now
	}
	session.UpdatedAt = now
	var existing SessionRecord
	err := s.db.WithContext(ctx).First(&existing, "workspace_id = ?", session.WorkspaceID).Error
	if err == nil {
		if err := s.db.WithContext(ctx).Model(&SessionRecord{}).
			Where("id = ?", existing.ID).
			Updates(map[string]any{"title": session.Title, "mode": session.Mode, "updated_at": session.UpdatedAt}).Error; err != nil {
			return SessionRecord{}, err
		}
		return s.GetSession(ctx, existing.ID)
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return SessionRecord{}, err
	}
	if err := s.db.WithContext(ctx).Create(&session).Error; err != nil {
		return SessionRecord{}, err
	}
	return session, nil
}

func (s *Store) GetSession(ctx context.Context, sessionID string) (SessionRecord, error) {
	var session SessionRecord
	if err := s.db.WithContext(ctx).First(&session, "id = ?", sessionID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return SessionRecord{}, ErrNotFound
		}
		return SessionRecord{}, err
	}
	return session, nil
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

func (s *Store) CreateAgentTask(ctx context.Context, task AgentTaskRecord) (AgentTaskRecord, error) {
	if task.ID == "" {
		id, err := newPrefixedID("task_")
		if err != nil {
			return AgentTaskRecord{}, err
		}
		task.ID = id
	}
	now := time.Now().UTC()
	if task.CreatedAt.IsZero() {
		task.CreatedAt = now
	}
	if task.UpdatedAt.IsZero() {
		task.UpdatedAt = task.CreatedAt
	}
	if err := s.db.WithContext(ctx).Create(&task).Error; err != nil {
		return AgentTaskRecord{}, err
	}
	return task, nil
}

func (s *Store) GetAgentTask(ctx context.Context, workspaceID, taskID string) (AgentTaskRecord, error) {
	var task AgentTaskRecord
	if err := s.db.WithContext(ctx).First(&task, "workspace_id = ? AND id = ?", workspaceID, taskID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return AgentTaskRecord{}, ErrNotFound
		}
		return AgentTaskRecord{}, err
	}
	return task, nil
}

func (s *Store) ListAgentTasks(ctx context.Context, workspaceID string) ([]AgentTaskRecord, error) {
	var tasks []AgentTaskRecord
	if err := s.db.WithContext(ctx).
		Where("workspace_id = ?", workspaceID).
		Order("created_at DESC, id DESC").
		Find(&tasks).Error; err != nil {
		return nil, err
	}
	return tasks, nil
}

func (s *Store) UpdateAgentTask(ctx context.Context, workspaceID, taskID string, updates map[string]any) (AgentTaskRecord, error) {
	updates["updated_at"] = time.Now().UTC()
	if err := s.db.WithContext(ctx).Model(&AgentTaskRecord{}).
		Where("workspace_id = ? AND id = ?", workspaceID, taskID).
		Updates(updates).Error; err != nil {
		return AgentTaskRecord{}, err
	}
	return s.GetAgentTask(ctx, workspaceID, taskID)
}

func (s *Store) CreateAgentTaskEvent(ctx context.Context, event AgentTaskEventRecord) (AgentTaskEventRecord, error) {
	if event.ID == "" {
		id, err := newPrefixedID("evt_")
		if err != nil {
			return AgentTaskEventRecord{}, err
		}
		event.ID = id
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}
	if err := s.db.WithContext(ctx).Create(&event).Error; err != nil {
		return AgentTaskEventRecord{}, err
	}
	return event, nil
}

func (s *Store) ListAgentTaskEvents(ctx context.Context, workspaceID, taskID string) ([]AgentTaskEventRecord, error) {
	var events []AgentTaskEventRecord
	query := s.db.WithContext(ctx).Where("workspace_id = ?", workspaceID)
	if taskID != "" {
		query = query.Where("task_id = ?", taskID)
	}
	if err := query.Order("created_at ASC, id ASC").Find(&events).Error; err != nil {
		return nil, err
	}
	return events, nil
}

func (s *Store) CreateAgentToolCall(ctx context.Context, call AgentToolCallRecord) (AgentToolCallRecord, error) {
	if call.ID == "" {
		id, err := newPrefixedID("evt_")
		if err != nil {
			return AgentToolCallRecord{}, err
		}
		call.ID = id
	}
	if call.CreatedAt.IsZero() {
		call.CreatedAt = time.Now().UTC()
	}
	if err := s.db.WithContext(ctx).Create(&call).Error; err != nil {
		return AgentToolCallRecord{}, err
	}
	return call, nil
}

func (s *Store) FinishAgentToolCall(ctx context.Context, workspaceID, taskID, callID, status, outputJSON string, finishedAt time.Time) (AgentToolCallRecord, error) {
	if err := s.db.WithContext(ctx).Model(&AgentToolCallRecord{}).
		Where("workspace_id = ? AND task_id = ? AND id = ?", workspaceID, taskID, callID).
		Updates(map[string]any{"status": status, "output_json": outputJSON, "finished_at": finishedAt}).Error; err != nil {
		return AgentToolCallRecord{}, err
	}
	var call AgentToolCallRecord
	if err := s.db.WithContext(ctx).First(&call, "workspace_id = ? AND task_id = ? AND id = ?", workspaceID, taskID, callID).Error; err != nil {
		return AgentToolCallRecord{}, err
	}
	return call, nil
}

func (s *Store) UpdateAgentToolCall(ctx context.Context, workspaceID, taskID, callID string, updates map[string]any) (AgentToolCallRecord, error) {
	if err := s.db.WithContext(ctx).Model(&AgentToolCallRecord{}).
		Where("workspace_id = ? AND task_id = ? AND id = ?", workspaceID, taskID, callID).
		Updates(updates).Error; err != nil {
		return AgentToolCallRecord{}, err
	}
	var call AgentToolCallRecord
	if err := s.db.WithContext(ctx).First(&call, "workspace_id = ? AND task_id = ? AND id = ?", workspaceID, taskID, callID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return AgentToolCallRecord{}, ErrNotFound
		}
		return AgentToolCallRecord{}, err
	}
	return call, nil
}

func (s *Store) GetAgentToolCall(ctx context.Context, workspaceID, taskID, callID string) (AgentToolCallRecord, error) {
	var call AgentToolCallRecord
	if err := s.db.WithContext(ctx).First(&call, "workspace_id = ? AND task_id = ? AND id = ?", workspaceID, taskID, callID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return AgentToolCallRecord{}, ErrNotFound
		}
		return AgentToolCallRecord{}, err
	}
	return call, nil
}

func (s *Store) ListAgentToolCalls(ctx context.Context, workspaceID, taskID string) ([]AgentToolCallRecord, error) {
	var calls []AgentToolCallRecord
	if err := s.db.WithContext(ctx).
		Where("workspace_id = ? AND task_id = ?", workspaceID, taskID).
		Order("created_at ASC, batch_id ASC, sequence ASC, id ASC").
		Find(&calls).Error; err != nil {
		return nil, err
	}
	return calls, nil
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

func newPrefixedID(prefix string) (string, error) {
	var random [8]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", err
	}
	return prefix + hex.EncodeToString(random[:]), nil
}
