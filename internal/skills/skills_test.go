package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/phamtanminhtien/patchpilot/internal/config"
)

func TestDiscoverLoadsAllSkillsAndOnlyExplicitFalseDisables(t *testing.T) {
	home := t.TempDir()
	writeSkill(t, home, ".agents", "fallback", skillFile("Fallback", "Fallback skill.", "Use fallback."))
	writeSkill(t, home, ".patchpilot", "local", skillFile("Local", "Local skill.", "Use local."))
	trueValue := true
	falseValue := false
	if err := config.SaveUserConfig(home, config.UserConfig{
		Skills: map[string]config.SkillConfig{
			"fallback": {Enabled: &trueValue},
			"local":    {Enabled: &falseValue},
		},
	}); err != nil {
		t.Fatalf("SaveUserConfig returned error: %v", err)
	}

	registry := Discover(home)
	if len(registry.Skills) != 2 {
		t.Fatalf("expected all discovered skills to be listed, got %+v", registry.Skills)
	}
	if skillByKey(registry, "fallback").Enabled != true {
		t.Fatalf("expected explicit true to behave as enabled, got %+v", registry.Skills)
	}
	if skillByKey(registry, "local").Enabled != false {
		t.Fatalf("expected explicit false to disable skill, got %+v", registry.Skills)
	}
	if got := skillByKey(registry, "fallback"); got.Name != "Fallback" || got.Description != "Fallback skill." || got.Instruction != "Use fallback." {
		t.Fatalf("expected parsed skill metadata and body, got %+v", got)
	}
}

func TestSetEnabledRemovesDisableInsteadOfWritingExplicitTrue(t *testing.T) {
	home := t.TempDir()
	writeSkill(t, home, ".patchpilot", "local", skillFile("Local", "Local skill.", "Use local."))
	falseValue := false
	if err := config.SaveUserConfig(home, config.UserConfig{
		Skills: map[string]config.SkillConfig{
			"local": {Enabled: &falseValue},
		},
	}); err != nil {
		t.Fatalf("SaveUserConfig returned error: %v", err)
	}

	skill, _, err := SetEnabled(home, "local", true)
	if err != nil {
		t.Fatalf("SetEnabled returned error: %v", err)
	}
	if !skill.Enabled {
		t.Fatalf("expected skill to be enabled, got %+v", skill)
	}
	cfg, err := config.LoadUserConfig(home)
	if err != nil {
		t.Fatalf("LoadUserConfig returned error: %v", err)
	}
	if _, ok := cfg.Skills["local"]; ok {
		t.Fatalf("expected enabled skill entry to be removed from config, got %+v", cfg.Skills)
	}
}

func TestDiscoverRequiresYamlFrontmatterNameDescriptionAndBody(t *testing.T) {
	tests := []struct {
		name    string
		content string
		warning string
	}{
		{
			name:    "missing_frontmatter",
			content: "# Missing",
			warning: "frontmatter",
		},
		{
			name: "missing_name",
			content: `---
description: Missing name.
---
Use skill.`,
			warning: "name",
		},
		{
			name: "missing_description",
			content: `---
name: Missing description
---
Use skill.`,
			warning: "description",
		},
		{
			name: "empty_body",
			content: `---
name: Empty body
description: Empty body skill.
---
`,
			warning: "body",
		},
		{
			name: "invalid_yaml",
			content: `---
name: [bad
description: Invalid YAML.
---
Use skill.`,
			warning: "parsed",
		},
		{
			name: "wrong_type",
			content: `---
name: 1
description: Wrong type.
---
Use skill.`,
			warning: "parsed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := t.TempDir()
			writeSkill(t, home, ".patchpilot", "local", tt.content)

			skill := skillByKey(Discover(home), "local")
			if skill.Valid {
				t.Fatalf("expected invalid skill, got %+v", skill)
			}
			if !strings.Contains(skill.Warning, tt.warning) {
				t.Fatalf("expected warning containing %q, got %+v", tt.warning, skill)
			}
		})
	}
}

func TestDiscoverAcceptsLargeYamlSkillAndMultilineDescription(t *testing.T) {
	home := t.TempDir()
	body := strings.Repeat("Use skill.\n", 20*1024)
	writeSkill(t, home, ".patchpilot", "large", `---
name: Large Skill
description: >-
  First line
  second line
---
`+body)

	skill := skillByKey(Discover(home), "large")
	if !skill.Valid {
		t.Fatalf("expected large skill to be valid, got %+v", skill)
	}
	if skill.Description != "First line second line" {
		t.Fatalf("expected folded multiline description, got %q", skill.Description)
	}
	if len(skill.Instruction) < 12*1024 {
		t.Fatalf("expected large instruction body, got %d bytes", len(skill.Instruction))
	}
}

func TestDiscoverMarksDuplicateEnabledNamesInvalid(t *testing.T) {
	home := t.TempDir()
	writeSkill(t, home, ".patchpilot", "alpha", skillFile("Shared", "First skill.", "Use first."))
	writeSkill(t, home, ".patchpilot", "beta", skillFile("Shared", "Second skill.", "Use second."))

	registry := Discover(home)
	if !skillByKey(registry, "alpha").Valid {
		t.Fatalf("expected first duplicate skill to remain valid, got %+v", registry.Skills)
	}
	beta := skillByKey(registry, "beta")
	if beta.Valid || !strings.Contains(beta.Warning, "duplicates") {
		t.Fatalf("expected second duplicate skill to be invalid, got %+v", beta)
	}
}

func TestDiscoverAllowsDuplicateNameWhenEarlierSkillDisabled(t *testing.T) {
	home := t.TempDir()
	writeSkill(t, home, ".patchpilot", "alpha", skillFile("Shared", "First skill.", "Use first."))
	writeSkill(t, home, ".patchpilot", "beta", skillFile("Shared", "Second skill.", "Use second."))
	falseValue := false
	if err := config.SaveUserConfig(home, config.UserConfig{
		Skills: map[string]config.SkillConfig{
			"alpha": {Enabled: &falseValue},
		},
	}); err != nil {
		t.Fatalf("SaveUserConfig returned error: %v", err)
	}

	beta := skillByKey(Discover(home), "beta")
	if !beta.Valid {
		t.Fatalf("expected enabled duplicate to be valid when earlier skill is disabled, got %+v", beta)
	}
}

func TestDiscoverUsesPatchPilotSkillBeforeAgentsSkillForDuplicateKey(t *testing.T) {
	home := t.TempDir()
	writeSkill(t, home, ".agents", "local", skillFile("Fallback", "Fallback skill.", "Use fallback."))
	writeSkill(t, home, ".patchpilot", "local", skillFile("Local", "Local skill.", "Use local."))

	registry := Discover(home)
	if len(registry.Skills) != 1 {
		t.Fatalf("expected duplicate key to be listed once, got %+v", registry.Skills)
	}
	if got := registry.Skills[0]; got.Source != "patchpilot" || got.Name != "Local" {
		t.Fatalf("expected patchpilot skill precedence, got %+v", got)
	}
}

func skillByKey(registry Registry, key string) Skill {
	for _, skill := range registry.Skills {
		if skill.Key == key {
			return skill
		}
	}
	return Skill{}
}

func skillFile(name, description, body string) string {
	return `---
name: ` + name + `
description: ` + description + `
---
` + body
}

func writeSkill(t *testing.T, home, root, key, content string) {
	t.Helper()
	path := filepath.Join(home, root, "skills", key, "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir skill: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write skill: %v", err)
	}
}
