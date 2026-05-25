package agent

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/phamtanminhtien/patchpilot/internal/config"
)

func TestDiscoverRepoInstructionsOrdersRootBeforeDescendantsAndSkipsSymlinks(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "AGENTS.md"), "root rules")
	writeTestFile(t, filepath.Join(root, "web", "AGENTS.md"), "web rules")
	writeTestFile(t, filepath.Join(root, "web", "src", "AGENTS.md"), "src rules")
	if err := os.Symlink(t.TempDir(), filepath.Join(root, "linked")); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	sources, warnings, err := DiscoverRepoInstructions(root)
	if err != nil {
		t.Fatalf("DiscoverRepoInstructions returned error: %v", err)
	}
	got := make([]string, 0, len(sources))
	for _, source := range sources {
		got = append(got, source.Path)
	}
	want := []string{"AGENTS.md", "web/AGENTS.md", "web/src/AGENTS.md"}
	if len(got) != len(want) {
		t.Fatalf("expected sources %v, got %v", want, got)
	}
	for index := range want {
		if got[index] != want[index] || sources[index].Precedence != index {
			t.Fatalf("unexpected source order: %+v", sources)
		}
	}
	if len(warnings) != 1 || warnings[0].Path != "linked" {
		t.Fatalf("expected safe symlink warning, got %+v", warnings)
	}
}

func TestRefreshContextDiscoversSkillsAndMCPWithoutSecretValues(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	writeTestFile(t, filepath.Join(root, "AGENTS.md"), "root rules")
	writeTestFile(t, filepath.Join(home, ".patchpilot", "skills", "local", "SKILL.md"), testSkillFile("Local", "Local skill.", "Use local skill."))
	writeTestFile(t, filepath.Join(home, ".agents", "skills", "local", "SKILL.md"), testSkillFile("Fallback", "Fallback skill.", "Do not use."))
	cfg := config.UserConfig{
		MCPServers: map[string]config.MCPServerConfig{
			"demo": {
				Transport:      "stdio",
				Command:        "demo-mcp",
				ApprovalPolicy: "always",
				Env:            map[string]string{"SECRET_TOKEN": "${PATCHPILOT_TEST_SECRET}"},
			},
		},
	}
	if err := config.SaveUserConfig(home, cfg); err != nil {
		t.Fatalf("SaveUserConfig returned error: %v", err)
	}

	manager := NewManagerWithHome(nil, nil, nil, nil, nil, nil, home)
	snapshot, err := manager.RefreshContext(context.Background(), root)
	if err != nil {
		t.Fatalf("RefreshContext returned error: %v", err)
	}
	if len(snapshot.InstructionSources) != 1 || snapshot.InstructionSources[0].Path != "AGENTS.md" {
		t.Fatalf("unexpected instruction sources: %+v", snapshot.InstructionSources)
	}
	if len(snapshot.Skills) != 1 || snapshot.Skills[0].Source != "patchpilot" || !snapshot.Skills[0].Enabled {
		t.Fatalf("expected patchpilot skill precedence, got %+v", snapshot.Skills)
	}
	if snapshot.Skills[0].Name != "Local" || snapshot.Skills[0].Description != "Local skill." {
		t.Fatalf("expected skill metadata, got %+v", snapshot.Skills[0])
	}
	if len(snapshot.MCPServers) != 1 || snapshot.MCPServers[0].ID != "demo" {
		t.Fatalf("unexpected MCP servers: %+v", snapshot.MCPServers)
	}
	if len(snapshot.ContextWarnings) != 1 || snapshot.ContextWarnings[0].Message == "${PATCHPILOT_TEST_SECRET}" {
		t.Fatalf("expected safe unresolved env warning, got %+v", snapshot.ContextWarnings)
	}
}

func testSkillFile(name, description, body string) string {
	return `---
name: ` + name + `
description: ` + description + `
---
` + body
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
}
