package database

import "time"

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
