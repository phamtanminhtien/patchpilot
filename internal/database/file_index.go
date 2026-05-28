package database

import (
	"context"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type FileIndexRecord struct {
	WorkspaceID string    `gorm:"primaryKey;column:workspace_id"`
	Path        string    `gorm:"primaryKey;column:path"`
	Name        string    `gorm:"column:name;not null"`
	Dir         string    `gorm:"column:dir;not null"`
	Extension   string    `gorm:"column:extension;not null"`
	Kind        string    `gorm:"column:kind;not null"`
	IndexStatus string    `gorm:"column:index_status;not null"`
	PathLower   string    `gorm:"column:path_lower;not null"`
	NameLower   string    `gorm:"column:name_lower;not null"`
	Depth       int       `gorm:"column:depth;not null"`
	Size        int64     `gorm:"column:size;not null"`
	ModifiedAt  time.Time `gorm:"column:modified_at;not null"`
	IndexedAt   time.Time `gorm:"column:indexed_at;not null;index"`
}

func (FileIndexRecord) TableName() string {
	return "file_index"
}

type WorkspaceIndexStateRecord struct {
	WorkspaceID    string     `gorm:"primaryKey;column:workspace_id"`
	Status         string     `gorm:"column:status;not null"`
	LastIndexedAt  *time.Time `gorm:"column:last_indexed_at"`
	LastFullScanAt *time.Time `gorm:"column:last_full_scan_at"`
	FileCount      int        `gorm:"column:file_count;not null"`
	SkippedCount   int        `gorm:"column:skipped_count;not null"`
	Truncated      bool       `gorm:"column:truncated;not null"`
	Error          *string    `gorm:"column:error"`
	UpdatedAt      time.Time  `gorm:"column:updated_at;not null"`
}

func (WorkspaceIndexStateRecord) TableName() string {
	return "workspace_index_state"
}

type FileIndexListOptions struct {
	Query          string
	Dir            string
	DirectChildren bool
	Kind           string
	IncludeSkipped bool
}

func (s *Store) ReplaceFileIndex(ctx context.Context, workspaceID string, entries []FileIndexRecord, state WorkspaceIndexStateRecord) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		seen := make([]string, 0, len(entries))
		for _, entry := range entries {
			seen = append(seen, entry.Path)
		}
		if len(seen) == 0 {
			if err := tx.Where("workspace_id = ?", workspaceID).Delete(&FileIndexRecord{}).Error; err != nil {
				return err
			}
		} else if err := tx.Where("workspace_id = ? AND path NOT IN ?", workspaceID, seen).Delete(&FileIndexRecord{}).Error; err != nil {
			return err
		}
		if len(entries) > 0 {
			if err := tx.Clauses(clause.OnConflict{UpdateAll: true}).CreateInBatches(entries, 500).Error; err != nil {
				return err
			}
		}
		return tx.Clauses(clause.OnConflict{UpdateAll: true}).Create(&state).Error
	})
}

func (s *Store) UpsertFileIndexEntry(ctx context.Context, entry FileIndexRecord) error {
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{UpdateAll: true}).Create(&entry).Error
}

func (s *Store) DeleteFileIndexEntry(ctx context.Context, workspaceID, path string) error {
	return s.db.WithContext(ctx).Where("workspace_id = ? AND path = ?", workspaceID, path).Delete(&FileIndexRecord{}).Error
}

func (s *Store) ListFileIndex(ctx context.Context, workspaceID string, opts FileIndexListOptions) ([]FileIndexRecord, error) {
	var entries []FileIndexRecord
	query := s.db.WithContext(ctx).Where("workspace_id = ?", workspaceID)
	if opts.Dir != "" {
		query = query.Where("dir = ?", opts.Dir)
	}
	if opts.DirectChildren {
		depth := 1
		if opts.Dir != "" {
			depth = strings.Count(opts.Dir, "/") + 2
		}
		query = query.Where("depth = ?", depth)
	}
	if opts.Kind != "" {
		query = query.Where("kind = ?", opts.Kind)
	}
	if !opts.IncludeSkipped {
		query = query.Where("index_status <> ?", "skipped")
	}
	if err := query.Order("path ASC").Find(&entries).Error; err != nil {
		return nil, err
	}
	entries = filterAndRankFileIndex(entries, opts.Query)
	return entries, nil
}

func (s *Store) GetWorkspaceIndexState(ctx context.Context, workspaceID string) (WorkspaceIndexStateRecord, error) {
	var state WorkspaceIndexStateRecord
	if err := s.db.WithContext(ctx).
		First(&state, "workspace_id = ?", workspaceID).Error; err != nil {
		return WorkspaceIndexStateRecord{}, err
	}
	return state, nil
}

func filterAndRankFileIndex(entries []FileIndexRecord, query string) []FileIndexRecord {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return entries
	}
	type rankedEntry struct {
		record FileIndexRecord
		score  int
	}
	ranked := make([]rankedEntry, 0, len(entries))
	for _, entry := range entries {
		score, ok := fileIndexMatchScore(entry, query)
		if ok {
			ranked = append(ranked, rankedEntry{record: entry, score: score})
		}
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].score != ranked[j].score {
			return ranked[i].score < ranked[j].score
		}
		return ranked[i].record.Path < ranked[j].record.Path
	})
	matched := make([]FileIndexRecord, 0, len(ranked))
	for _, entry := range ranked {
		matched = append(matched, entry.record)
	}
	return matched
}

func fileIndexMatchScore(entry FileIndexRecord, query string) (int, bool) {
	name := entry.NameLower
	path := entry.PathLower
	switch {
	case name == query:
		return 0, true
	case strings.HasPrefix(name, query):
		return 10, true
	case strings.Contains(name, query):
		return 20 + strings.Index(name, query), true
	case strings.Contains(path, query):
		return 40 + strings.Index(path, query), true
	}
	if score, ok := fuzzySubsequenceScore(name, query); ok {
		return 80 + score, true
	}
	if score, ok := fuzzySubsequenceScore(path, query); ok {
		return 120 + score, true
	}
	return 0, false
}

func fuzzySubsequenceScore(value, query string) (int, bool) {
	if query == "" {
		return 0, true
	}
	queryIndex := 0
	first := -1
	last := -1
	gaps := 0
	for valueIndex, char := range value {
		if queryIndex >= len(query) {
			break
		}
		if byte(char) != query[queryIndex] {
			continue
		}
		if first == -1 {
			first = valueIndex
		}
		if last != -1 {
			gaps += valueIndex - last - 1
		}
		last = valueIndex
		queryIndex++
	}
	if queryIndex != len(query) {
		return 0, false
	}
	return first*2 + gaps + len(value) - len(query), true
}
