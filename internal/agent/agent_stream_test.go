package agent

import (
	"context"
	"testing"

	"github.com/phamtanminhtien/patchpilot/internal/events"
)

func TestManagerPublishesTransientDeltaWithoutStoringIt(t *testing.T) {
	root := initAgentGitRepo(t)
	provider := &testProvider{streamDeltas: []string{"hel", "lo"}}
	hub := events.NewHub()
	manager, store := newAgentTestManagerWithHub(t, provider, hub)
	eventsCh, unsubscribe := hub.Subscribe()
	defer unsubscribe()

	run, err := manager.Create(context.Background(), "ws_1", root, CreateRunInput{
		Prompt:           "say hello",
		ConversationID:   "conv_1",
		TriggerMessageID: "msg_1",
		Model:            "gpt-5.5",
		ReasoningEffort:  "medium",
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	waitForAgentRun(t, manager, "ws_1", run.ConversationID, run.ID, StatusDone)
	if !receivedAgentDelta(eventsCh) {
		t.Fatal("expected transient agent.delta on hub")
	}
	records, err := store.ListAgentRunEvents(context.Background(), "ws_1", run.ID)
	if err != nil {
		t.Fatalf("ListAgentRunEvents returned error: %v", err)
	}
	if hasEvent(records, "agent.delta") {
		t.Fatalf("agent.delta should not be stored, got %+v", records)
	}
}

func TestManagerSelectedSkillsAreAvailableInProviderRequest(t *testing.T) {
	root := initAgentGitRepo(t)
	home := t.TempDir()
	writeAgentSkill(t, home, "browser", "Browser", "Browser automation.", "Use the in-app browser.")
	provider := &testProvider{turns: []ProviderResult{
		{Text: "Done", Done: true},
	}}
	manager, _ := newAgentTestManager(t, provider)
	manager.homeDir = home

	run, err := manager.Create(context.Background(), "ws_1", root, CreateRunInput{
		Prompt:           "use browser skill",
		ConversationID:   "conv_1",
		TriggerMessageID: "msg_1",
		Model:            "gpt-5.5",
		ReasoningEffort:  "medium",
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	waitForAgentRun(t, manager, "ws_1", run.ConversationID, run.ID, StatusDone)
	if len(provider.seen) == 0 || len(provider.seen[0].SelectedSkills) != 1 {
		t.Fatalf("expected selected skill in provider request, got %+v", provider.seen)
	}
	skill := provider.seen[0].SelectedSkills[0]
	if skill.Key != "browser" || skill.Name != "Browser" || skill.Description != "Browser automation." {
		t.Fatalf("unexpected selected skill metadata: %+v", skill)
	}
}

func TestManagerStreamsWhitespaceOnlyDeltas(t *testing.T) {
	hub := events.NewHub()
	manager := NewManager(nil, nil, nil, nil, hub, nil)
	manager.setRuntime(&runRuntime{runID: "run_1"})
	eventsCh, unsubscribe := hub.Subscribe()
	defer unsubscribe()

	stream := manager.stream(Run{ID: "run_1", WorkspaceID: "ws_1"})
	for _, delta := range []string{"##", " ", "1", "\n", "```txt", "\n", "internal/"} {
		stream.Delta(context.Background(), delta)
	}

	const want = "## 1\n```txt\ninternal/"
	if got := manager.DraftText("run_1"); got != want {
		t.Fatalf("expected draft text to preserve whitespace, got %q", got)
	}
	if got := receiveAgentDeltaText(t, eventsCh, 7); got != want {
		t.Fatalf("expected streamed text to preserve whitespace, got %q", got)
	}
}

func TestManagerDraftTextTracksActiveStreamOnly(t *testing.T) {
	root := initAgentGitRepo(t)
	provider := &testProvider{
		streamDeltas: []string{"draft ", "text"},
		turns: []ProviderResult{{
			Text:      "I will inspect and prepare the patch.",
			ToolCalls: []ToolRequest{patchToolRequest("call_patch")},
		}},
	}
	manager, _ := newAgentTestManager(t, provider)

	run, err := manager.Create(context.Background(), "ws_1", root, CreateRunInput{
		Prompt:           "fix bug",
		ConversationID:   "conv_1",
		TriggerMessageID: "msg_1",
		Model:            "gpt-5.5",
		ReasoningEffort:  "medium",
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	waitForAgentRun(t, manager, "ws_1", run.ConversationID, run.ID, StatusWaitingToolApproval)
	if got := manager.DraftText(run.ID); got != "" {
		t.Fatalf("draft should clear after assistant message persistence, got %q", got)
	}
}
