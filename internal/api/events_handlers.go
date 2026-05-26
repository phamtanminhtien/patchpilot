package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/phamtanminhtien/patchpilot/internal/agent"
	"github.com/phamtanminhtien/patchpilot/internal/events"
)

func (s *Server) workspaceEvents(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.workspaceFromRequest(w, r)
	if !ok {
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "sse_unsupported", "Streaming is unavailable", nil)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher.Flush()
	events, unsubscribe := s.events.Subscribe()
	defer unsubscribe()
	for {
		select {
		case <-r.Context().Done():
			return
		case event := <-events:
			if event.WorkspaceID != ws.ID {
				continue
			}
			if isRunEventType(event.Type) {
				continue
			}
			writeSSE(w, event)
			flusher.Flush()
		}
	}
}

func (s *Server) agentRunEvents(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.workspaceFromRequest(w, r)
	if !ok {
		return
	}
	conversationID := r.PathValue("conversationId")
	runID := r.PathValue("runId")
	if _, err := s.store.GetAgentRun(r.Context(), ws.ID, conversationID, runID); err != nil {
		writeAgentError(w, agent.ErrRunNotFound)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "sse_unsupported", "Streaming is unavailable", nil)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	lastEventID := strings.TrimSpace(r.Header.Get("Last-Event-ID"))
	if err := s.replayAgentRunEvents(r.Context(), w, ws.ID, runID, lastEventID); err != nil {
		writeError(w, http.StatusInternalServerError, "event_replay_failed", "Run events could not be replayed", nil)
		return
	}
	s.writeAgentOutputSnapshot(w, ws.ID, runID)
	flusher.Flush()

	events, unsubscribe := s.events.Subscribe()
	defer unsubscribe()
	for {
		select {
		case <-r.Context().Done():
			return
		case event := <-events:
			if event.WorkspaceID != ws.ID || !isRunEventType(event.Type) || liveEventRunID(event) != runID {
				continue
			}
			writeSSE(w, event)
			flusher.Flush()
		}
	}
}

func (s *Server) replayAgentRunEvents(ctx context.Context, w http.ResponseWriter, workspaceID, runID, lastEventID string) error {
	records, err := s.store.ListAgentRunEvents(ctx, workspaceID, runID)
	if err != nil {
		return err
	}
	afterLastEvent := lastEventID == ""
	for _, record := range records {
		if record.Type == "agent.delta" || record.Type == "agent.output.snapshot" {
			continue
		}
		if !afterLastEvent {
			if record.ID == lastEventID {
				afterLastEvent = true
			}
			continue
		}
		writeSSE(w, events.Event{
			ID:          record.ID,
			WorkspaceID: record.WorkspaceID,
			Type:        record.Type,
			CreatedAt:   record.CreatedAt,
			Payload:     json.RawMessage(record.PayloadJSON),
		})
	}
	return nil
}

func (s *Server) writeAgentOutputSnapshot(w http.ResponseWriter, workspaceID, runID string) {
	if s.agent == nil {
		return
	}
	text := s.agent.DraftText(runID)
	if strings.TrimSpace(text) == "" {
		return
	}
	writeSSE(w, events.Event{
		WorkspaceID: workspaceID,
		Type:        "agent.output.snapshot",
		Payload:     map[string]string{"runId": runID, "text": text},
	})
}

func writeSSE(w http.ResponseWriter, event events.Event) {
	if event.ID == "" {
		event = events.Event{
			ID:          "evt_inline",
			WorkspaceID: event.WorkspaceID,
			Type:        event.Type,
			CreatedAt:   event.CreatedAt,
			Payload:     event.Payload,
		}
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}
	data, err := json.Marshal(event)
	if err != nil {
		return
	}
	_, _ = fmt.Fprintf(w, "id: %s\n", event.ID)
	_, _ = fmt.Fprintf(w, "event: %s\n", event.Type)
	_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
}

func isRunEventType(eventType string) bool {
	switch eventType {
	case "agent.delta", "agent.output.snapshot", "agent.tool.started", "agent.tool.finished", "agent.approval_required", "agent.run.status_changed", "conversation.message.created":
		return true
	default:
		return false
	}
}

func liveEventRunID(event events.Event) string {
	switch payload := event.Payload.(type) {
	case agent.Run:
		return payload.ID
	case agent.ToolCall:
		return payload.RunID
	case map[string]string:
		return payload["runId"]
	case map[string]any:
		if runID, ok := payload["runId"].(string); ok {
			return runID
		}
	}
	payloadBytes, err := json.Marshal(event.Payload)
	if err != nil {
		return ""
	}
	var payload struct {
		ID    string `json:"id"`
		RunID string `json:"runId"`
	}
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return ""
	}
	if payload.RunID != "" {
		return payload.RunID
	}
	if event.Type == "agent.run.status_changed" {
		return payload.ID
	}
	return ""
}
