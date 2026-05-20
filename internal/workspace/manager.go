package workspace

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
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

type Manager struct {
	allowedRoots []string
	nextID       atomic.Uint64

	mu         sync.RWMutex
	workspaces map[string]Workspace
}

func NewManager(allowedRoots []string) (*Manager, error) {
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
		workspaces:   make(map[string]Workspace),
	}, nil
}

func (m *Manager) Create(rootPath string) (Workspace, error) {
	root, err := m.normalizeRoot(rootPath)
	if err != nil {
		return Workspace{}, err
	}
	if _, err := os.Stat(filepath.Join(root, ".git")); err != nil {
		return Workspace{}, ErrNotGitRepo
	}

	now := time.Now().UTC()
	id := fmt.Sprintf("ws_%d", m.nextID.Add(1))
	ws := Workspace{
		ID:        id,
		Name:      filepath.Base(root),
		RootPath:  root,
		Status:    "ready",
		CreatedAt: now,
		UpdatedAt: now,
	}

	m.mu.Lock()
	m.workspaces[id] = ws
	m.mu.Unlock()

	return ws, nil
}

func (m *Manager) Get(id string) (Workspace, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ws, ok := m.workspaces[id]
	if !ok {
		return Workspace{}, ErrNotFound
	}
	return ws, nil
}

func (m *Manager) List() []Workspace {
	m.mu.RLock()
	defer m.mu.RUnlock()

	workspaces := make([]Workspace, 0, len(m.workspaces))
	for _, ws := range m.workspaces {
		workspaces = append(workspaces, ws)
	}
	sort.Slice(workspaces, func(i, j int) bool {
		if workspaces[i].CreatedAt.Equal(workspaces[j].CreatedAt) {
			return workspaces[i].ID > workspaces[j].ID
		}
		return workspaces[i].CreatedAt.After(workspaces[j].CreatedAt)
	})
	return workspaces
}

func (m *Manager) normalizeRoot(rootPath string) (string, error) {
	if strings.TrimSpace(rootPath) == "" || !filepath.IsAbs(rootPath) {
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
