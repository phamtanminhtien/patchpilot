package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/phamtanminhtien/patchpilot/internal/api"
	fileapi "github.com/phamtanminhtien/patchpilot/internal/files"
	gitapi "github.com/phamtanminhtien/patchpilot/internal/git"
	"github.com/phamtanminhtien/patchpilot/internal/runner"
	"github.com/phamtanminhtien/patchpilot/internal/workspace"
)

func main() {
	allowedRoots, err := configuredAllowedRoots()
	if err != nil {
		log.Fatalf("configure allowed roots: %v", err)
	}
	workspaces, err := workspace.NewManager(allowedRoots)
	if err != nil {
		log.Fatalf("create workspace manager: %v", err)
	}

	server := api.NewServer(workspaces, fileapi.NewService(), gitapi.NewClient(), runner.NewRunner())
	httpServer := &http.Server{
		Addr:              addr(),
		Handler:           server.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("patchpilot listening on %s", httpServer.Addr)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func addr() string {
	if value := strings.TrimSpace(os.Getenv("PATCHPILOT_ADDR")); value != "" {
		return value
	}
	return "127.0.0.1:8080"
}

func configuredAllowedRoots() ([]string, error) {
	value := strings.TrimSpace(os.Getenv("PATCHPILOT_ALLOWED_ROOTS"))
	if value == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		return []string{cwd}, nil
	}

	parts := strings.Split(value, string(os.PathListSeparator))
	roots := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		abs, err := filepath.Abs(part)
		if err != nil {
			return nil, err
		}
		roots = append(roots, abs)
	}
	return roots, nil
}
