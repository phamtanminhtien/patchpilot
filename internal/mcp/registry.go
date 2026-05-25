package mcp

import (
	"os"
	"sort"
	"strings"

	"github.com/phamtanminhtien/patchpilot/internal/config"
)

type Warning struct {
	ServerID string `json:"serverId,omitempty"`
	Message  string `json:"message"`
}

type Server struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Transport      string    `json:"transport"`
	Disabled       bool      `json:"disabled"`
	Status         string    `json:"status"`
	LastError      string    `json:"lastError,omitempty"`
	ApprovalPolicy string    `json:"approvalPolicy"`
	Warnings       []Warning `json:"warnings,omitempty"`
}

type Tool struct {
	ID             string `json:"id"`
	ServerID       string `json:"serverId"`
	Name           string `json:"name"`
	ReadOnlyHint   bool   `json:"readOnlyHint"`
	ApprovalPolicy string `json:"approvalPolicy"`
}

type Registry struct {
	Servers  []Server  `json:"servers"`
	Tools    []Tool    `json:"tools"`
	Warnings []Warning `json:"warnings,omitempty"`
}

func Discover(home string) Registry {
	cfg, err := config.LoadUserConfig(home)
	registry := Registry{}
	if err != nil {
		registry.Warnings = append(registry.Warnings, Warning{Message: "Local config could not be read."})
		return registry
	}
	ids := make([]string, 0, len(cfg.MCPServers))
	for id := range cfg.MCPServers {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		serverCfg := cfg.MCPServers[id]
		transport := strings.TrimSpace(serverCfg.Transport)
		if transport == "" {
			if strings.TrimSpace(serverCfg.URL) != "" {
				transport = "http"
			} else {
				transport = "stdio"
			}
		}
		policy := strings.TrimSpace(serverCfg.ApprovalPolicy)
		if policy == "" {
			policy = "always"
		}
		server := Server{
			ID:             id,
			Name:           id,
			Transport:      transport,
			Disabled:       serverCfg.Disabled,
			Status:         "configured",
			ApprovalPolicy: policy,
		}
		if server.Disabled {
			server.Status = "disabled"
		}
		for name, value := range serverCfg.Env {
			if unresolvedEnvPlaceholder(value) {
				server.Warnings = append(server.Warnings, Warning{ServerID: id, Message: "Environment placeholder " + name + " is unresolved."})
			}
		}
		if transport == "http" && strings.TrimSpace(serverCfg.URL) == "" {
			server.LastError = "HTTP MCP server URL is required."
			server.Status = "error"
		}
		if transport == "stdio" && strings.TrimSpace(serverCfg.Command) == "" {
			server.LastError = "Stdio MCP server command is required."
			server.Status = "error"
		}
		registry.Servers = append(registry.Servers, server)
		registry.Warnings = append(registry.Warnings, server.Warnings...)
	}
	return registry
}

func unresolvedEnvPlaceholder(value string) bool {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "${") && strings.HasSuffix(value, "}") {
		name := strings.TrimSuffix(strings.TrimPrefix(value, "${"), "}")
		_, ok := os.LookupEnv(name)
		return !ok
	}
	return false
}
