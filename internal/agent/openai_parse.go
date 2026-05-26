package agent

import (
	"encoding/json"
	"path/filepath"
	"strings"
)

func extractResponseText(response openAIResponsesResponse) string {
	if response.OutputText != "" {
		return response.OutputText
	}
	var builder strings.Builder
	for _, rawOutput := range response.RawOutput {
		var output openAIOutput
		if err := json.Unmarshal(rawOutput, &output); err != nil {
			continue
		}
		for _, content := range output.Content {
			if content.Text != "" {
				builder.WriteString(content.Text)
			}
		}
	}
	return builder.String()
}

func extractToolCalls(response openAIResponsesResponse) []openAIToolCall {
	calls := make([]openAIToolCall, 0)
	for _, rawOutput := range response.RawOutput {
		var output openAIOutput
		if err := json.Unmarshal(rawOutput, &output); err != nil {
			continue
		}
		if output.Type != "function_call" {
			continue
		}
		calls = append(calls, openAIToolCall{
			CallID:    output.CallID,
			Name:      output.Name,
			Arguments: output.Arguments,
		})
	}
	return calls
}

func parseProviderText(text string) ProviderResult {
	text = strings.TrimSpace(text)
	var parsed struct {
		Text    string `json:"text"`
		Summary string `json:"summary"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal([]byte(text), &parsed); err == nil {
		for _, candidate := range []string{parsed.Text, parsed.Summary, parsed.Message} {
			if strings.TrimSpace(candidate) != "" {
				return ProviderResult{Text: strings.TrimSpace(candidate), Done: true}
			}
		}
	}
	return ProviderResult{Text: text, Done: true}
}

func normalizeProviderPatch(patch string) string {
	patch = strings.TrimSpace(strings.TrimPrefix(strings.TrimSuffix(strings.TrimSpace(patch), "```"), "```diff"))
	if patch == "" {
		return patch
	}
	if strings.Contains(patch, "diff --git ") {
		return strings.TrimRight(patch, "\r\n") + "\n"
	}

	lines := strings.Split(patch, "\n")
	var builder strings.Builder
	currentPath := ""
	inHunk := false
	wroteFile := false
	for _, line := range lines {
		line = strings.TrimRight(line, "\r")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if isCompactPatchPath(trimmed) {
			if inHunk && wroteFile {
				builder.WriteString("\n")
			}
			currentPath = trimmed
			inHunk = false
			wroteFile = false
			continue
		}
		diffLine := compactDiffLine(line)
		if strings.HasPrefix(diffLine, "@@ ") && currentPath != "" {
			if !wroteFile {
				builder.WriteString("diff --git a/")
				builder.WriteString(currentPath)
				builder.WriteString(" b/")
				builder.WriteString(currentPath)
				builder.WriteString("\n--- a/")
				builder.WriteString(currentPath)
				builder.WriteString("\n+++ b/")
				builder.WriteString(currentPath)
				builder.WriteString("\n")
				wroteFile = true
			}
			inHunk = true
		}
		if !inHunk {
			continue
		}
		if isCompactDiffSummary(trimmed) {
			inHunk = false
			continue
		}
		if !isUnifiedDiffLine(diffLine) {
			inHunk = false
			continue
		}
		builder.WriteString(diffLine)
		builder.WriteString("\n")
	}
	normalized := builder.String()
	if strings.TrimSpace(normalized) == "" {
		return ""
	}
	return strings.TrimRight(normalized, "\r\n") + "\n"
}

func isCompactPatchPath(line string) bool {
	if strings.ContainsAny(line, " \t|") || strings.HasPrefix(line, "diff --git ") {
		return false
	}
	base := filepath.Base(line)
	return strings.Contains(base, ".") && base != "." && base != ".."
}

func isUnifiedDiffLine(line string) bool {
	if strings.HasPrefix(line, "@@ ") || strings.HasPrefix(line, "+") || strings.HasPrefix(line, "-") || strings.HasPrefix(line, " ") {
		return true
	}
	return line == `\ No newline at end of file`
}

func isCompactDiffSummary(line string) bool {
	fields := strings.Fields(line)
	return len(fields) == 2 && strings.HasPrefix(fields[0], "+") && strings.HasPrefix(fields[1], "-")
}

func compactDiffLine(line string) string {
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "@@ ") || strings.HasPrefix(trimmed, "+") || strings.HasPrefix(trimmed, "-") {
		return trimmed
	}
	if strings.HasPrefix(line, "  ") {
		return strings.TrimPrefix(line, "  ")
	}
	if strings.HasPrefix(line, " ") {
		return strings.TrimPrefix(line, " ")
	}
	return line
}
