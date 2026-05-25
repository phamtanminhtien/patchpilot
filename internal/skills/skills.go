package skills

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/phamtanminhtien/patchpilot/internal/config"
	"gopkg.in/yaml.v3"
)

type Warning struct {
	Path    string `json:"path,omitempty"`
	Message string `json:"message"`
}

type Skill struct {
	Key         string    `json:"key"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Path        string    `json:"path"`
	Source      string    `json:"source"`
	Enabled     bool      `json:"enabled"`
	Valid       bool      `json:"valid"`
	Warning     string    `json:"warning,omitempty"`
	Instruction string    `json:"instruction,omitempty"`
	Warnings    []Warning `json:"warnings,omitempty"`
}

type Registry struct {
	Skills   []Skill   `json:"skills"`
	Warnings []Warning `json:"warnings,omitempty"`
}

func Discover(home string) Registry {
	userConfig, err := config.LoadUserConfig(home)
	registry := Registry{}
	if err != nil {
		registry.Warnings = append(registry.Warnings, Warning{Message: "Local config could not be read."})
	}
	seen := map[string]struct{}{}
	for _, source := range []struct {
		name string
		root string
	}{
		{name: "patchpilot", root: filepath.Join(home, ".patchpilot", "skills")},
		{name: "agents", root: filepath.Join(home, ".agents", "skills")},
	} {
		entries, readErr := os.ReadDir(source.root)
		if readErr != nil {
			if !os.IsNotExist(readErr) {
				registry.Warnings = append(registry.Warnings, Warning{Message: source.name + " skills could not be listed."})
			}
			continue
		}
		sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			key := entry.Name()
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			setting := userConfig.Skills[key]
			enabled := true
			if setting.Enabled != nil && !*setting.Enabled {
				enabled = false
			}
			skill := Skill{
				Key:     key,
				Name:    firstNonEmpty(setting.DisplayName, humanizeKey(key)),
				Path:    filepath.ToSlash(filepath.Join(source.name, key)),
				Source:  source.name,
				Enabled: enabled,
			}
			content, readErr := os.ReadFile(filepath.Join(source.root, key, "SKILL.md"))
			if readErr != nil {
				skill.Warning = "SKILL.md could not be read."
				skill.Valid = false
			} else {
				metadata, instruction, parseErr := parseSkillFile(content)
				if parseErr != "" {
					skill.Warning = parseErr
					skill.Valid = false
				} else {
					skill.Name = metadata.Name
					skill.Description = metadata.Description
					skill.Instruction = instruction
					skill.Valid = true
				}
			}
			if skill.Warning != "" {
				skill.Warnings = append(skill.Warnings, Warning{Path: skill.Path + "/SKILL.md", Message: skill.Warning})
			}
			registry.Skills = append(registry.Skills, skill)
		}
	}
	markDuplicateEnabledNames(&registry)
	return registry
}

func EnabledContext(registry Registry) []Skill {
	out := make([]Skill, 0, len(registry.Skills))
	for _, skill := range registry.Skills {
		if skill.Enabled && skill.Valid {
			out = append(out, skill)
		}
	}
	return out
}

func SetEnabled(home, key string, enabled bool) (Skill, Registry, error) {
	cfg, err := config.LoadUserConfig(home)
	if err != nil {
		return Skill{}, Registry{}, err
	}
	if cfg.Skills == nil {
		cfg.Skills = map[string]config.SkillConfig{}
	}
	setting := cfg.Skills[key]
	if enabled {
		setting.Enabled = nil
		if setting.DisplayName == "" {
			delete(cfg.Skills, key)
		} else {
			cfg.Skills[key] = setting
		}
	} else {
		setting.Enabled = &enabled
		cfg.Skills[key] = setting
	}
	if err := config.SaveUserConfig(home, cfg); err != nil {
		return Skill{}, Registry{}, err
	}
	registry := Discover(home)
	for _, skill := range registry.Skills {
		if skill.Key == key {
			return skill, registry, nil
		}
	}
	return Skill{Key: key, Name: humanizeKey(key), Enabled: enabled, Valid: false, Warning: "Skill was not found."}, registry, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

type skillMetadata struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

func parseSkillFile(content []byte) (skillMetadata, string, string) {
	text := strings.ReplaceAll(string(content), "\r\n", "\n")
	lines := strings.Split(text, "\n")
	if len(lines) < 3 || strings.TrimSpace(lines[0]) != "---" {
		return skillMetadata{}, "", "SKILL.md must start with YAML frontmatter."
	}
	closing := -1
	for index := 1; index < len(lines); index++ {
		if strings.TrimSpace(lines[index]) == "---" {
			closing = index
			break
		}
	}
	if closing == -1 {
		return skillMetadata{}, "", "SKILL.md YAML frontmatter is not closed."
	}
	metadata, err := parseSkillMetadata([]byte(strings.Join(lines[1:closing], "\n")))
	if err != nil {
		return skillMetadata{}, "", "SKILL.md YAML frontmatter could not be parsed."
	}
	metadata.Name = strings.TrimSpace(metadata.Name)
	metadata.Description = strings.TrimSpace(metadata.Description)
	if metadata.Name == "" {
		return skillMetadata{}, "", "SKILL.md frontmatter requires a non-empty name."
	}
	if metadata.Description == "" {
		return skillMetadata{}, "", "SKILL.md frontmatter requires a non-empty description."
	}
	instruction := strings.TrimSpace(strings.Join(lines[closing+1:], "\n"))
	if instruction == "" {
		return skillMetadata{}, "", "SKILL.md body must not be empty."
	}
	return metadata, instruction, ""
}

func parseSkillMetadata(content []byte) (skillMetadata, error) {
	var document yaml.Node
	if err := yaml.Unmarshal(content, &document); err != nil {
		return skillMetadata{}, err
	}
	if len(document.Content) == 0 || document.Content[0].Kind != yaml.MappingNode {
		return skillMetadata{}, errors.New("invalid skill metadata")
	}
	values := map[string]string{}
	mapping := document.Content[0]
	for index := 0; index+1 < len(mapping.Content); index += 2 {
		keyNode := mapping.Content[index]
		valueNode := mapping.Content[index+1]
		if keyNode.Kind != yaml.ScalarNode || keyNode.Value == "" {
			continue
		}
		if keyNode.Value != "name" && keyNode.Value != "description" {
			continue
		}
		if valueNode.Kind != yaml.ScalarNode || valueNode.Tag != "!!str" {
			return skillMetadata{}, errors.New("invalid skill metadata")
		}
		values[keyNode.Value] = valueNode.Value
	}
	return skillMetadata{Name: values["name"], Description: values["description"]}, nil
}

func markDuplicateEnabledNames(registry *Registry) {
	seen := map[string]struct{}{}
	for index := range registry.Skills {
		skill := &registry.Skills[index]
		if !skill.Enabled || !skill.Valid {
			continue
		}
		nameKey := strings.ToLower(strings.TrimSpace(skill.Name))
		if _, ok := seen[nameKey]; ok {
			skill.Valid = false
			skill.Warning = "Skill name duplicates another enabled skill."
			skill.Warnings = append(skill.Warnings, Warning{Path: skill.Path + "/SKILL.md", Message: skill.Warning})
			continue
		}
		seen[nameKey] = struct{}{}
	}
}

func humanizeKey(key string) string {
	parts := strings.FieldsFunc(key, func(r rune) bool {
		return r == '-' || r == '_' || r == '.'
	})
	for index, part := range parts {
		if part == "" {
			continue
		}
		parts[index] = strings.ToUpper(part[:1]) + part[1:]
	}
	return strings.Join(parts, " ")
}
