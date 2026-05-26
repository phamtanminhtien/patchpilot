package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
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
