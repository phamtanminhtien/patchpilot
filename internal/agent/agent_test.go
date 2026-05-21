package agent

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/phamtanminhtien/patchpilot/internal/database"
	"github.com/phamtanminhtien/patchpilot/internal/events"
	"github.com/phamtanminhtien/patchpilot/internal/filestore"
	"github.com/phamtanminhtien/patchpilot/internal/gitrepo"
	"github.com/phamtanminhtien/patchpilot/internal/runner"
)

type testProvider struct {
	err error
	run func(context.Context, ProviderRequest, *Tools, Stream) (ProviderResult, error)
}

func (p testProvider) Configured() bool {
	return true
}

func (p testProvider) Generate(ctx context.Context, request ProviderRequest, tools *Tools, stream Stream) (ProviderResult, error) {
	if p.run != nil {
		return p.run(ctx, request, tools, stream)
	}
	if p.err != nil {
		return ProviderResult{}, p.err
	}
	stream.Delta(ctx, "provider streamed")
	return ProviderResult{Plan: "plan", Summary: "summary", Patch: "diff --git a/a b/a\n"}, nil
}

type unconfiguredProvider struct{}

func (unconfiguredProvider) Configured() bool {
	return false
}

func (unconfiguredProvider) Generate(context.Context, ProviderRequest, *Tools, Stream) (ProviderResult, error) {
	return ProviderResult{}, ErrProviderUnavailable
}

func TestManagerCreatesTaskAndStoresPatch(t *testing.T) {
	ctx := context.Background()
	root := initAgentGitRepo(t)
	manager, store := newAgentTestManager(t, testProvider{})

	task, err := manager.Create(ctx, "ws_1", root, CreateTaskInput{
		Prompt:          "fix bug",
		Model:           "gpt-5.5",
		ReasoningEffort: "medium",
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	detail := waitForAgentTask(t, manager, "ws_1", task.ID, StatusWaitingApproval)
	if detail.Task.Model != "gpt-5.5" || detail.Task.ReasoningEffort != "medium" {
		t.Fatalf("unexpected task selections: %+v", detail.Task)
	}
	if len(detail.Patches) != 1 || detail.Patches[0].Status != "created" {
		t.Fatalf("expected created patch, got %+v", detail.Patches)
	}
	events, err := store.ListAgentTaskEvents(ctx, "ws_1", task.ID)
	if err != nil {
		t.Fatalf("ListAgentTaskEvents returned error: %v", err)
	}
	if len(events) == 0 {
		t.Fatal("expected stored task events")
	}
}

func TestManagerValidatesInputAndProvider(t *testing.T) {
	manager, _ := newAgentTestManager(t, unconfiguredProvider{})
	_, err := manager.Create(context.Background(), "ws_1", t.TempDir(), CreateTaskInput{
		Prompt:          "fix",
		Model:           "gpt-5.5",
		ReasoningEffort: "medium",
	})
	if !errors.Is(err, ErrProviderUnavailable) {
		t.Fatalf("expected provider unavailable, got %v", err)
	}

	manager, _ = newAgentTestManager(t, testProvider{})
	_, err = manager.Create(context.Background(), "ws_1", t.TempDir(), CreateTaskInput{
		Prompt:          "",
		Model:           "gpt-5.5",
		ReasoningEffort: "medium",
	})
	if !errors.Is(err, ErrEmptyPrompt) {
		t.Fatalf("expected empty prompt, got %v", err)
	}
	_, err = manager.Create(context.Background(), "ws_1", t.TempDir(), CreateTaskInput{
		Prompt:          "fix",
		Model:           "bad",
		ReasoningEffort: "medium",
	})
	if !errors.Is(err, ErrInvalidModel) {
		t.Fatalf("expected invalid model, got %v", err)
	}
	_, err = manager.Create(context.Background(), "ws_1", t.TempDir(), CreateTaskInput{
		Prompt:          "fix",
		Model:           "gpt-5.5",
		ReasoningEffort: "none",
	})
	if !errors.Is(err, ErrInvalidEffort) {
		t.Fatalf("expected invalid effort, got %v", err)
	}
}

func TestToolsBlockSecretsAndRequireCommandApproval(t *testing.T) {
	root := initAgentGitRepo(t)
	if err := os.WriteFile(filepath.Join(root, ".env"), []byte("SECRET=value"), 0o600); err != nil {
		t.Fatalf("write secret: %v", err)
	}
	provider := testProvider{
		run: func(ctx context.Context, _ ProviderRequest, tools *Tools, _ Stream) (ProviderResult, error) {
			if _, err := tools.ReadFile(root, ".env"); !errors.Is(err, ErrSecretPath) {
				return ProviderResult{}, fmt.Errorf("expected secret path error, got %v", err)
			}
			if _, err := tools.RunCommand(ctx, "node scripts/check.js"); err == nil {
				return ProviderResult{}, errors.New("expected command approval")
			}
			return ProviderResult{Plan: "plan", Summary: "approval required"}, nil
		},
	}
	manager, _ := newAgentTestManager(t, provider)
	task, err := manager.Create(context.Background(), "ws_1", root, CreateTaskInput{
		Prompt:          "fix",
		Model:           "gpt-5.5",
		ReasoningEffort: "medium",
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	detail := waitForAgentTask(t, manager, "ws_1", task.ID, StatusDone)
	foundApproval := false
	for _, event := range detail.Events {
		if event.Type == "agent.approval_required" {
			foundApproval = true
		}
	}
	if !foundApproval {
		t.Fatalf("expected approval event, got %+v", detail.Events)
	}
}

func newAgentTestManager(t *testing.T, provider Provider) (*Manager, *database.Store) {
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
	manager := NewManager(store, filestore.NewService(), gitrepo.NewClient(), runner.NewRunner(), events.NewHub(), provider)
	return manager, store
}

func waitForAgentTask(t *testing.T, manager *Manager, workspaceID, taskID string, status TaskStatus) Detail {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		detail, err := manager.Detail(context.Background(), workspaceID, taskID)
		if err != nil {
			t.Fatalf("Detail returned error: %v", err)
		}
		if detail.Task.Status == string(status) {
			return detail
		}
		time.Sleep(20 * time.Millisecond)
	}
	detail, _ := manager.Detail(context.Background(), workspaceID, taskID)
	t.Fatalf("task did not reach %s: %+v", status, detail.Task)
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
