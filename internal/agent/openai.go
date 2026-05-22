package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"
)

var ErrOpenAIRequestFailed = errors.New("openai request failed")

const maxOpenAIToolRounds = 60

type OpenAIProvider struct {
	apiKey  string
	client  *http.Client
	baseURL string
}

func NewOpenAIProvider(apiKey, baseURL string) *OpenAIProvider {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	return &OpenAIProvider{
		apiKey:  strings.TrimSpace(apiKey),
		client:  &http.Client{Timeout: 2 * time.Minute},
		baseURL: baseURL,
	}
}

func (p *OpenAIProvider) Configured() bool {
	return p != nil && p.apiKey != ""
}

func (p *OpenAIProvider) Generate(ctx context.Context, request ProviderRequest, tools *Tools, stream Stream) (ProviderResult, error) {
	if !p.Configured() {
		return ProviderResult{}, ErrProviderUnavailable
	}
	if stream != nil {
		stream.Delta(ctx, "Calling OpenAI provider.")
	}
	input := []any{
		openAIInputMessage{
			Type: "message",
			Role: "user",
			Content: []openAIInputContent{
				{
					Type: "input_text",
					Text: buildProviderPrompt(request),
				},
			},
		},
	}
	body := openAIResponsesRequest{
		Model: request.Task.Model,
		Input: input,
		Reasoning: openAIReasoning{
			Effort: request.Task.ReasoningEffort,
		},
		Tools: openAIFileTools(tools),
	}

	for range maxOpenAIToolRounds {
		response, err := p.createResponse(ctx, body)
		if err != nil {
			return ProviderResult{}, err
		}
		text := extractResponseText(response)
		if strings.TrimSpace(text) != "" {
			if stream != nil {
				stream.Delta(ctx, "OpenAI response received.")
			}
			return parseProviderResult(text), nil
		}

		toolCalls := extractToolCalls(response)
		if len(toolCalls) == 0 || tools == nil {
			return ProviderResult{}, fmt.Errorf("%w: empty response", ErrOpenAIRequestFailed)
		}
		if stream != nil {
			stream.Delta(ctx, "OpenAI requested workspace file context.")
		}
		for _, call := range toolCalls {
			input = append(input, openAIFunctionCallInput{
				Type:      "function_call",
				CallID:    call.CallID,
				Name:      call.Name,
				Arguments: call.Arguments,
			})
		}
		for _, output := range runOpenAIFileTools(ctx, tools, toolCalls) {
			input = append(input, output)
		}
		body = openAIResponsesRequest{
			Model: request.Task.Model,
			Input: input,
			Reasoning: openAIReasoning{
				Effort: request.Task.ReasoningEffort,
			},
			Tools: openAIFileTools(tools),
		}
	}
	return ProviderResult{}, fmt.Errorf("%w: too many tool calls", ErrOpenAIRequestFailed)
}

func (p *OpenAIProvider) createResponse(ctx context.Context, body openAIResponsesRequest) (openAIResponsesResponse, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return openAIResponsesResponse{}, err
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/responses", bytes.NewReader(payload))
	if err != nil {
		return openAIResponsesResponse{}, err
	}
	httpRequest.Header.Set("Authorization", "Bearer "+p.apiKey)
	httpRequest.Header.Set("Content-Type", "application/json")

	response, err := p.client.Do(httpRequest)
	if err != nil {
		return openAIResponsesResponse{}, err
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return openAIResponsesResponse{}, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return openAIResponsesResponse{}, fmt.Errorf("%w: status %d: %s", ErrOpenAIRequestFailed, response.StatusCode, strings.TrimSpace(string(responseBody)))
	}
	var parsed openAIResponsesResponse
	if err := json.Unmarshal(responseBody, &parsed); err != nil {
		return openAIResponsesResponse{}, err
	}
	return parsed, nil
}

type openAIResponsesRequest struct {
	Model     string          `json:"model"`
	Input     any             `json:"input"`
	Reasoning openAIReasoning `json:"reasoning"`
	Tools     []openAITool    `json:"tools,omitempty"`
}

type openAIReasoning struct {
	Effort string `json:"effort"`
}

type openAITool struct {
	Type        string               `json:"type"`
	Name        string               `json:"name"`
	Description string               `json:"description"`
	Parameters  openAIToolParameters `json:"parameters"`
	Strict      bool                 `json:"strict"`
}

type openAIToolParameters struct {
	Type                 string                        `json:"type"`
	Properties           map[string]openAIToolProperty `json:"properties"`
	Required             []string                      `json:"required"`
	AdditionalProperties bool                          `json:"additionalProperties"`
}

type openAIToolProperty struct {
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
}

type openAIResponsesResponse struct {
	ID         string            `json:"id"`
	OutputText string            `json:"output_text"`
	RawOutput  []json.RawMessage `json:"output"`
}

type openAIOutput struct {
	Type      string          `json:"type"`
	CallID    string          `json:"call_id"`
	Name      string          `json:"name"`
	Arguments string          `json:"arguments"`
	Content   []openAIContent `json:"content"`
}

type openAIInputMessage struct {
	Type    string               `json:"type"`
	Role    string               `json:"role"`
	Content []openAIInputContent `json:"content"`
}

type openAIInputContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type openAIFunctionCallInput struct {
	Type      string `json:"type"`
	CallID    string `json:"call_id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type openAIContent struct {
	Text string `json:"text"`
	Type string `json:"type"`
}

type openAIFunctionCallOutput struct {
	Type   string `json:"type"`
	CallID string `json:"call_id"`
	Output string `json:"output"`
}

type openAIToolCall struct {
	CallID    string
	Name      string
	Arguments string
}

func buildProviderPrompt(request ProviderRequest) string {
	return fmt.Sprintf(`You are PatchPilot's coding agent.

Rules:
- Inspect context only from the server-provided prompt and the available workspace file tools.
- Propose patches only. Do not claim files were modified.
- Do not include secrets.
- Return JSON only with string fields "plan", "summary", and "patch".
- "patch" must be a complete git-apply-compatible unified diff. Use an empty string if no patch is needed.
- Every changed file in "patch" must include diff --git, ---/+++, and @@ hunk headers with line numbers.
- Do not return compact diff summaries or hunks without file headers.
- Use search_files to locate files and read_file to inspect file contents before producing patches when needed.

Workspace git status:
%s

User prompt:
%s`, request.GitStatus, request.Task.Prompt)
}

func openAIFileTools(tools *Tools) []openAITool {
	if tools == nil {
		return nil
	}
	return []openAITool{
		{
			Type:        "function",
			Name:        "search_files",
			Description: "Search workspace text files by filename or content. Returns workspace-relative paths and match previews.",
			Parameters: openAIToolParameters{
				Type: "object",
				Properties: map[string]openAIToolProperty{
					"query": {Type: "string", Description: "Filename or content query."},
				},
				Required:             []string{"query"},
				AdditionalProperties: false,
			},
			Strict: true,
		},
		{
			Type:        "function",
			Name:        "read_file",
			Description: "Read one text file by workspace-relative path. Secret, ignored, oversized, binary, and escaping paths are rejected.",
			Parameters: openAIToolParameters{
				Type: "object",
				Properties: map[string]openAIToolProperty{
					"path": {Type: "string", Description: "Workspace-relative file path."},
				},
				Required:             []string{"path"},
				AdditionalProperties: false,
			},
			Strict: true,
		},
	}
}

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

func runOpenAIFileTools(ctx context.Context, tools *Tools, calls []openAIToolCall) []openAIFunctionCallOutput {
	outputs := make([]openAIFunctionCallOutput, 0, len(calls))
	for _, call := range calls {
		outputs = append(outputs, openAIFunctionCallOutput{
			Type:   "function_call_output",
			CallID: call.CallID,
			Output: executeOpenAIFileTool(ctx, tools, call),
		})
	}
	return outputs
}

func executeOpenAIFileTool(ctx context.Context, tools *Tools, call openAIToolCall) string {
	switch call.Name {
	case "search_files":
		var args struct {
			Query string `json:"query"`
		}
		if err := json.Unmarshal([]byte(call.Arguments), &args); err != nil {
			return openAIToolError(err)
		}
		results, err := tools.SearchFiles(ctx, args.Query)
		if err != nil {
			return openAIToolError(err)
		}
		if len(results) > 25 {
			results = results[:25]
		}
		return openAIToolJSON(map[string]any{"results": results})
	case "read_file":
		var args struct {
			Path string `json:"path"`
		}
		if err := json.Unmarshal([]byte(call.Arguments), &args); err != nil {
			return openAIToolError(err)
		}
		file, err := tools.ReadFile("", args.Path)
		if err != nil {
			return openAIToolError(err)
		}
		return openAIToolJSON(file)
	default:
		return openAIToolError(fmt.Errorf("unknown tool: %s", call.Name))
	}
}

func openAIToolError(err error) string {
	return openAIToolJSON(map[string]string{"error": err.Error()})
}

func openAIToolJSON(value any) string {
	payload, err := json.Marshal(value)
	if err != nil {
		return `{"error":"failed to encode tool output"}`
	}
	return string(payload)
}

func parseProviderResult(text string) ProviderResult {
	text = strings.TrimSpace(text)
	var parsed struct {
		Plan    string `json:"plan"`
		Summary string `json:"summary"`
		Patch   string `json:"patch"`
	}
	if err := json.Unmarshal([]byte(text), &parsed); err == nil {
		return ProviderResult{
			Plan:    parsed.Plan,
			Summary: parsed.Summary,
			Patch:   normalizeProviderPatch(parsed.Patch),
		}
	}
	return ProviderResult{
		Plan:    "Review the request and produce a patch-first response.",
		Summary: text,
		Patch:   normalizeProviderPatch(text),
	}
}

func normalizeProviderPatch(patch string) string {
	patch = strings.TrimSpace(strings.TrimPrefix(strings.TrimSuffix(strings.TrimSpace(patch), "```"), "```diff"))
	if patch == "" || strings.Contains(patch, "diff --git ") {
		return patch
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
