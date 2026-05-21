package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFromEnvUsesDefaults(t *testing.T) {
	cwd := t.TempDir()
	home := t.TempDir()

	cfg, err := LoadFromEnv(cwd, home, emptyEnv)
	if err != nil {
		t.Fatalf("LoadFromEnv returned error: %v", err)
	}
	if cfg.Addr != defaultAddr {
		t.Fatalf("expected default addr, got %q", cfg.Addr)
	}
	if len(cfg.AllowedRoots) != 1 || cfg.AllowedRoots[0] != cwd {
		t.Fatalf("expected cwd allowed root, got %+v", cfg.AllowedRoots)
	}
	if cfg.DataDir != filepath.Join(home, defaultDataDir) {
		t.Fatalf("unexpected data dir: %q", cfg.DataDir)
	}
	if cfg.DBPath != filepath.Join(home, defaultDataDir, defaultDBName) {
		t.Fatalf("unexpected db path: %q", cfg.DBPath)
	}
	if cfg.StaticDir != "" {
		t.Fatalf("expected no static dir, got %q", cfg.StaticDir)
	}
	if cfg.LogFormat != "console" {
		t.Fatalf("expected default console log format, got %q", cfg.LogFormat)
	}
}

func TestLoadFromEnvUsesOverrides(t *testing.T) {
	cwd := t.TempDir()
	home := t.TempDir()
	firstRoot := filepath.Join(cwd, "one")
	secondRoot := filepath.Join(cwd, "two")
	dbPath := filepath.Join(cwd, "state", "app.db")

	cfg, err := LoadFromEnv(cwd, home, func(key string) string {
		switch key {
		case "PATCHPILOT_ADDR":
			return "0.0.0.0:9090"
		case "PATCHPILOT_ALLOWED_ROOTS":
			return firstRoot + string(os.PathListSeparator) + secondRoot
		case "PATCHPILOT_DB_PATH":
			return dbPath
		case "PATCHPILOT_STATIC_DIR":
			return "web/dist"
		case "PATCHPILOT_LOG_FORMAT":
			return "JSON"
		default:
			return ""
		}
	})
	if err != nil {
		t.Fatalf("LoadFromEnv returned error: %v", err)
	}
	if cfg.Addr != "0.0.0.0:9090" {
		t.Fatalf("unexpected addr: %q", cfg.Addr)
	}
	if len(cfg.AllowedRoots) != 2 || cfg.AllowedRoots[0] != firstRoot || cfg.AllowedRoots[1] != secondRoot {
		t.Fatalf("unexpected allowed roots: %+v", cfg.AllowedRoots)
	}
	if cfg.DBPath != dbPath {
		t.Fatalf("unexpected db path: %q", cfg.DBPath)
	}
	if cfg.StaticDir != filepath.Join(cwd, "web", "dist") {
		t.Fatalf("unexpected static dir: %q", cfg.StaticDir)
	}
	if cfg.LogFormat != "json" {
		t.Fatalf("unexpected log format: %q", cfg.LogFormat)
	}
}

func TestLoadUsesDotEnvWhenEnvironmentIsUnset(t *testing.T) {
	cwd := t.TempDir()
	home := t.TempDir()
	if err := os.WriteFile(filepath.Join(cwd, ".env"), []byte("PATCHPILOT_ADDR=127.0.0.1:9091\nPATCHPILOT_LOG_FORMAT=json\n"), 0o600); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	dotenv, err := loadDotEnv(filepath.Join(cwd, ".env"))
	if err != nil {
		t.Fatalf("loadDotEnv returned error: %v", err)
	}
	cfg, err := LoadFromEnv(cwd, home, getenvWithDotEnv(dotenv))
	if err != nil {
		t.Fatalf("LoadFromEnv returned error: %v", err)
	}
	if cfg.Addr != "127.0.0.1:9091" {
		t.Fatalf("expected addr from .env, got %q", cfg.Addr)
	}
	if cfg.LogFormat != "json" {
		t.Fatalf("expected log format from .env, got %q", cfg.LogFormat)
	}
}

func TestEnvironmentOverridesDotEnv(t *testing.T) {
	t.Setenv("PATCHPILOT_ADDR", "127.0.0.1:7070")
	getenv := getenvWithDotEnv(map[string]string{
		"PATCHPILOT_ADDR": "127.0.0.1:6060",
	})

	if value := getenv("PATCHPILOT_ADDR"); value != "127.0.0.1:7070" {
		t.Fatalf("expected OS env to win, got %q", value)
	}
}

func TestLoadDotEnvUsesGodotenvSyntax(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	content := "PATCHPILOT_ADDR=127.0.0.1:8080\nexport PATCHPILOT_LOG_FORMAT=\"json\"\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	values, err := loadDotEnv(path)
	if err != nil {
		t.Fatalf("loadDotEnv returned error: %v", err)
	}
	if values["PATCHPILOT_ADDR"] != "127.0.0.1:8080" {
		t.Fatalf("unexpected addr: %q", values["PATCHPILOT_ADDR"])
	}
	if values["PATCHPILOT_LOG_FORMAT"] != "json" {
		t.Fatalf("unexpected log format: %q", values["PATCHPILOT_LOG_FORMAT"])
	}
}

func emptyEnv(string) string {
	return ""
}
