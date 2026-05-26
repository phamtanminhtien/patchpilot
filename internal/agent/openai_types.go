package agent

import "encoding/json"

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
