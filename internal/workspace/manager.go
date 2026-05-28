package workspace

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/phamtanminhtien/patchpilot/internal/database"
	"github.com/phamtanminhtien/patchpilot/internal/gitrepo"
)

var (
	ErrInvalidRoot   = errors.New("invalid workspace root")
	ErrNotGitRepo    = errors.New("workspace root is not a git repository")
	ErrNotFound      = errors.New("workspace not found")
	ErrOutsideRoots  = errors.New("workspace root is outside allowed roots")
	errNoAllowedRoot = errors.New("at least one allowed root is required")
)

type Workspace struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	RootPath  string    `json:"rootPath"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type FileIndexEntry struct {
	Path        string    `json:"path"`
	Name        string    `json:"name"`
	Dir         string    `json:"dir"`
	Extension   string    `json:"extension"`
	Kind        string    `json:"kind"`
	IndexStatus string    `json:"indexStatus"`
	Size        int64     `json:"size"`
	ModifiedAt  time.Time `json:"modifiedAt"`
}

type FileIndexState struct {
	Status         string     `json:"status"`
	LastIndexedAt  *time.Time `json:"lastIndexedAt,omitempty"`
	LastFullScanAt *time.Time `json:"lastFullScanAt,omitempty"`
	FileCount      int        `json:"fileCount"`
	SkippedCount   int        `json:"skippedCount"`
	Truncated      bool       `json:"truncated"`
	Error          *string    `json:"error,omitempty"`
	UpdatedAt      time.Time  `json:"updatedAt"`
}

type FileIndexListOptions struct {
	Query          string
	Dir            string
	DirectChildren bool
	Kind           string
	IncludeSkipped bool
}

type Manager struct {
	allowedRoots []string
	store        *database.Store
	git          *gitrepo.Client
}

func NewManager(allowedRoots []string, store *database.Store, git *gitrepo.Client) (*Manager, error) {
	if store == nil {
		return nil, errors.New("workspace store is required")
	}
	if git == nil {
		return nil, errors.New("git client is required")
	}
	normalized := make([]string, 0, len(allowedRoots))
	for _, root := range allowedRoots {
		root = strings.TrimSpace(root)
		if root == "" {
			continue
		}
		abs, err := filepath.Abs(root)
		if err != nil {
			return nil, fmt.Errorf("%w: %s", ErrInvalidRoot, root)
		}
		clean, err := filepath.EvalSymlinks(abs)
		if err != nil {
			return nil, fmt.Errorf("%w: %s", ErrInvalidRoot, root)
		}
		normalized = append(normalized, clean)
	}
	if len(normalized) == 0 {
		return nil, errNoAllowedRoot
	}

	return &Manager{
		allowedRoots: normalized,
		store:        store,
		git:          git,
	}, nil
}

func (m *Manager) Create(ctx context.Context, rootPath string) (Workspace, error) {
	root, err := m.normalizeRoot(rootPath)
	if err != nil {
		return Workspace{}, err
	}
	repositoryRoot, err := m.git.RepositoryRoot(ctx, root)
	if err != nil {
		return Workspace{}, ErrNotGitRepo
	}
	if repositoryRoot != root {
		return Workspace{}, ErrNotGitRepo
	}

	existing, err := m.store.FindWorkspaceByRoot(ctx, root)
	if err == nil {
		touched, err := m.store.TouchWorkspace(ctx, existing.ID, time.Now().UTC())
		if err != nil {
			return Workspace{}, err
		}
		return fromRecord(touched), nil
	}
	if !errors.Is(err, database.ErrNotFound) {
		return Workspace{}, err
	}

	now := time.Now().UTC()
	id, err := newWorkspaceID()
	if err != nil {
		return Workspace{}, err
	}
	record, err := m.store.CreateWorkspace(ctx, database.WorkspaceRecord{
		ID:        id,
		Name:      filepath.Base(root),
		RootPath:  root,
		Status:    "ready",
		CreatedAt: now,
		UpdatedAt: now,
	})
	if err != nil {
		return Workspace{}, err
	}

	return fromRecord(record), nil
}

func (m *Manager) Get(ctx context.Context, id string) (Workspace, error) {
	record, err := m.store.GetWorkspace(ctx, id)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return Workspace{}, ErrNotFound
		}
		return Workspace{}, err
	}
	if !m.isAllowed(record.RootPath) {
		return Workspace{}, ErrNotFound
	}
	return fromRecord(record), nil
}

func (m *Manager) List(ctx context.Context) ([]Workspace, error) {
	records, err := m.store.ListWorkspaces(ctx)
	if err != nil {
		return nil, err
	}
	workspaces := make([]Workspace, 0, len(records))
	for _, record := range records {
		if !m.isAllowed(record.RootPath) {
			continue
		}
		workspaces = append(workspaces, fromRecord(record))
	}
	return workspaces, nil
}

func (m *Manager) Delete(ctx context.Context, workspaceID string) error {
	if _, err := m.Get(ctx, workspaceID); err != nil {
		return err
	}
	return m.store.DeleteWorkspaceMetadata(ctx, workspaceID)
}

func (m *Manager) ReplaceFileIndex(ctx context.Context, workspaceID string, entries []FileIndexEntry) error {
	if _, err := m.Get(ctx, workspaceID); err != nil {
		return err
	}
	now := time.Now().UTC()
	records := make([]database.FileIndexRecord, 0, len(entries))
	fileCount := 0
	skippedCount := 0
	for _, entry := range entries {
		if entry.Kind == "" {
			entry.Kind = "file"
		}
		if entry.IndexStatus == "" {
			entry.IndexStatus = "indexed"
		}
		if entry.Name == "" {
			entry.Name = filepath.Base(entry.Path)
		}
		if entry.Dir == "" {
			entry.Dir = pathDir(entry.Path)
		}
		if entry.Extension == "" && entry.Kind == "file" {
			entry.Extension = strings.TrimPrefix(strings.ToLower(filepath.Ext(entry.Name)), ".")
		}
		if entry.Kind == "file" {
			fileCount++
		}
		if entry.IndexStatus == "skipped" {
			skippedCount++
		}
		records = append(records, database.FileIndexRecord{
			WorkspaceID: workspaceID,
			Path:        entry.Path,
			Name:        entry.Name,
			Dir:         entry.Dir,
			Extension:   entry.Extension,
			Kind:        entry.Kind,
			IndexStatus: entry.IndexStatus,
			PathLower:   strings.ToLower(entry.Path),
			NameLower:   strings.ToLower(entry.Name),
			Depth:       pathDepth(entry.Path),
			Size:        entry.Size,
			ModifiedAt:  entry.ModifiedAt,
			IndexedAt:   now,
		})
	}
	return m.store.ReplaceFileIndex(ctx, workspaceID, records, database.WorkspaceIndexStateRecord{
		WorkspaceID:    workspaceID,
		Status:         "ready",
		LastIndexedAt:  &now,
		LastFullScanAt: &now,
		FileCount:      fileCount,
		SkippedCount:   skippedCount,
		Truncated:      false,
		UpdatedAt:      now,
	})
}

func (m *Manager) UpsertFileIndexEntry(ctx context.Context, workspaceID string, entry FileIndexEntry) error {
	if _, err := m.Get(ctx, workspaceID); err != nil {
		return err
	}
	now := time.Now().UTC()
	if entry.Kind == "" {
		entry.Kind = "file"
	}
	if entry.IndexStatus == "" {
		entry.IndexStatus = "indexed"
	}
	if entry.Name == "" {
		entry.Name = filepath.Base(entry.Path)
	}
	if entry.Dir == "" {
		entry.Dir = pathDir(entry.Path)
	}
	if entry.Extension == "" && entry.Kind == "file" {
		entry.Extension = strings.TrimPrefix(strings.ToLower(filepath.Ext(entry.Name)), ".")
	}
	return m.store.UpsertFileIndexEntry(ctx, database.FileIndexRecord{
		WorkspaceID: workspaceID,
		Path:        entry.Path,
		Name:        entry.Name,
		Dir:         entry.Dir,
		Extension:   entry.Extension,
		Kind:        entry.Kind,
		IndexStatus: entry.IndexStatus,
		PathLower:   strings.ToLower(entry.Path),
		NameLower:   strings.ToLower(entry.Name),
		Depth:       pathDepth(entry.Path),
		Size:        entry.Size,
		ModifiedAt:  entry.ModifiedAt,
		IndexedAt:   now,
	})
}

func (m *Manager) DeleteFileIndexEntry(ctx context.Context, workspaceID, path string) error {
	if _, err := m.Get(ctx, workspaceID); err != nil {
		return err
	}
	return m.store.DeleteFileIndexEntry(ctx, workspaceID, path)
}

func (m *Manager) FileIndex(ctx context.Context, workspaceID string, opts FileIndexListOptions) ([]FileIndexEntry, *FileIndexState, error) {
	if _, err := m.Get(ctx, workspaceID); err != nil {
		return nil, nil, err
	}
	records, err := m.store.ListFileIndex(ctx, workspaceID, database.FileIndexListOptions{
		Query:          strings.ToLower(strings.TrimSpace(opts.Query)),
		Dir:            strings.Trim(strings.TrimSpace(opts.Dir), "/"),
		DirectChildren: opts.DirectChildren,
		Kind:           strings.TrimSpace(opts.Kind),
		IncludeSkipped: opts.IncludeSkipped,
	})
	if err != nil {
		return nil, nil, err
	}
	entries := make([]FileIndexEntry, 0, len(records))
	for _, record := range records {
		entries = append(entries, FileIndexEntry{
			Path:        record.Path,
			Name:        record.Name,
			Dir:         record.Dir,
			Extension:   record.Extension,
			Kind:        record.Kind,
			IndexStatus: record.IndexStatus,
			Size:        record.Size,
			ModifiedAt:  record.ModifiedAt,
		})
	}
	stateRecord, err := m.store.GetWorkspaceIndexState(ctx, workspaceID)
	if err != nil {
		return entries, nil, nil
	}
	state := FileIndexState{
		Status:         stateRecord.Status,
		LastIndexedAt:  stateRecord.LastIndexedAt,
		LastFullScanAt: stateRecord.LastFullScanAt,
		FileCount:      stateRecord.FileCount,
		SkippedCount:   stateRecord.SkippedCount,
		Truncated:      stateRecord.Truncated,
		Error:          stateRecord.Error,
		UpdatedAt:      stateRecord.UpdatedAt,
	}
	return entries, &state, nil
}

func (m *Manager) normalizeRoot(rootPath string) (string, error) {
	rootPath = strings.TrimSpace(rootPath)
	if rootPath == "" || !filepath.IsAbs(rootPath) {
		return "", ErrInvalidRoot
	}
	clean, err := filepath.EvalSymlinks(filepath.Clean(rootPath))
	if err != nil {
		return "", fmt.Errorf("%w: %s", ErrInvalidRoot, rootPath)
	}
	if !m.isAllowed(clean) {
		return "", ErrOutsideRoots
	}
	return clean, nil
}

func (m *Manager) isAllowed(root string) bool {
	for _, allowed := range m.allowedRoots {
		if root == allowed {
			return true
		}
		rel, err := filepath.Rel(allowed, root)
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

func fromRecord(record database.WorkspaceRecord) Workspace {
	return Workspace{
		ID:        record.ID,
		Name:      record.Name,
		RootPath:  record.RootPath,
		Status:    record.Status,
		CreatedAt: record.CreatedAt,
		UpdatedAt: record.UpdatedAt,
	}
}

func newWorkspaceID() (string, error) {
	var random [8]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", err
	}
	return "ws_" + hex.EncodeToString(random[:]), nil
}

func pathDir(path string) string {
	dir := filepath.ToSlash(filepath.Dir(filepath.ToSlash(path)))
	if dir == "." {
		return ""
	}
	return dir
}

func pathDepth(path string) int {
	parts := strings.Split(filepath.ToSlash(path), "/")
	depth := 0
	for _, part := range parts {
		if part != "" {
			depth++
		}
	}
	return depth
}
