package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
)

const (
	defaultAddr          = "127.0.0.1:8080"
	defaultDataDir       = ".patchpilot"
	defaultDBName        = "patchpilot.db"
	defaultLightModel    = "gpt-5.4-mini"
	defaultOpenAIBaseURL = "https://api.openai.com/v1"
	defaultStaticDir     = "web/dist"
)

type Config struct {
	Addr          string
	AdminToken    string
	AllowedRoots  []string
	DataDir       string
	DBPath        string
	HomeDir       string
	LogFormat     string
	LightModel    string
	OpenAIAPIKey  string
	OpenAIBaseURL string
	StaticDir     string
}

type UserConfig struct {
	Preferences          SettingsPreferences            `json:"preferences,omitempty"`
	Fonts                []InstalledFont                `json:"fonts,omitempty"`
	Skills               map[string]SkillConfig         `json:"skills,omitempty"`
	MCPServers           map[string]MCPServerConfig     `json:"mcpServers,omitempty"`
	WorkspacePermissions map[string]WorkspacePermission `json:"workspacePermissions,omitempty"`
}

type SettingsPreferences struct {
	Theme                  string `json:"theme,omitempty"`
	AppFontFamily          string `json:"appFontFamily,omitempty"`
	CodeFontFamily         string `json:"codeFontFamily,omitempty"`
	TerminalFontFamily     string `json:"terminalFontFamily,omitempty"`
	DefaultModel           string `json:"defaultModel,omitempty"`
	DefaultReasoningEffort string `json:"defaultReasoningEffort,omitempty"`
}

type WorkspacePermission struct {
	Mode          string `json:"mode"`
	EditFiles     bool   `json:"editFiles"`
	RunCommands   bool   `json:"runCommands"`
	GitOperations bool   `json:"gitOperations"`
}

func DefaultWorkspacePermission() WorkspacePermission {
	return WorkspacePermission{
		Mode:          "balanced",
		EditFiles:     true,
		RunCommands:   true,
		GitOperations: true,
	}
}

func NormalizeWorkspacePermission(permission WorkspacePermission) WorkspacePermission {
	if permission.Mode == "" {
		permission.Mode = "balanced"
	}
	return permission
}

type InstalledFont struct {
	ID        string `json:"id"`
	Family    string `json:"family"`
	Filename  string `json:"filename"`
	MimeType  string `json:"mimeType,omitempty"`
	Size      int64  `json:"size"`
	CreatedAt string `json:"createdAt"`
}

type SkillConfig struct {
	Enabled     *bool  `json:"enabled,omitempty"`
	DisplayName string `json:"displayName,omitempty"`
}

type MCPServerConfig struct {
	Disabled       bool              `json:"disabled,omitempty"`
	Transport      string            `json:"transport,omitempty"`
	Command        string            `json:"command,omitempty"`
	Args           []string          `json:"args,omitempty"`
	URL            string            `json:"url,omitempty"`
	Env            map[string]string `json:"env,omitempty"`
	ApprovalPolicy string            `json:"approvalPolicy,omitempty"`
}

func Load() (Config, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return Config{}, err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return Config{}, err
	}
	dotenv, err := loadDotEnv(filepath.Join(cwd, ".env"))
	if err != nil {
		return Config{}, err
	}
	return LoadFromEnv(cwd, home, getenvWithDotEnv(dotenv))
}

func LoadFromEnv(cwd string, home string, getenv func(string) string) (Config, error) {
	addr := strings.TrimSpace(getenv("PATCHPILOT_ADDR"))
	if addr == "" {
		addr = defaultAddr
	}

	allowedRoots, err := splitPathList(getenv("PATCHPILOT_ALLOWED_ROOTS"), cwd)
	if err != nil {
		return Config{}, err
	}

	dataDir := filepath.Join(home, defaultDataDir)
	dataDir, err = filepath.Abs(dataDir)
	if err != nil {
		return Config{}, err
	}

	dbPath := filepath.Join(dataDir, defaultDBName)
	dbPath, err = filepath.Abs(dbPath)
	if err != nil {
		return Config{}, err
	}

	logFormat := strings.ToLower(strings.TrimSpace(getenv("PATCHPILOT_LOG_FORMAT")))
	if logFormat == "" {
		logFormat = "console"
	}

	staticDir := strings.TrimSpace(getenv("PATCHPILOT_STATIC_DIR"))
	if staticDir == "" {
		staticDir = defaultStaticDir
	}
	if !filepath.IsAbs(staticDir) {
		staticDir = filepath.Join(cwd, staticDir)
	}
	staticDir, err = filepath.Abs(staticDir)
	if err != nil {
		return Config{}, err
	}

	return Config{
		Addr:          addr,
		AdminToken:    strings.TrimSpace(getenv("PATCHPILOT_ADMIN_TOKEN")),
		AllowedRoots:  allowedRoots,
		DataDir:       dataDir,
		DBPath:        dbPath,
		HomeDir:       home,
		LightModel:    lightModel(getenv("PATCHPILOT_LIGHT_MODEL")),
		LogFormat:     logFormat,
		OpenAIAPIKey:  strings.TrimSpace(getenv("PATCHPILOT_OPENAI_API_KEY")),
		OpenAIBaseURL: openAIBaseURL(getenv("PATCHPILOT_OPENAI_BASE_URL")),
		StaticDir:     staticDir,
	}, nil
}

func lightModel(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return defaultLightModel
	}
	return value
}

func openAIBaseURL(value string) string {
	value = strings.TrimRight(strings.TrimSpace(value), "/")
	if value == "" {
		return defaultOpenAIBaseURL
	}
	return value
}

func splitPathList(value, fallback string) ([]string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return []string{fallback}, nil
	}

	parts := strings.Split(value, string(os.PathListSeparator))
	roots := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if !filepath.IsAbs(part) {
			part = filepath.Join(fallback, part)
		}
		abs, err := filepath.Abs(part)
		if err != nil {
			return nil, err
		}
		roots = append(roots, abs)
	}
	return roots, nil
}

func getenvWithDotEnv(dotenv map[string]string) func(string) string {
	return func(key string) string {
		if value, ok := os.LookupEnv(key); ok {
			return value
		}
		return dotenv[key]
	}
}

func loadDotEnv(path string) (map[string]string, error) {
	values, err := godotenv.Read(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]string{}, nil
		}
		return nil, err
	}
	return values, nil
}

func UserConfigPath(home string) string {
	return filepath.Join(home, defaultDataDir, "config.json")
}

func LoadUserConfig(home string) (UserConfig, error) {
	var cfg UserConfig
	content, err := os.ReadFile(UserConfigPath(home))
	if err != nil {
		if os.IsNotExist(err) {
			return UserConfig{}, nil
		}
		return UserConfig{}, err
	}
	if err := json.Unmarshal(content, &cfg); err != nil {
		return UserConfig{}, err
	}
	return cfg, nil
}

func SaveUserConfig(home string, cfg UserConfig) error {
	path := UserConfigPath(home)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	content, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	content = append(content, '\n')
	return os.WriteFile(path, content, 0o600)
}
