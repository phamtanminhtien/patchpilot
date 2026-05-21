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

func (p *OpenAIProvider) Generate(ctx context.Context, request ProviderRequest, _ *Tools, stream Stream) (ProviderResult, error) {
	if !p.Configured() {
		return ProviderResult{}, ErrProviderUnavailable
	}
	if stream != nil {
		stream.Delta(ctx, "Calling OpenAI provider.")
	}
	body := openAIResponsesRequest{
		Model: request.Task.Model,
		Input: buildProviderPrompt(request),
		Reasoning: openAIReasoning{
			Effort: request.Task.ReasoningEffort,
		},
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return ProviderResult{}, err
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/responses", bytes.NewReader(payload))
	if err != nil {
		return ProviderResult{}, err
	}
	httpRequest.Header.Set("Authorization", "Bearer "+p.apiKey)
	httpRequest.Header.Set("Content-Type", "application/json")

	response, err := p.client.Do(httpRequest)
	if err != nil {
		return ProviderResult{}, err
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return ProviderResult{}, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return ProviderResult{}, fmt.Errorf("%w: status %d", ErrOpenAIRequestFailed, response.StatusCode)
	}
	text := extractResponseText(responseBody)
	if strings.TrimSpace(text) == "" {
		return ProviderResult{}, fmt.Errorf("%w: empty response", ErrOpenAIRequestFailed)
	}
	if stream != nil {
		stream.Delta(ctx, "OpenAI response received.")
	}
	return parseProviderResult(text), nil
}

type openAIResponsesRequest struct {
	Model     string          `json:"model"`
	Input     string          `json:"input"`
	Reasoning openAIReasoning `json:"reasoning"`
}

type openAIReasoning struct {
	Effort string `json:"effort"`
}

func buildProviderPrompt(request ProviderRequest) string {
	return fmt.Sprintf(`You are PatchPilot's coding agent.

Rules:
- Inspect context only from the server-provided prompt.
- Propose patches only. Do not claim files were modified.
- Do not include secrets.
- Return JSON only with string fields "plan", "summary", and "patch".
- "patch" must be a unified diff. Use an empty string if no patch is needed.

Workspace git status:
%s

User prompt:
%s`, request.GitStatus, request.Task.Prompt)
}

func extractResponseText(body []byte) string {
	var parsed struct {
		OutputText string `json:"output_text"`
		Output     []struct {
			Content []struct {
				Text string `json:"text"`
				Type string `json:"type"`
			} `json:"content"`
		} `json:"output"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return ""
	}
	if parsed.OutputText != "" {
		return parsed.OutputText
	}
	var builder strings.Builder
	for _, output := range parsed.Output {
		for _, content := range output.Content {
			if content.Text != "" {
				builder.WriteString(content.Text)
			}
		}
	}
	return builder.String()
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
			Patch:   parsed.Patch,
		}
	}
	return ProviderResult{
		Plan:    "Review the request and produce a patch-first response.",
		Summary: text,
	}
}
