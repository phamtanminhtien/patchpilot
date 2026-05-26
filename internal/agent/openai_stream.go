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
	"strings"
)

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
