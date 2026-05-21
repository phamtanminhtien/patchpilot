package agent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
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
