package agent

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/phamtanminhtien/patchpilot/internal/skills"
)

var ErrOpenAIRequestFailed = errors.New("openai request failed")

const (
	openAIXMLTagPatchPilotAgent    openAIXMLTag = "patchpilot_agent"
	openAIXMLTagEnvironmentContext openAIXMLTag = "environment_context"
	openAIXMLTagCWD                openAIXMLTag = "cwd"
	openAIXMLTagShell              openAIXMLTag = "shell"
	openAIXMLTagCurrentDate        openAIXMLTag = "current_date"
	openAIXMLTagTimezone           openAIXMLTag = "timezone"
	openAIXMLTagRepoInstructions   openAIXMLTag = "repo_instructions"
	openAIXMLTagSkillsInstructions openAIXMLTag = "skills_instructions"
	openAIXMLTagContextWarnings    openAIXMLTag = "context_warnings"
	openAIXMLTagSummaryTask        openAIXMLTag = "summary_task"
	openAIXMLTagSummaryRules       openAIXMLTag = "summary_rules"
)

const maxGeneratedTitleLength = 80

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
		client:  &http.Client{Timeout: 20 * time.Minute},
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
	body := openAIResponsesRequest{
		Model: request.Run.Model,
		Input: buildOpenAIInput(request),
		Reasoning: &openAIReasoning{
			Effort: request.Run.ReasoningEffort,
		},
		Tools:  openAITools(),
		Stream: true,
	}
	result, err := p.createStreamingResponse(ctx, body, stream)
	if err != nil {
		return ProviderResult{}, err
	}
	text := strings.TrimSpace(result.Text)
	if len(result.ToolCalls) > 0 {
		requests := make([]ToolRequest, 0, len(result.ToolCalls))
		for _, call := range result.ToolCalls {
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
	return parseProviderText(text), nil
}

func (p *OpenAIProvider) Summarize(ctx context.Context, request SummaryRequest) (string, error) {
	if !p.Configured() {
		return "", ErrProviderUnavailable
	}
	body := openAIResponsesRequest{
		Model: request.Run.Model,
		Input: buildSummaryInput(request),
		Reasoning: &openAIReasoning{
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

func (p *OpenAIProvider) GenerateTitle(ctx context.Context, prompt, model string) (string, error) {
	if !p.Configured() {
		return "", ErrProviderUnavailable
	}
	prompt = strings.TrimSpace(prompt)
	model = strings.TrimSpace(model)
	if prompt == "" {
		return "", ErrEmptyPrompt
	}
	if model == "" {
		return "", ErrInvalidModel
	}
	body := openAIResponsesRequest{
		Model: model,
		Input: []any{
			openAIInputMessage{
				Type: "message",
				Role: "developer",
				Content: []openAIInputContent{{
					Type: "input_text",
					Text: "Create a concise conversation title from the user's first message. Return only the title. Use 3 to 8 words. Do not use quotation marks or trailing punctuation.",
				}},
			},
			openAIInputMessage{
				Type: "message",
				Role: "user",
				Content: []openAIInputContent{{
					Type: "input_text",
					Text: prompt,
				}},
			},
		},
	}
	response, err := p.createResponse(ctx, body)
	if err != nil {
		return "", err
	}
	title := sanitizeGeneratedTitle(extractResponseText(response))
	if title == "" {
		return "", fmt.Errorf("%w: empty title response", ErrOpenAIRequestFailed)
	}
	return title, nil
}

func sanitizeGeneratedTitle(title string) string {
	title = strings.TrimSpace(title)
	title = strings.Trim(title, "\"'`“”‘’")
	title = strings.Join(strings.Fields(title), " ")
	title = strings.TrimSpace(title)
	title = strings.Trim(title, "\"'`“”‘’")
	runes := []rune(title)
	if len(runes) <= maxGeneratedTitleLength {
		return title
	}
	return strings.TrimSpace(string(runes[:maxGeneratedTitleLength]))
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
	Model        string           `json:"model"`
	Instructions string           `json:"instructions,omitempty"`
	Input        any              `json:"input"`
	Reasoning    *openAIReasoning `json:"reasoning,omitempty"`
	Tools        []openAITool     `json:"tools,omitempty"`
	Stream       bool             `json:"stream,omitempty"`
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
	Type        string              `json:"type"`
	Description string              `json:"description,omitempty"`
	Items       *openAIToolProperty `json:"items,omitempty"`
	Enum        []string            `json:"enum,omitempty"`
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

type openAIStreamingResult struct {
	Text      string
	ToolCalls []openAIToolCall
}

func (p *OpenAIProvider) createStreamingResponse(ctx context.Context, body openAIResponsesRequest, stream Stream) (openAIStreamingResult, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return openAIStreamingResult{}, err
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/responses", bytes.NewReader(payload))
	if err != nil {
		return openAIStreamingResult{}, err
	}
	httpRequest.Header.Set("Authorization", "Bearer "+p.apiKey)
	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("Accept", "text/event-stream")

	response, err := p.client.Do(httpRequest)
	if err != nil {
		return openAIStreamingResult{}, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		responseBody, readErr := io.ReadAll(io.LimitReader(response.Body, 4<<20))
		if readErr != nil {
			return openAIStreamingResult{}, readErr
		}
		return openAIStreamingResult{}, fmt.Errorf("%w: status %d: %s", ErrOpenAIRequestFailed, response.StatusCode, strings.TrimSpace(string(responseBody)))
	}
	if !strings.Contains(strings.ToLower(response.Header.Get("Content-Type")), "text/event-stream") {
		responseBody, readErr := io.ReadAll(io.LimitReader(response.Body, 4<<20))
		if readErr != nil {
			return openAIStreamingResult{}, readErr
		}
		var parsed openAIResponsesResponse
		if err := json.Unmarshal(responseBody, &parsed); err != nil {
			return openAIStreamingResult{}, err
		}
		return openAIStreamingResult{
			Text:      extractResponseText(parsed),
			ToolCalls: extractToolCalls(parsed),
		}, nil
	}
	return readOpenAIStream(ctx, response.Body, stream)
}

func readOpenAIStream(ctx context.Context, reader io.Reader, stream Stream) (openAIStreamingResult, error) {
	lineReader := bufio.NewReader(reader)
	var result openAIStreamingResult
	var eventType string
	var dataLines []string
	for {
		line, err := lineReader.ReadString('\n')
		if err != nil && len(line) == 0 {
			if errors.Is(err, io.EOF) {
				return result, nil
			}
			return openAIStreamingResult{}, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			if err := handleOpenAIStreamEvent(ctx, eventType, strings.Join(dataLines, "\n"), stream, &result); err != nil {
				return openAIStreamingResult{}, err
			}
			eventType = ""
			dataLines = nil
			if err != nil {
				if errors.Is(err, io.EOF) {
					return result, nil
				}
				return openAIStreamingResult{}, err
			}
			continue
		}
		if strings.HasPrefix(line, "event:") {
			eventType = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		}
		if strings.HasPrefix(line, "data:") {
			data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if data != "[DONE]" {
				dataLines = append(dataLines, data)
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				if err := handleOpenAIStreamEvent(ctx, eventType, strings.Join(dataLines, "\n"), stream, &result); err != nil {
					return openAIStreamingResult{}, err
				}
				return result, nil
			}
			return openAIStreamingResult{}, err
		}
	}
}

func handleOpenAIStreamEvent(ctx context.Context, eventType, data string, stream Stream, result *openAIStreamingResult) error {
	if strings.TrimSpace(data) == "" {
		return nil
	}
	var envelope struct {
		Type      string                  `json:"type"`
		Delta     string                  `json:"delta"`
		Text      string                  `json:"text"`
		Output    string                  `json:"output"`
		CallID    string                  `json:"call_id"`
		Name      string                  `json:"name"`
		Arguments string                  `json:"arguments"`
		Item      openAIOutput            `json:"item"`
		Response  openAIResponsesResponse `json:"response"`
		Error     *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(data), &envelope); err != nil {
		return err
	}
	if envelope.Error != nil {
		return fmt.Errorf("%w: %s", ErrOpenAIRequestFailed, envelope.Error.Message)
	}
	if eventType == "" {
		eventType = envelope.Type
	}
	switch eventType {
	case "response.output_text.delta":
		if envelope.Delta != "" {
			result.Text += envelope.Delta
			if stream != nil {
				stream.Delta(ctx, envelope.Delta)
			}
		}
	case "response.output_text.done":
		if strings.TrimSpace(result.Text) == "" && envelope.Text != "" {
			result.Text = envelope.Text
		}
	case "response.output_item.done":
		if envelope.Item.Type == "function_call" {
			result.ToolCalls = upsertOpenAIToolCall(result.ToolCalls, openAIToolCall{
				CallID:    envelope.Item.CallID,
				Name:      envelope.Item.Name,
				Arguments: envelope.Item.Arguments,
			})
		}
	case "response.function_call_arguments.done":
		if envelope.CallID != "" {
			result.ToolCalls = upsertOpenAIToolCall(result.ToolCalls, openAIToolCall{
				CallID:    envelope.CallID,
				Name:      envelope.Name,
				Arguments: envelope.Arguments,
			})
		}
	case "response.completed":
		if strings.TrimSpace(result.Text) == "" {
			result.Text = extractResponseText(envelope.Response)
		}
		if len(result.ToolCalls) == 0 {
			result.ToolCalls = extractToolCalls(envelope.Response)
		}
	case "response.failed", "response.incomplete":
		return fmt.Errorf("%w: stream ended with %s", ErrOpenAIRequestFailed, eventType)
	}
	return nil
}

func upsertOpenAIToolCall(calls []openAIToolCall, next openAIToolCall) []openAIToolCall {
	for index, call := range calls {
		if call.CallID != next.CallID {
			continue
		}
		if next.Name != "" {
			calls[index].Name = next.Name
		}
		if next.Arguments != "" {
			calls[index].Arguments = next.Arguments
		}
		return calls
	}
	return append(calls, next)
}

func buildOpenAIInput(request ProviderRequest) []any {
	input := make([]any, 0, 3+len(request.ConversationContext)+len(request.History))
	input = append(input, openAIDeveloperMessage(buildProviderDeveloperPrompt(request)...))
	if contextMessage := buildProviderUserContextMessage(request); contextMessage != "" {
		input = append(input, openAIMessage("user", contextMessage))
	}
	hasConversationInput := false
	if strings.TrimSpace(request.ContextSummary) != "" {
		input = append(input, openAIMessage("user", "Conversation summary:\n"+strings.TrimSpace(request.ContextSummary)))
		hasConversationInput = true
	}
	for _, message := range request.ConversationContext {
		if strings.TrimSpace(message.Content) == "" {
			continue
		}
		input = append(input, openAIMessage(message.Role, message.Content))
		hasConversationInput = true
	}
	if !hasConversationInput {
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

func openAIDeveloperMessage(parts ...string) openAIInputMessage {
	content := make([]openAIInputContent, 0, len(parts))
	for _, part := range parts {
		if strings.TrimSpace(part) == "" {
			continue
		}
		content = append(content, openAIInputContent{Type: "input_text", Text: strings.TrimSpace(part)})
	}
	return openAIInputMessage{
		Type:    "message",
		Role:    "developer",
		Content: content,
	}
}

func buildSummaryInput(request SummaryRequest) []any {
	input := make([]any, 0, 2+len(request.Messages))
	input = append(input, openAIDeveloperMessage(summaryDeveloperPrompt()...))
	if strings.TrimSpace(request.ExistingSummary) != "" {
		input = append(input, openAIMessage("user", "Existing conversation summary:\n"+strings.TrimSpace(request.ExistingSummary)))
	}
	for _, message := range request.Messages {
		input = append(input, openAIMessage(message.Role, message.Content))
	}
	return input
}

func buildProviderPrompt(request ProviderRequest) string {
	return strings.Join(buildProviderDeveloperPrompt(request), "\n\n") + "\n\nCurrent user prompt:\n" + strings.TrimSpace(request.Prompt)
}

func skillPromptPath(skill skills.Skill) string {
	key := strings.TrimSpace(skill.Key)
	if key == "" {
		return ""
	}
	switch skill.Source {
	case "patchpilot":
		return "~/.patchpilot/skills/" + key + "/SKILL.md"
	case "agents":
		return "~/.agents/skills/" + key + "/SKILL.md"
	default:
		return ""
	}
}

func buildProviderUserContextMessage(request ProviderRequest) string {
	blocks := []string{}
	var builder strings.Builder
	blocks = append(blocks, xmlBlockPreserve(openAIXMLTagEnvironmentContext, buildEnvironmentContext(request)))

	if len(request.RepoInstructions) > 0 {
		builder.WriteString("Effective repo instructions, in precedence order:\n")
		for _, source := range request.RepoInstructions {
			if strings.TrimSpace(source.Content) == "" {
				continue
			}
			builder.WriteString("\nSource: ")
			builder.WriteString(source.Path)
			builder.WriteString("\n")
			builder.WriteString(source.Content)
			builder.WriteString("\n")
		}
	}
	if text := strings.TrimSpace(builder.String()); text != "" {
		blocks = append(blocks, xmlBlock(openAIXMLTagRepoInstructions, text))
	}

	builder.Reset()
	if len(request.ContextWarnings) > 0 {
		builder.WriteString("Context warnings:\n")
		for _, warning := range request.ContextWarnings {
			if warning.Path != "" {
				builder.WriteString("- ")
				builder.WriteString(warning.Path)
				builder.WriteString(": ")
			} else {
				builder.WriteString("- ")
			}
			builder.WriteString(warning.Message)
			builder.WriteString("\n")
		}
	}
	if text := strings.TrimSpace(builder.String()); text != "" {
		blocks = append(blocks, xmlBlock(openAIXMLTagContextWarnings, text))
	}
	return strings.Join(blocks, "\n\n")
}

func buildProviderDeveloperPrompt(request ProviderRequest) []string {
	blocks := []string{
		xmlBlock(openAIXMLTagPatchPilotAgent, providerInstructions()),
	}
	if text := buildSelectedSkillsBlock(request.SelectedSkills); text != "" {
		blocks = append(blocks, text)
	}
	return blocks
}

func buildSelectedSkillsBlock(selectedSkills []skills.Skill) string {
	var builder strings.Builder
	if len(selectedSkills) > 0 {
		builder.WriteString("Selected local skills:\n")
		for _, skill := range selectedSkills {
			if strings.TrimSpace(skill.Name) == "" || strings.TrimSpace(skill.Description) == "" {
				continue
			}
			builder.WriteString("\nName: ")
			builder.WriteString(skill.Name)
			builder.WriteString("\nDescription: ")
			builder.WriteString(skill.Description)
			builder.WriteString("\n")
			if path := skillPromptPath(skill); path != "" {
				builder.WriteString("Path: ")
				builder.WriteString(path)
				builder.WriteString("\n")
			}
		}
	}
	if text := strings.TrimSpace(builder.String()); text != "" {
		return xmlBlock(openAIXMLTagSkillsInstructions, text)
	}
	return ""
}

func buildEnvironmentContext(request ProviderRequest) string {
	now := time.Now()
	cwd := strings.TrimSpace(request.WorkspaceRoot)
	shell := strings.TrimSpace(request.Shell)
	if shell == "" {
		shell = filepath.Base(os.Getenv("SHELL"))
	}
	currentDate := strings.TrimSpace(request.CurrentDate)
	if currentDate == "" {
		currentDate = now.Format("2006-01-02")
	}
	timezone := strings.TrimSpace(request.Timezone)
	if timezone == "" {
		timezone = os.Getenv("TZ")
	}
	if timezone == "" {
		timezone = now.Location().String()
	}
	return strings.Join([]string{
		"\t" + xmlInlineBlock(openAIXMLTagCWD, cwd),
		"\t" + xmlInlineBlock(openAIXMLTagShell, shell),
		"\t" + xmlInlineBlock(openAIXMLTagCurrentDate, currentDate),
		"\t" + xmlInlineBlock(openAIXMLTagTimezone, timezone),
	}, "\n")
}

type openAIXMLTag string

func xmlBlock(tag openAIXMLTag, content string) string {
	name := string(tag)
	return "<" + name + ">\n" + strings.TrimSpace(content) + "\n</" + name + ">"
}

func xmlBlockPreserve(tag openAIXMLTag, content string) string {
	name := string(tag)
	return "<" + name + ">\n" + content + "\n</" + name + ">"
}

func xmlInlineBlock(tag openAIXMLTag, content string) string {
	name := string(tag)
	return "<" + name + ">" + strings.TrimSpace(content) + "</" + name + ">"
}

func providerInstructions() string {
	return `You are PatchPilot's coding agent.

Rules:
- Inspect context only from the server-provided conversation context and the available workspace tools.
- Return assistant text when useful.
- Use tools for workspace discovery, file reads, commands, and patches.
- When a local skill description is relevant and you need its detailed instructions, read the listed skill Path through run_command. Prefer cat <skill-path>; use sed chunks only when needed.
- When calling tools, include concise output_text in the same response that tells the user what you are checking or changing so the user sees progress while tool calls are pending.
- Write assistant text, including output_text sent with tool calls, in the same language as the user's prompt unless the user explicitly asks for a different language.
- If the user prompt asks for a change or investigation, do not answer with readiness, greetings, or "what would you like me to do" questions.
- For change or investigation prompts, first call at least one workspace inspection tool such as search_files, list_files, or run_command unless the answer can be completed entirely from prior tool results.
- Read file contents through run_command. Prefer sed chunks such as sed -n '1,160p' path/to/file. Use cat path/to/file only when the full file is needed.
- Inspect Git only through run_command. Prefer exact commands git status, git diff, and git log; do not call dedicated Git status/diff tools.
- Ask a clarifying question only when the user prompt is genuinely missing the target or desired outcome.
- Do not claim files were modified unless the apply_patch tool result says the patch was applied.
- Do not include secrets.
- apply_patch input must include "summary" and a complete git-apply-compatible unified "diff".
- Every changed file in a patch diff must include diff --git, ---/+++, and @@ hunk headers with line numbers.
- Tool calls in one response may run as a batch. Approval-required tools are decided by the user before the batch runs.`
}

func summaryDeveloperPrompt() []string {
	return []string{
		xmlBlock(openAIXMLTagSummaryTask, "Summarize older PatchPilot conversation context for a coding agent."),
		xmlBlock(openAIXMLTagSummaryRules, summaryRules()),
	}
}

func summaryRules() string {
	return `Rules:
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
