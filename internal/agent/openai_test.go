package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/phamtanminhtien/patchpilot/internal/filestore"
)

func TestOpenAIProviderUsesCustomBaseURLResponsesPath(t *testing.T) {
	var requestPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.Path
		if r.Header.Get("Authorization") != "Bearer sk-test" {
			t.Fatalf("unexpected authorization header: %q", r.Header.Get("Authorization"))
		}
		_, _ = w.Write([]byte(`{"output_text":"{\"plan\":\"plan\",\"summary\":\"summary\",\"patch\":\"\"}"}`))
	}))
	defer server.Close()

	provider := NewOpenAIProvider("sk-test", server.URL+"/v1/")
	task := Task{
		ID:              "task_1",
		WorkspaceID:     "ws_1",
		Prompt:          "fix",
		Model:           "gpt-5.5",
		ReasoningEffort: "medium",
	}
	result, err := provider.Generate(context.Background(), ProviderRequest{Task: task}, nil, nil)
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	if requestPath != "/v1/responses" {
		t.Fatalf("expected /v1/responses path, got %q", requestPath)
	}
	if result.Plan != "plan" || result.Summary != "summary" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestOpenAIProviderRunsFileTools(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "note.txt"), []byte("hello from workspace"), 0o644); err != nil {
		t.Fatalf("write note: %v", err)
	}

	var requests []map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		requests = append(requests, body)

		switch len(requests) {
		case 1:
			tools, ok := body["tools"].([]any)
			if !ok || len(tools) != 2 {
				t.Fatalf("expected file tools in initial request, got %#v", body["tools"])
			}
			_, _ = w.Write([]byte(`{
				"id":"resp_1",
				"output":[
					{"type":"reasoning","id":"rs_1","summary":[]},
					{"type":"function_call","call_id":"call_search","name":"search_files","arguments":"{\"query\":\"note\"}"},
					{"type":"function_call","call_id":"call_read","name":"read_file","arguments":"{\"path\":\"note.txt\"}"}
				]
			}`))
		case 2:
			input, ok := body["input"].([]any)
			if !ok || len(input) != 5 {
				t.Fatalf("expected prompt, two synthetic tool calls, and two tool outputs, got %#v", body["input"])
			}
			encoded, err := json.Marshal(input)
			if err != nil {
				t.Fatalf("marshal input: %v", err)
			}
			if !strings.Contains(string(encoded), "function_call_output") ||
				!strings.Contains(string(encoded), "hello from workspace") ||
				!strings.Contains(string(encoded), "note.txt") {
				t.Fatalf("expected read/search tool output, got %s", encoded)
			}
			if strings.Contains(string(encoded), "rs_1") || strings.Contains(string(encoded), `"summary":[]`) {
				t.Fatalf("expected stateless replay without reasoning item, got %s", encoded)
			}
			_, _ = w.Write([]byte(`{"output_text":"{\"plan\":\"plan\",\"summary\":\"summary\",\"patch\":\"diff --git a/note.txt b/note.txt\\n\"}"}`))
		default:
			t.Fatalf("unexpected request count: %d", len(requests))
		}
	}))
	defer server.Close()

	provider := NewOpenAIProvider("sk-test", server.URL)
	task := Task{
		ID:              "task_1",
		WorkspaceID:     "ws_1",
		Prompt:          "update note.txt",
		Model:           "gpt-5.5",
		ReasoningEffort: "medium",
	}
	result, err := provider.Generate(context.Background(), ProviderRequest{Task: task}, &Tools{
		workspaceRoot: root,
		files:         filestore.NewService(),
	}, nil)
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	if result.Patch == "" {
		t.Fatalf("expected patch after tool calls, got %+v", result)
	}
	if len(requests) != 2 {
		t.Fatalf("expected two OpenAI requests, got %d", len(requests))
	}
}

func TestBuildProviderPromptRequiresGitApplyCompatiblePatch(t *testing.T) {
	prompt := buildProviderPrompt(ProviderRequest{Task: Task{Prompt: "fix"}})

	for _, expected := range []string{
		"complete git-apply-compatible unified diff",
		"diff --git",
		"---/+++",
		"@@ hunk headers with line numbers",
		"Do not return compact diff summaries",
	} {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("expected prompt to contain %q, got %s", expected, prompt)
		}
	}
}

func TestNormalizeProviderPatchConvertsCompactDiff(t *testing.T) {
	compact := `internal/agent/openai.go
  @@ -1,3 +1,3 @@
   package agent
  -old
  +new
  
  +1 -1`

	normalized := normalizeProviderPatch(compact)
	for _, expected := range []string{
		"diff --git a/internal/agent/openai.go b/internal/agent/openai.go",
		"--- a/internal/agent/openai.go",
		"+++ b/internal/agent/openai.go",
		"@@ -1,3 +1,3 @@",
		"-old",
		"+new",
	} {
		if !strings.Contains(normalized, expected) {
			t.Fatalf("expected normalized patch to contain %q, got %s", expected, normalized)
		}
	}
	if strings.Contains(normalized, "+1 -1") {
		t.Fatalf("expected compact summary to be removed, got %s", normalized)
	}
}

func TestNormalizeProviderPatchConvertsMultipleCompactFiles(t *testing.T) {
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
		"docs/mvp-spec.md\n" +
		"  @@ -315,10 +315,9 @@ Workspace source files stay in their original repositories and are not copied in\n" +
		"  -queued -> running -> waiting_approval -> applying -> done\n" +
		"  +queued -> running -> waiting_approval\n" +
		"   ```\n" +
		"  \n" +
		"   Patch status:\n" +
		"  +1 -1\n" +
		"\n" +
		"web/src/shared/api/types.ts\n" +
		"  @@ -142,10 +142,7 @@ export type AgentTaskStatus =\n" +
		"  -  | \"applying\"\n" +
		"     | \"done\"\n" +
		"  +0 -1"

	normalized := normalizeProviderPatch(compact)
	for _, expected := range []string{
		"diff --git a/docs/mvp-spec.md b/docs/mvp-spec.md",
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
