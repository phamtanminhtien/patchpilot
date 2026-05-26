package agent

func openAITools() []openAITool {
	return []openAITool{
		{
			Type:        "function",
			Name:        "list_files",
			Description: "List files/directories under a workspace-relative path.",
			Parameters: openAIToolParameters{
				Type: "object",
				Properties: map[string]openAIToolProperty{
					"path": {Type: "string", Description: "Workspace-relative directory path. Use empty string for root."},
				},
				Required:             []string{"path"},
				AdditionalProperties: false,
			},
			Strict: true,
		},
		{
			Type:        "function",
			Name:        "search_files",
			Description: "Search workspace text files by filename or content. Returns workspace-relative paths and match previews.",
			Parameters: openAIToolParameters{
				Type: "object",
				Properties: map[string]openAIToolProperty{
					"query": {Type: "string", Description: "Filename or content query."},
					"path":  {Type: "string", Description: "Optional workspace-relative file or directory path to search. Empty or omitted searches the workspace root."},
				},
				Required:             []string{"query"},
				AdditionalProperties: false,
			},
			Strict: true,
		},
		{
			Type:        "function",
			Name:        "run_command",
			Description: "Run a workspace command. Non-allowlisted commands require user approval.",
			Parameters: openAIToolParameters{
				Type: "object",
				Properties: map[string]openAIToolProperty{
					"command": {Type: "string", Description: "Command to run from workspace root."},
				},
				Required:             []string{"command"},
				AdditionalProperties: false,
			},
			Strict: true,
		},
		{
			Type:        "function",
			Name:        "apply_patch",
			Description: "Request user approval to apply a complete unified diff to the workspace.",
			Parameters: openAIToolParameters{
				Type: "object",
				Properties: map[string]openAIToolProperty{
					"summary": {Type: "string", Description: "Short summary of the patch."},
					"diff":    {Type: "string", Description: "Complete git-apply-compatible unified diff."},
				},
				Required:             []string{"summary", "diff"},
				AdditionalProperties: false,
			},
			Strict: true,
		},
	}
}

func emptyToolParameters() openAIToolParameters {
	return openAIToolParameters{
		Type:                 "object",
		Properties:           map[string]openAIToolProperty{},
		Required:             []string{},
		AdditionalProperties: false,
	}
}
