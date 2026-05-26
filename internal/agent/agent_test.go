package agent

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/phamtanminhtien/patchpilot/internal/database"
	"github.com/phamtanminhtien/patchpilot/internal/events"
	"github.com/phamtanminhtien/patchpilot/internal/filestore"
	"github.com/phamtanminhtien/patchpilot/internal/gitrepo"
	"github.com/phamtanminhtien/patchpilot/internal/runner"
)

type testProvider struct {
	mu              sync.Mutex
	err             error
	summary         string
	streamDeltas    []string
	turns           []ProviderResult
	seen            []ProviderRequest
	summaryRequests []SummaryRequest
}

func (p *testProvider) Configured() bool {
	return true
}

func (p *testProvider) Generate(ctx context.Context, request ProviderRequest, stream Stream) (ProviderResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.seen = append(p.seen, request)
	if p.err != nil {
		return ProviderResult{}, p.err
	}
	if len(p.turns) == 0 {
		for _, delta := range p.streamDeltas {
			if stream != nil {
				stream.Delta(ctx, delta)
			}
		}
		return ProviderResult{Text: "done", Done: true}, nil
	}
	turn := p.turns[0]
	p.turns = p.turns[1:]
	for _, delta := range p.streamDeltas {
		if stream != nil {
			stream.Delta(ctx, delta)
		}
	}
	return turn, nil
}

func (p *testProvider) Summarize(ctx context.Context, request SummaryRequest) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.summaryRequests = append(p.summaryRequests, request)
	if p.err != nil {
		return "", p.err
	}
	if p.summary != "" {
		return p.summary, nil
	}
	return "summarized older context", nil
}

type unconfiguredProvider struct{}

func (unconfiguredProvider) Configured() bool {
	return false
}

func (unconfiguredProvider) Generate(context.Context, ProviderRequest, Stream) (ProviderResult, error) {
	return ProviderResult{}, ErrProviderUnavailable
}

func (unconfiguredProvider) Summarize(context.Context, SummaryRequest) (string, error) {
	return "", ErrProviderUnavailable
}

type blockingTestProvider struct {
	delta string
	done  chan struct{}
}

func (p blockingTestProvider) Configured() bool {
	return true
}

func (p blockingTestProvider) Generate(ctx context.Context, request ProviderRequest, stream Stream) (ProviderResult, error) {
	if stream != nil {
		stream.Delta(ctx, p.delta)
	}
	if p.done != nil {
		close(p.done)
	}
	<-ctx.Done()
	return ProviderResult{}, ctx.Err()
}

func (p blockingTestProvider) Summarize(context.Context, SummaryRequest) (string, error) {
	return "fake summary", nil
}

func writeAgentSkill(t *testing.T, home, key, name, description, body string) {
	t.Helper()
	path := filepath.Join(home, ".patchpilot", "skills", key, "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir skill: %v", err)
	}
	content := `---
name: ` + name + `
description: ` + description + `
---
` + body
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write skill: %v", err)
	}
}

func hasEvent(events []database.AgentRunEventRecord, eventType string) bool {
	for _, event := range events {
		if event.Type == eventType {
			return true
		}
	}
	return false
}

func hasAgentDeltaText(events []database.AgentRunEventRecord, text string) bool {
	for _, event := range events {
		if event.Type != "agent.delta" {
			continue
		}
		if strings.Contains(event.PayloadJSON, text) {
			return true
		}
	}
	return false
}

func hasAgentMessage(messages []database.MessageRecord, role, content string) bool {
	for _, message := range messages {
		if message.Role == role && message.Content == content {
			return true
		}
	}
	return false
}

func newAgentTestManager(t *testing.T, provider Provider) (*Manager, *database.Store) {
	t.Helper()
	return newAgentTestManagerWithHub(t, provider, events.NewHub())
}

func newAgentTestManagerWithHub(t *testing.T, provider Provider, hub *events.Hub) (*Manager, *database.Store) {
	t.Helper()
	store, err := database.Open(filepath.Join(t.TempDir(), "patchpilot.db"))
	if err != nil {
		t.Fatalf("database.Open returned error: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	})
	manager := NewManager(store, filestore.NewService(), gitrepo.NewClient(), runner.NewRunner(), hub, provider)
	if _, err := store.CreateConversation(context.Background(), database.ConversationRecord{
		ID:          "conv_1",
		WorkspaceID: "ws_1",
		Title:       "Test conversation",
	}); err != nil {
		t.Fatalf("CreateConversation returned error: %v", err)
	}
	return manager, store
}

func receivedAgentDelta(eventsCh <-chan events.Event) bool {
	deadline := time.After(500 * time.Millisecond)
	for {
		select {
		case event := <-eventsCh:
			if event.Type == "agent.delta" {
				return true
			}
		case <-deadline:
			return false
		}
	}
}

func receiveAgentDeltaText(t *testing.T, eventsCh <-chan events.Event, count int) string {
	t.Helper()
	var builder strings.Builder
	received := 0
	deadline := time.After(500 * time.Millisecond)
	for received < count {
		select {
		case event := <-eventsCh:
			if event.Type != "agent.delta" {
				continue
			}
			payload, ok := event.Payload.(map[string]string)
			if !ok {
				t.Fatalf("expected agent.delta payload map, got %T", event.Payload)
			}
			builder.WriteString(payload["text"])
			received++
		case <-deadline:
			t.Fatalf("timed out waiting for %d agent.delta events, got %q", count, builder.String())
		}
	}
	return builder.String()
}

func waitForAgentRun(t *testing.T, manager *Manager, workspaceID, conversationID, runID string, status RunStatus) Detail {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		detail, err := manager.Detail(context.Background(), workspaceID, conversationID, runID)
		if err != nil {
			t.Fatalf("Detail returned error: %v", err)
		}
		if detail.Run.Status == string(status) {
			return detail
		}
		time.Sleep(20 * time.Millisecond)
	}
	detail, _ := manager.Detail(context.Background(), workspaceID, conversationID, runID)
	taskError := ""
	if detail.Run.Error != nil {
		taskError = *detail.Run.Error
	}
	t.Fatalf("run did not reach %s: %+v error=%q", status, detail.Run, taskError)
	return Detail{}
}

func initAgentGitRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	cmd := exec.Command("git", "init")
	cmd.Dir = root
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v\n%s", err, output)
	}
	return root
}

func readAgentFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ""
		}
		t.Fatalf("read file: %v", err)
	}
	return string(content)
}
