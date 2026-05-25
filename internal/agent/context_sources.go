package agent

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/phamtanminhtien/patchpilot/internal/filestore"
	"github.com/phamtanminhtien/patchpilot/internal/mcp"
	"github.com/phamtanminhtien/patchpilot/internal/skills"
)

type ContextWarning struct {
	Path    string `json:"path,omitempty"`
	Message string `json:"message"`
}

type InstructionSource struct {
	Path       string `json:"path"`
	Content    string `json:"content"`
	Precedence int    `json:"precedence"`
}

type ContextSnapshot struct {
	InstructionSources []InstructionSource `json:"instructionSources"`
	SkippedSources     []ContextWarning    `json:"skippedSources,omitempty"`
	Skills             []skills.Skill      `json:"skills"`
	MCPServers         []mcp.Server        `json:"mcpServers"`
	MCPTools           []mcp.Tool          `json:"mcpTools"`
	ContextWarnings    []ContextWarning    `json:"contextWarnings,omitempty"`
	RefreshedAt        time.Time           `json:"refreshedAt"`
}

func (m *Manager) RefreshContext(ctx context.Context, workspaceRoot string) (ContextSnapshot, error) {
	select {
	case <-ctx.Done():
		return ContextSnapshot{}, ctx.Err()
	default:
	}
	instructions, skipped, err := DiscoverRepoInstructions(workspaceRoot)
	if err != nil {
		return ContextSnapshot{}, err
	}
	home := m.homeDir
	if strings.TrimSpace(home) == "" {
		home, _ = os.UserHomeDir()
	}
	skillRegistry := skills.Discover(home)
	mcpRegistry := mcp.Discover(home)
	warnings := make([]ContextWarning, 0, len(skillRegistry.Warnings)+len(mcpRegistry.Warnings))
	for _, warning := range skillRegistry.Warnings {
		warnings = append(warnings, ContextWarning{Path: warning.Path, Message: warning.Message})
	}
	for _, warning := range mcpRegistry.Warnings {
		warnings = append(warnings, ContextWarning{Path: warning.ServerID, Message: warning.Message})
	}
	if instructions == nil {
		instructions = []InstructionSource{}
	}
	if skipped == nil {
		skipped = []ContextWarning{}
	}
	if skillRegistry.Skills == nil {
		skillRegistry.Skills = []skills.Skill{}
	}
	if mcpRegistry.Servers == nil {
		mcpRegistry.Servers = []mcp.Server{}
	}
	if mcpRegistry.Tools == nil {
		mcpRegistry.Tools = []mcp.Tool{}
	}
	return ContextSnapshot{
		InstructionSources: instructions,
		SkippedSources:     skipped,
		Skills:             skillRegistry.Skills,
		MCPServers:         mcpRegistry.Servers,
		MCPTools:           mcpRegistry.Tools,
		ContextWarnings:    warnings,
		RefreshedAt:        time.Now().UTC(),
	}, nil
}

func DiscoverRepoInstructions(workspaceRoot string) ([]InstructionSource, []ContextWarning, error) {
	root, err := filepath.EvalSymlinks(workspaceRoot)
	if err != nil {
		return nil, nil, err
	}
	type candidate struct {
		path string
		rel  string
	}
	candidates := make([]candidate, 0)
	warnings := make([]ContextWarning, 0)
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			warnings = append(warnings, ContextWarning{Message: "A workspace path could not be inspected."})
			return nil
		}
		if path == root {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			warnings = append(warnings, ContextWarning{Message: "A path outside the workspace was skipped."})
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		rel = filepath.ToSlash(rel)
		if entry.Type()&fs.ModeSymlink != 0 {
			warnings = append(warnings, ContextWarning{Path: rel, Message: "Symlink path was skipped."})
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() && isIgnoredInstructionDir(entry.Name()) {
			return filepath.SkipDir
		}
		if entry.Name() == "AGENTS.md" {
			candidates = append(candidates, candidate{path: path, rel: rel})
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].rel == "AGENTS.md" {
			return true
		}
		if candidates[j].rel == "AGENTS.md" {
			return false
		}
		leftDepth := strings.Count(candidates[i].rel, "/")
		rightDepth := strings.Count(candidates[j].rel, "/")
		if leftDepth != rightDepth {
			return leftDepth < rightDepth
		}
		return candidates[i].rel < candidates[j].rel
	})
	sources := make([]InstructionSource, 0, len(candidates))
	for _, candidate := range candidates {
		info, statErr := os.Stat(candidate.path)
		if statErr != nil {
			warnings = append(warnings, ContextWarning{Path: candidate.rel, Message: "Instruction file could not be read."})
			continue
		}
		if info.Size() > filestore.MaxReadableFileSize {
			warnings = append(warnings, ContextWarning{Path: candidate.rel, Message: "Instruction file is too large."})
			continue
		}
		content, readErr := os.ReadFile(candidate.path)
		if readErr != nil {
			warnings = append(warnings, ContextWarning{Path: candidate.rel, Message: "Instruction file could not be read."})
			continue
		}
		if !looksText(content) {
			warnings = append(warnings, ContextWarning{Path: candidate.rel, Message: "Instruction file is not text."})
			continue
		}
		sources = append(sources, InstructionSource{Path: candidate.rel, Content: strings.TrimSpace(string(content)), Precedence: len(sources)})
	}
	return sources, warnings, nil
}

func isIgnoredInstructionDir(name string) bool {
	return name == ".git" || name == "node_modules" || name == "build"
}

func looksText(content []byte) bool {
	for _, b := range content {
		if b == 0 {
			return false
		}
	}
	return true
}
