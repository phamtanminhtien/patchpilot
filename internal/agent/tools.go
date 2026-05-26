package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/phamtanminhtien/patchpilot/internal/database"
	"github.com/phamtanminhtien/patchpilot/internal/filestore"
	"github.com/phamtanminhtien/patchpilot/internal/gitrepo"
	"github.com/phamtanminhtien/patchpilot/internal/runner"
)

func (m *Manager) prepareToolRequest(ctx context.Context, workspaceRoot string, request ToolRequest) (bool, string, string) {
	switch request.Name {
	case "apply_patch":
		var args struct {
			Diff string `json:"diff"`
		}
		if err := json.Unmarshal([]byte(request.Arguments), &args); err != nil {
			return false, ToolStatusFailed, openToolError(err)
		}
		if err := m.git.CheckPatch(ctx, workspaceRoot, normalizeProviderPatch(args.Diff)); err != nil {
			return false, ToolStatusFailed, openToolError(fmt.Errorf("patch failed git apply check: %w", err))
		}
		if policy, ok := staticToolPolicy(request.Name); ok && policy == toolRequiresApproval {
			return true, ToolStatusWaitingApproval, "{}"
		}
		return false, ToolStatusPending, "{}"
	case "run_command":
		var args struct {
			Command string `json:"command"`
		}
		if err := json.Unmarshal([]byte(request.Arguments), &args); err != nil {
			return false, ToolStatusFailed, openToolError(err)
		}
		decision, err := runner.Classify(args.Command)
		if err != nil {
			return false, ToolStatusFailed, openToolError(err)
		}
		if decision.Level == runner.SafetyNeedsConfirmation {
			return true, ToolStatusWaitingApproval, "{}"
		}
		return false, ToolStatusPending, "{}"
	default:
		if strings.HasPrefix(request.Name, "mcp.") {
			return true, ToolStatusWaitingApproval, "{}"
		}
		if policy, ok := staticToolPolicy(request.Name); ok && policy == toolRequiresApproval {
			return true, ToolStatusWaitingApproval, "{}"
		}
		return false, ToolStatusPending, "{}"
	}
}

func (m *Manager) executeTool(ctx context.Context, runtime *runRuntime, record database.AgentToolCallRecord) (string, error) {
	workspaceRoot := runtime.workspaceRoot
	switch record.Name {
	case "list_files":
		var args struct {
			Path string `json:"path"`
		}
		if err := json.Unmarshal([]byte(record.InputJSON), &args); err != nil {
			return "", err
		}
		entries, err := m.files.List(workspaceRoot, args.Path)
		if err != nil {
			return "", err
		}
		return openToolJSON(map[string]any{"entries": entries}), nil
	case "search_files":
		var args struct {
			Query string `json:"query"`
			Path  string `json:"path"`
		}
		if err := json.Unmarshal([]byte(record.InputJSON), &args); err != nil {
			return "", err
		}
		results, err := m.files.SearchWithOptions(workspaceRoot, args.Query, filestore.SearchOptions{Path: args.Path})
		if err != nil {
			return "", err
		}
		if len(results) > 25 {
			results = results[:25]
		}
		return openToolJSON(map[string]any{"results": results}), nil
	case "run_command":
		return m.executeCommandTool(ctx, runtime, record)
	case "apply_patch":
		var args struct {
			Diff    string `json:"diff"`
			Summary string `json:"summary"`
		}
		if err := json.Unmarshal([]byte(record.InputJSON), &args); err != nil {
			return "", err
		}
		diff := normalizeProviderPatch(args.Diff)
		if err := m.git.ApplyPatch(ctx, workspaceRoot, diff, gitrepo.ApplyForward); err != nil {
			return "", err
		}
		return openToolJSON(map[string]string{"status": "applied", "summary": args.Summary}), nil
	default:
		if strings.HasPrefix(record.Name, "mcp.") {
			return "", fmt.Errorf("MCP tool execution is not connected for %s", record.Name)
		}
		return "", fmt.Errorf("unknown tool: %s", record.Name)
	}
}

func toolSourceMetadata(name, inputJSON string, requiresApproval bool) (string, *string, string) {
	if strings.HasPrefix(name, "mcp.") {
		ref := strings.TrimPrefix(name, "mcp.")
		return "mcp", &ref, "MCP tools require approval unless configured read-only and safe."
	}
	if strings.HasPrefix(name, "skill.") {
		ref := strings.TrimPrefix(name, "skill.")
		return "skill", &ref, "Skill tool call."
	}
	if requiresApproval {
		return "builtin", nil, "Built-in tool requires approval."
	}
	return "builtin", nil, ""
}

func (m *Manager) executeCommandTool(ctx context.Context, runtime *runRuntime, record database.AgentToolCallRecord) (string, error) {
	var args struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal([]byte(record.InputJSON), &args); err != nil {
		return "", err
	}
	decision, err := runner.Classify(args.Command)
	if err != nil {
		return "", err
	}
	if decision.Level == runner.SafetyBlocked {
		return openToolJSON(map[string]any{"status": "blocked", "decision": decision}), nil
	}
	if decision.Level == runner.SafetyNeedsConfirmation && record.Decision == nil {
		return "", fmt.Errorf("command requires approval: %s", decision.Reason)
	}
	done := make(chan runner.FinishResult, 1)
	var output strings.Builder
	err = m.runner.Start(runner.RunSpec{
		ID:          record.ID,
		WorkspaceID: record.WorkspaceID,
		Command:     args.Command,
		Cwd:         runtime.workspaceRoot,
	}, runner.Hooks{
		OnOutput: func(_, chunk string) {
			if output.Len() < 1024*1024 {
				output.WriteString(chunk)
			}
		},
		OnFinished: func(result runner.FinishResult) {
			done <- result
		},
	})
	if err != nil {
		return "", err
	}
	m.addRuntimeCommand(runtime, record.ID)
	defer m.removeRuntimeCommand(runtime, record.ID)
	var result runner.FinishResult
	select {
	case <-ctx.Done():
		m.runner.Stop(record.ID)
		return "", context.Canceled
	case result = <-done:
	}
	return openToolJSON(map[string]any{"status": result.Status, "exitCode": result.ExitCode, "output": output.String()}), nil
}

func openToolError(err error) string {
	return openToolJSON(map[string]string{"error": err.Error()})
}

func openToolJSON(value any) string {
	payload, err := json.Marshal(value)
	if err != nil {
		return `{"error":"failed to encode tool output"}`
	}
	return string(payload)
}
