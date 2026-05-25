package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/phamtanminhtien/patchpilot/internal/skills"
)

func TestOpenAIProviderUsesCustomBaseURLResponsesPath(t *testing.T) {
	var requestPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.Path
		if r.Header.Get("Authorization") != "Bearer sk-test" {
			t.Fatalf("unexpected authorization header: %q", r.Header.Get("Authorization"))
		}
		writeOpenAIStream(w, "response.output_text.delta", `{"delta":"Done"}`)
		writeOpenAIStream(w, "response.completed", `{"response":{"output_text":"Done"}}`)
	}))
	defer server.Close()

	provider := NewOpenAIProvider("sk-test", server.URL+"/v1/")
	run := Run{
		ID:              "run_1",
		WorkspaceID:     "ws_1",
		Model:           "gpt-5.5",
		ReasoningEffort: "medium",
	}
	result, err := provider.Generate(context.Background(), ProviderRequest{Run: run, Prompt: "fix"}, nil)
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	if requestPath != "/v1/responses" {
		t.Fatalf("expected /v1/responses path, got %q", requestPath)
	}
	if result.Text != "Done" || !result.Done {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestOpenAIProviderReadsStreamingEventTypeFromData(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(`data: {"type":"response.output_text.delta","delta":"Streamed"}` + "\n\n"))
		_, _ = w.Write([]byte(`data: {"type":"response.completed","response":{"output_text":"Streamed"}}` + "\n\n"))
	}))
	defer server.Close()

	provider := NewOpenAIProvider("sk-test", server.URL)
	run := Run{
		ID:              "run_1",
		WorkspaceID:     "ws_1",
		Model:           "gpt-5.5",
		ReasoningEffort: "medium",
	}
	result, err := provider.Generate(context.Background(), ProviderRequest{Run: run, Prompt: "fix"}, nil)
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	if result.Text != "Streamed" {
		t.Fatalf("expected streamed text, got %+v", result)
	}
}

func TestOpenAIProviderFallsBackToJSONResponseWhenStreamingUnavailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"output_text":"JSON done"}`))
	}))
	defer server.Close()

	provider := NewOpenAIProvider("sk-test", server.URL)
	run := Run{
		ID:              "run_1",
		WorkspaceID:     "ws_1",
		Model:           "gpt-5.5",
		ReasoningEffort: "medium",
	}
	result, err := provider.Generate(context.Background(), ProviderRequest{Run: run, Prompt: "fix"}, nil)
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	if result.Text != "JSON done" {
		t.Fatalf("expected JSON fallback text, got %+v", result)
	}
}

func TestOpenAIProviderReturnsToolCallsAndReplaysHistory(t *testing.T) {
	var requests []map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		requests = append(requests, body)

		switch len(requests) {
		case 1:
			instructions, ok := body["instructions"].(string)
			if !ok || !strings.Contains(instructions, "PatchPilot's coding agent") {
				t.Fatalf("expected provider instructions, got %#v", body["instructions"])
			}
			encodedInput, err := json.Marshal(body["input"])
			if err != nil {
				t.Fatalf("marshal input: %v", err)
			}
			if strings.Contains(string(encodedInput), "PatchPilot's coding agent") {
				t.Fatalf("expected agent instructions outside input, got %s", encodedInput)
			}
			tools, ok := body["tools"].([]any)
			if !ok || len(tools) != 6 {
				t.Fatalf("expected six tools in initial request, got %#v", body["tools"])
			}
			writeOpenAIStream(w, "response.output_text.delta", `{"delta":"I will inspect the workspace before patching."}`)
			writeOpenAIStream(w, "response.output_item.done", `{"item":{"type":"function_call","call_id":"call_search","name":"search_files","arguments":"{\"query\":\"note\"}"}}`)
			writeOpenAIStream(w, "response.output_item.done", `{"item":{"type":"function_call","call_id":"call_patch","name":"apply_patch","arguments":"{\"summary\":\"s\",\"diff\":\"d\"}"}}`)
			writeOpenAIStream(w, "response.completed", `{"response":{"output_text":"I will inspect the workspace before patching."}}`)
		case 2:
			input, ok := body["input"].([]any)
			if !ok || len(input) != 5 {
				t.Fatalf("expected prompt, two calls, and two outputs, got %#v", body["input"])
			}
			encoded, err := json.Marshal(input)
			if err != nil {
				t.Fatalf("marshal input: %v", err)
			}
			if !strings.Contains(string(encoded), "function_call_output") ||
				!strings.Contains(string(encoded), "search output") ||
				!strings.Contains(string(encoded), "patch rejected") {
				t.Fatalf("expected tool history replay, got %s", encoded)
			}
			if strings.Contains(string(encoded), "rs_1") || strings.Contains(string(encoded), `"summary":[]`) {
				t.Fatalf("expected stateless replay without reasoning item, got %s", encoded)
			}
			writeOpenAIStream(w, "response.output_text.delta", `{"delta":"All done"}`)
			writeOpenAIStream(w, "response.completed", `{"response":{"output_text":"All done"}}`)
		default:
			t.Fatalf("unexpected request count: %d", len(requests))
		}
	}))
	defer server.Close()

	provider := NewOpenAIProvider("sk-test", server.URL)
	run := Run{
		ID:              "run_1",
		WorkspaceID:     "ws_1",
		Model:           "gpt-5.5",
		ReasoningEffort: "medium",
	}
	result, err := provider.Generate(context.Background(), ProviderRequest{Run: run, Prompt: "update note.txt"}, nil)
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	if result.Text != "I will inspect the workspace before patching." {
		t.Fatalf("expected output_text with tool calls, got %+v", result)
	}
	if len(result.ToolCalls) != 2 || result.ToolCalls[1].Name != "apply_patch" {
		t.Fatalf("expected tool calls, got %+v", result)
	}
	result, err = provider.Generate(context.Background(), ProviderRequest{
		Run:    run,
		Prompt: "update note.txt",
		History: []ProviderHistoryItem{
			{Type: "tool_call", ToolCall: result.ToolCalls[0]},
			{Type: "tool_call", ToolCall: result.ToolCalls[1]},
			{Type: "tool_result", ToolResult: ToolResponse{CallID: "call_search", Output: "search output"}},
			{Type: "tool_result", ToolResult: ToolResponse{CallID: "call_patch", Output: "patch rejected"}},
		},
	}, nil)
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	if result.Text != "All done" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if len(requests) != 2 {
		t.Fatalf("expected two OpenAI requests, got %d", len(requests))
	}
}

func writeOpenAIStream(w http.ResponseWriter, eventType, data string) {
	w.Header().Set("Content-Type", "text/event-stream")
	_, _ = w.Write([]byte("event: " + eventType + "\n"))
	_, _ = w.Write([]byte("data: " + data + "\n\n"))
}

func TestOpenAIProviderPromptRejectsReadinessForConcreteTasks(t *testing.T) {
	run := Run{
		ID:              "run_1",
		WorkspaceID:     "ws_1",
		Model:           "gpt-5.5",
		ReasoningEffort: "medium",
	}
	prompt := buildProviderPrompt(ProviderRequest{Run: run, Prompt: "Inspect the README and update one sentence."})
	for _, expected := range []string{
		"do not answer with readiness",
		"first call at least one workspace inspection tool",
		"Inspect Git only through run_command",
		"Ask a clarifying question only",
		"include concise output_text in the same response",
		"same language as the user's prompt",
		"Use use_skill",
	} {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("expected provider prompt to contain %q, got %s", expected, prompt)
		}
	}
}

func TestOpenAIToolsExposeFileOptionsAndNoDedicatedGitTools(t *testing.T) {
	tools := openAITools()
	names := make(map[string]openAITool)
	for _, tool := range tools {
		names[tool.Name] = tool
	}
	for _, removed := range []string{"git_" + "status", "git_" + "diff"} {
		if _, ok := names[removed]; ok {
			t.Fatalf("expected %s to be removed from agent tools", removed)
		}
	}
	searchTool, ok := names["search_files"]
	if !ok {
		t.Fatal("expected search_files tool")
	}
	if _, ok := searchTool.Parameters.Properties["path"]; !ok {
		t.Fatalf("expected search_files path parameter, got %+v", searchTool.Parameters.Properties)
	}
	readTool, ok := names["read_file"]
	if !ok {
		t.Fatal("expected read_file tool")
	}
	for _, param := range []string{"start_line", "end_line"} {
		if _, ok := readTool.Parameters.Properties[param]; !ok {
			t.Fatalf("expected read_file %s parameter, got %+v", param, readTool.Parameters.Properties)
		}
	}
}

func TestOpenAIProviderInputListsSkillMetadataWithoutBody(t *testing.T) {
	run := Run{
		ID:              "run_1",
		WorkspaceID:     "ws_1",
		Model:           "gpt-5.5",
		ReasoningEffort: "medium",
	}
	input := buildOpenAIInput(ProviderRequest{
		Run:    run,
		Prompt: "Use the browser skill.",
		SelectedSkills: []skills.Skill{
			{
				Name:        "Browser",
				Description: "Browser automation.",
				Instruction: "Secret body instructions should load only through use_skill.",
			},
		},
	})
	encodedInput, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("marshal input: %v", err)
	}
	encoded := string(encodedInput)
	for _, expected := range []string{"Selected local skills", "Name: Browser", "Description: Browser automation."} {
		if !strings.Contains(encoded, expected) {
			t.Fatalf("expected provider input to contain %q, got %s", expected, encoded)
		}
	}
	if strings.Contains(encoded, "Secret body instructions") {
		t.Fatalf("expected provider input to omit skill body, got %s", encoded)
	}
}

func TestOpenAIProviderInputDoesNotEmbedGitStatus(t *testing.T) {
	run := Run{
		ID:              "run_1",
		WorkspaceID:     "ws_1",
		Model:           "gpt-5.5",
		ReasoningEffort: "medium",
	}
	input := buildOpenAIInput(ProviderRequest{Run: run, Prompt: "Review the workspace."})
	encodedInput, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("marshal input: %v", err)
	}
	for _, unexpected := range []string{
		"Workspace git status:",
		" M internal/agent/openai.go",
	} {
		if strings.Contains(string(encodedInput), unexpected) {
			t.Fatalf("expected provider input to omit %q, got %s", unexpected, encodedInput)
		}
	}
}

func TestNormalizeProviderPatchConvertsCompactDiff(t *testing.T) {
	compact := `internal/agent/openai.go
  @@ -1,1 +1,1 @@
  -old
  +new
  +1 -1

internal/agent/openai_test.go
  @@ -2,1 +2,1 @@
  -before
  +after

  +1 -1`

	normalized := normalizeProviderPatch(compact)
	for _, expected := range []string{
		"diff --git a/internal/agent/openai.go b/internal/agent/openai.go",
		"diff --git a/internal/agent/openai_test.go b/internal/agent/openai_test.go",
		"--- a/internal/agent/openai_test.go",
		"+++ b/internal/agent/openai_test.go",
	} {
		if !strings.Contains(normalized, expected) {
			t.Fatalf("expected normalized patch to contain %q, got %s", expected, normalized)
		}
	}
	if strings.Contains(normalized, "+1 -1") {
		t.Fatalf("expected compact summaries to be removed, got %s", normalized)
	}
}

func TestNormalizeProviderPatchConvertsIndentedCompactDiff(t *testing.T) {
	compact := "\n" +
		"docs/product-spec.md\n" +
		"  @@ -315,10 +315,9 @@ Workspace source files stay in their original repositories and are not copied in\n" +
		"  -queued -> running -> waiting_approval -> applying -> done\n" +
		"  +queued -> running -> waiting_approval\n" +
		"   ```\n" +
		"  \n" +
		"   Patch status:\n" +
		"  +1 -1\n" +
		"\n" +
		"web/src/shared/api/types.ts\n" +
		"  @@ -142,10 +142,7 @@ export type AgentRunStatus =\n" +
		"  -  | \"applying\"\n" +
		"     | \"done\"\n" +
		"  +0 -1"

	normalized := normalizeProviderPatch(compact)
	for _, expected := range []string{
		"diff --git a/docs/product-spec.md b/docs/product-spec.md",
		"diff --git a/web/src/shared/api/types.ts b/web/src/shared/api/types.ts",
		"-queued -> running -> waiting_approval -> applying -> done",
		"+queued -> running -> waiting_approval",
		" | \"done\"",
	} {
		if !strings.Contains(normalized, expected) {
			t.Fatalf("expected normalized patch to contain %q, got %s", expected, normalized)
		}
	}
	if strings.Contains(normalized, "+1 -1") || strings.Contains(normalized, "+0 -1") {
		t.Fatalf("expected compact summaries to be removed, got %s", normalized)
	}
}
