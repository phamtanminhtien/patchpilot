package config

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
)

const (
	defaultAddr          = "127.0.0.1:8080"
	defaultDataDir       = ".patchpilot"
	defaultDBName        = "patchpilot.db"
	defaultOpenAIBaseURL = "https://api.openai.com/v1"
)

type Config struct {
	Addr          string
	AdminToken    string
	AllowedRoots  []string
	DataDir       string
	DBPath        string
	LogFormat     string
	OpenAIAPIKey  string
	OpenAIBaseURL string
	StaticDir     string
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

	dataDir := strings.TrimSpace(getenv("PATCHPILOT_DATA_DIR"))
	if dataDir == "" {
		dataDir = filepath.Join(home, defaultDataDir)
	} else if !filepath.IsAbs(dataDir) {
		dataDir = filepath.Join(cwd, dataDir)
	}
	dataDir, err = filepath.Abs(dataDir)
	if err != nil {
		return Config{}, err
	}

	dbPath := strings.TrimSpace(getenv("PATCHPILOT_DB_PATH"))
	if dbPath == "" {
		dbPath = filepath.Join(dataDir, defaultDBName)
	} else if !filepath.IsAbs(dbPath) {
		dbPath = filepath.Join(cwd, dbPath)
	}
	dbPath, err = filepath.Abs(dbPath)
	if err != nil {
		return Config{}, err
	}

	logFormat := strings.ToLower(strings.TrimSpace(getenv("PATCHPILOT_LOG_FORMAT")))
	if logFormat == "" {
		logFormat = "console"
	}

	staticDir := strings.TrimSpace(getenv("PATCHPILOT_STATIC_DIR"))
	if staticDir != "" {
		if !filepath.IsAbs(staticDir) {
			staticDir = filepath.Join(cwd, staticDir)
		}
		staticDir, err = filepath.Abs(staticDir)
		if err != nil {
			return Config{}, err
		}
	}

	return Config{
		Addr:          addr,
		AdminToken:    strings.TrimSpace(getenv("PATCHPILOT_ADMIN_TOKEN")),
		AllowedRoots:  allowedRoots,
		DataDir:       dataDir,
		DBPath:        dbPath,
		LogFormat:     logFormat,
		OpenAIAPIKey:  strings.TrimSpace(getenv("PATCHPILOT_OPENAI_API_KEY")),
		OpenAIBaseURL: openAIBaseURL(getenv("PATCHPILOT_OPENAI_BASE_URL")),
		StaticDir:     staticDir,
	}, nil
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
