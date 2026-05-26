package agent

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/phamtanminhtien/patchpilot/internal/database"
	"github.com/phamtanminhtien/patchpilot/internal/events"
	"github.com/phamtanminhtien/patchpilot/internal/skills"
)

func (m *Manager) run(runtime *runRuntime) {
	ctx := runtime.ctx
	startedAt := time.Now().UTC()
	record, err := m.store.UpdateAgentRun(ctx, runtime.workspaceID, runtime.conversationID, runtime.runID, map[string]any{
		"status":     string(StatusRunning),
		"started_at": startedAt,
	})
	if err != nil {
		return
	}
	run := RunFromRecord(record)
	_ = m.publish(ctx, run, "agent.run.status_changed", run)

	if err := m.prepareConversationContext(ctx, run, runtime); err != nil {
		m.fail(ctx, run, err)
		return
	}
	m.loop(ctx, runtime)
}

func (m *Manager) resume(runtime *runRuntime) {
	ctx := runtime.ctx
	record, err := m.store.UpdateAgentRun(ctx, runtime.workspaceID, runtime.conversationID, runtime.runID, map[string]any{
		"status": string(StatusRunning),
	})
	if err != nil {
		return
	}
	run := RunFromRecord(record)
	_ = m.publish(ctx, run, "agent.run.status_changed", run)
	calls, err := m.store.ListAgentToolCalls(ctx, runtime.workspaceID, runtime.runID)
	if err != nil {
		m.fail(ctx, run, err)
		return
	}
	batch := callsForBatch(calls, runtime.pendingBatch)
	results := m.executeBatch(ctx, run, runtime, batch)
	runtime.history = append(runtime.history, results...)
	runtime.pendingBatch = ""
	m.loop(ctx, runtime)
}

func (m *Manager) prepareConversationContext(ctx context.Context, run Run, runtime *runRuntime) error {
	snapshot, err := m.RefreshContext(ctx, runtime.workspaceRoot)
	if err != nil {
		return err
	}
	runtime.repoInstructions = snapshot.InstructionSources
	runtime.selectedSkills = skills.EnabledContext(skills.Registry{Skills: snapshot.Skills})
	runtime.contextWarnings = append(snapshot.SkippedSources, snapshot.ContextWarnings...)
	conversation, err := m.store.GetConversation(ctx, runtime.workspaceID, runtime.conversationID)
	if err != nil {
		return err
	}
	afterMessageID := ""
	if conversation.ContextSummaryThroughMessageID != nil {
		afterMessageID = *conversation.ContextSummaryThroughMessageID
	}
	messages, err := m.store.ListMessagesAfter(ctx, runtime.workspaceID, runtime.conversationID, afterMessageID)
	if err != nil {
		return err
	}
	context := buildConversationContext(conversation, messages, runtime.triggerMessageID)
	if len(context.SummarizeRecords) > 0 {
		m.events.Publish(events.Event{
			WorkspaceID: run.WorkspaceID,
			Type:        "agent.delta",
			Payload:     map[string]string{"runId": run.ID, "text": "Summarizing earlier conversation context."},
		})
		summary, err := m.provider.Summarize(ctx, SummaryRequest{
			Run:             run,
			ExistingSummary: context.Summary,
			Messages:        providerMessagesFromRecords(context.SummarizeRecords),
		})
		if err != nil {
			return err
		}
		summary = strings.TrimSpace(summary)
		throughMessageID := context.SummarizeRecords[len(context.SummarizeRecords)-1].ID
		conversation, err = m.store.UpdateConversationContextSummary(ctx, runtime.workspaceID, runtime.conversationID, summary, throughMessageID, time.Now().UTC())
		if err != nil {
			return err
		}
		messages, err = m.store.ListMessagesAfter(ctx, runtime.workspaceID, runtime.conversationID, throughMessageID)
		if err != nil {
			return err
		}
		context = buildConversationContext(conversation, messages, runtime.triggerMessageID)
	}
	runtime.contextSummary = context.Summary
	runtime.conversationContext = context.Messages
	return nil
}

func (m *Manager) loop(ctx context.Context, runtime *runRuntime) {
	for {
		record, err := m.store.GetAgentRun(ctx, runtime.workspaceID, runtime.conversationID, runtime.runID)
		if err != nil {
			return
		}
		run := RunFromRecord(record)
		m.resetDraftText(runtime)
		result, err := m.provider.Generate(ctx, ProviderRequest{
			Run:                 run,
			Prompt:              runtime.prompt,
			WorkspaceRoot:       runtime.workspaceRoot,
			RepoInstructions:    runtime.repoInstructions,
			SelectedSkills:      runtime.selectedSkills,
			ContextWarnings:     runtime.contextWarnings,
			ContextSummary:      runtime.contextSummary,
			ConversationContext: runtime.conversationContext,
			History:             runtime.history,
		}, m.stream(run))
		if err != nil {
			if errors.Is(err, context.Canceled) || ctx.Err() != nil {
				return
			}
			m.fail(ctx, run, err)
			return
		}
		if strings.TrimSpace(result.Text) != "" {
			runtime.history = append(runtime.history, ProviderHistoryItem{Type: "text", Text: result.Text})
		}
		if len(result.ToolCalls) == 0 {
			now := time.Now().UTC()
			summary := strings.TrimSpace(result.Text)
			if summary != "" {
				runID := runtime.runID
				message, _ := m.store.CreateMessage(ctx, database.MessageRecord{
					WorkspaceID:    runtime.workspaceID,
					ConversationID: runtime.conversationID,
					Role:           "assistant",
					Content:        summary,
					RunID:          &runID,
					CreatedAt:      now,
				})
				if message.ID != "" {
					_ = m.publish(ctx, run, "conversation.message.created", messageEventPayload(message))
				}
			}
			m.resetDraftText(runtime)
			record, err = m.store.UpdateAgentRun(ctx, runtime.workspaceID, runtime.conversationID, runtime.runID, map[string]any{
				"status":      string(StatusDone),
				"summary":     summary,
				"finished_at": now,
			})
			if err == nil {
				_ = m.publish(ctx, RunFromRecord(record), "agent.run.status_changed", RunFromRecord(record))
			}
			m.deleteRuntime(runtime.runID)
			return
		}
		if strings.TrimSpace(result.Text) != "" {
			runID := runtime.runID
			message, _ := m.store.CreateMessage(ctx, database.MessageRecord{
				WorkspaceID:    runtime.workspaceID,
				ConversationID: runtime.conversationID,
				Role:           "assistant",
				Content:        strings.TrimSpace(result.Text),
				RunID:          &runID,
			})
			if message.ID != "" {
				_ = m.publish(ctx, run, "conversation.message.created", messageEventPayload(message))
			}
			m.resetDraftText(runtime)
		}
		waiting, results, err := m.prepareOrExecuteBatch(ctx, run, runtime, result.ToolCalls)
		if err != nil {
			m.fail(ctx, run, err)
			return
		}
		runtime.history = append(runtime.history, results...)
		if waiting {
			return
		}
	}
}

func (m *Manager) fail(ctx context.Context, run Run, failure error) {
	if errors.Is(failure, context.Canceled) || ctx.Err() != nil {
		return
	}
	message := failure.Error()
	now := time.Now().UTC()
	record, err := m.store.UpdateAgentRun(ctx, run.WorkspaceID, run.ConversationID, run.ID, map[string]any{
		"status":      string(StatusFailed),
		"error":       message,
		"finished_at": now,
	})
	if err != nil {
		return
	}
	m.deleteRuntime(run.ID)
	publicRun := RunFromRecord(record)
	_ = m.publish(ctx, publicRun, "agent.run.status_changed", publicRun)
}

func (m *Manager) publish(ctx context.Context, run Run, eventType string, payload any) error {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	record, err := m.store.CreateAgentRunEvent(ctx, database.AgentRunEventRecord{
		WorkspaceID: run.WorkspaceID,
		RunID:       run.ID,
		Type:        eventType,
		PayloadJSON: string(payloadBytes),
	})
	if err != nil {
		return err
	}
	m.events.Publish(events.Event{
		ID:          record.ID,
		WorkspaceID: run.WorkspaceID,
		Type:        eventType,
		CreatedAt:   record.CreatedAt,
		Payload:     payload,
	})
	return nil
}

func (m *Manager) stream(run Run) Stream {
	return streamFunc(func(ctx context.Context, text string) {
		if text == "" {
			return
		}
		m.appendDraftText(run.ID, text)
		m.events.Publish(events.Event{
			WorkspaceID: run.WorkspaceID,
			Type:        "agent.delta",
			Payload:     map[string]string{"runId": run.ID, "text": text},
		})
	})
}

type streamFunc func(context.Context, string)

func (s streamFunc) Delta(ctx context.Context, text string) {
	s(ctx, text)
}
