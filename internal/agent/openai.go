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

func (p *OpenAIProvider) Generate(ctx context.Context, request ProviderRequest, stream Stream) (ProviderResult, error) {
	if !p.Configured() {
		return ProviderResult{}, ErrProviderUnavailable
	}
	if stream != nil {
		stream.Delta(ctx, "Calling OpenAI provider.")
	}
	body := openAIResponsesRequest{
		Model:        request.Run.Model,
		Instructions: providerInstructions(),
		Input:        buildOpenAIInput(request),
		Reasoning: openAIReasoning{
			Effort: request.Run.ReasoningEffort,
		},
		Tools: openAITools(),
	}
	response, err := p.createResponse(ctx, body)
	if err != nil {
		return ProviderResult{}, err
	}
	text := strings.TrimSpace(extractResponseText(response))
	toolCalls := extractToolCalls(response)
	if len(toolCalls) > 0 {
		if stream != nil {
			stream.Delta(ctx, "OpenAI requested workspace tools.")
		}
		requests := make([]ToolRequest, 0, len(toolCalls))
		for _, call := range toolCalls {
			requests = append(requests, ToolRequest{
				CallID:    call.CallID,
				Name:      call.Name,
				Arguments: call.Arguments,
			})
		}
		return ProviderResult{Text: text, ToolCalls: requests}, nil
	}
	if text == "" {
		return ProviderResult{}, fmt.Errorf("%w: empty response", ErrOpenAIRequestFailed)
	}
	if stream != nil {
		stream.Delta(ctx, "OpenAI response received.")
	}
	return parseProviderText(text), nil
}

func (p *OpenAIProvider) Summarize(ctx context.Context, request SummaryRequest) (string, error) {
	if !p.Configured() {
		return "", ErrProviderUnavailable
	}
	body := openAIResponsesRequest{
		Model:        request.Run.Model,
		Instructions: summaryInstructions(),
		Input:        buildSummaryInput(request),
		Reasoning: openAIReasoning{
			Effort: request.Run.ReasoningEffort,
		},
	}
	response, err := p.createResponse(ctx, body)
	if err != nil {
		return "", err
	}
	text := strings.TrimSpace(extractResponseText(response))
	if text == "" {
		return "", fmt.Errorf("%w: empty summary response", ErrOpenAIRequestFailed)
	}
	return text, nil
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
	Model        string          `json:"model"`
	Instructions string          `json:"instructions,omitempty"`
	Input        any             `json:"input"`
	Reasoning    openAIReasoning `json:"reasoning"`
	Tools        []openAITool    `json:"tools,omitempty"`
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

func buildOpenAIInput(request ProviderRequest) []any {
	input := make([]any, 0, 1+len(request.ConversationContext)+len(request.History))
	if strings.TrimSpace(request.ContextSummary) != "" {
		input = append(input, openAIMessage("user", "Conversation summary:\n"+strings.TrimSpace(request.ContextSummary)))
	}
	for _, message := range request.ConversationContext {
		if strings.TrimSpace(message.Content) == "" {
			continue
		}
		input = append(input, openAIMessage(message.Role, message.Content))
	}
	if len(input) == 0 {
		input = append(input, openAIMessage("user", strings.TrimSpace(request.Prompt)))
	}
	for _, item := range request.History {
		switch item.Type {
		case "tool_call":
			input = append(input, openAIFunctionCallInput{
				Type:      "function_call",
				CallID:    item.ToolCall.CallID,
				Name:      item.ToolCall.Name,
				Arguments: item.ToolCall.Arguments,
			})
		case "tool_result":
			input = append(input, openAIFunctionCallOutput{
				Type:   "function_call_output",
				CallID: item.ToolResult.CallID,
				Output: item.ToolResult.Output,
			})
		case "text":
			if strings.TrimSpace(item.Text) != "" {
				input = append(input, openAIInputMessage{
					Type: "message",
					Role: "assistant",
					Content: []openAIInputContent{
						{Type: "output_text", Text: item.Text},
					},
				})
			}
		}
	}
	return input
}

func openAIMessage(role, text string) openAIInputMessage {
	if role != "assistant" {
		role = "user"
	}
	contentType := "input_text"
	if role == "assistant" {
		contentType = "output_text"
	}
	return openAIInputMessage{
		Type: "message",
		Role: role,
		Content: []openAIInputContent{
			{Type: contentType, Text: strings.TrimSpace(text)},
		},
	}
}

func buildSummaryInput(request SummaryRequest) []any {
	input := make([]any, 0, 1+len(request.Messages))
	if strings.TrimSpace(request.ExistingSummary) != "" {
		input = append(input, openAIMessage("user", "Existing conversation summary:\n"+strings.TrimSpace(request.ExistingSummary)))
	}
	for _, message := range request.Messages {
		input = append(input, openAIMessage(message.Role, message.Content))
	}
	return input
}

func buildProviderPrompt(request ProviderRequest) string {
	return providerInstructions() + "\n\nCurrent user prompt:\n" + strings.TrimSpace(request.Prompt)
}

func providerInstructions() string {
	return `You are PatchPilot's coding agent.

Rules:
- Inspect context only from the server-provided conversation context and the available workspace tools.
- Return assistant text when useful.
- Use tools for workspace reads, git inspection, commands, and patches.
- When calling tools, include concise output_text in the same response that tells the user what you are checking or changing so the user sees progress while tool calls are pending.
- Write assistant text, including output_text sent with tool calls, in the same language as the user's prompt unless the user explicitly asks for a different language.
- If the user prompt asks for a change or investigation, do not answer with readiness, greetings, or "what would you like me to do" questions.
- For change or investigation prompts, first call at least one workspace inspection tool such as search_files, list_files, read_file, git_status, or git_diff unless the answer can be completed entirely from prior tool results.
- Ask a clarifying question only when the user prompt is genuinely missing the target or desired outcome.
- Do not claim files were modified unless the apply_patch tool result says the patch was applied.
- Do not include secrets.
- apply_patch input must include "summary" and a complete git-apply-compatible unified "diff".
- Every changed file in a patch diff must include diff --git, ---/+++, and @@ hunk headers with line numbers.
- Tool calls in one response may run as a batch. Approval-required tools are decided by the user before the batch runs.`
}

func summaryInstructions() string {
	return `Summarize older PatchPilot conversation context for a coding agent.

Rules:
- Preserve user intent, decisions, constraints, referenced files, commands, applied or rejected changes, and open tasks.
- Keep the summary concise and factual.
- Do not invent details.
- Do not include secrets, raw environment values, or host paths outside the workspace.`
}

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
		{
			Type:        "function",
			Name:        "git_status",
			Description: "Inspect current workspace git status.",
			Parameters:  emptyToolParameters(),
			Strict:      true,
		},
		{
			Type:        "function",
			Name:        "git_diff",
			Description: "Inspect the current git diff for the workspace or one workspace-relative path.",
			Parameters: openAIToolParameters{
				Type: "object",
				Properties: map[string]openAIToolProperty{
					"path": {Type: "string", Description: "Workspace-relative path. Use empty string for full diff."},
				},
				Required:             []string{"path"},
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
