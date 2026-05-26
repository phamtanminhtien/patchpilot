package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/phamtanminhtien/patchpilot/internal/agent"
	"github.com/phamtanminhtien/patchpilot/internal/database"
	"github.com/phamtanminhtien/patchpilot/internal/events"
)

type conversationRequest struct {
	Title string `json:"title"`
}

type createMessageRequest struct {
	Content         string `json:"content"`
	Model           string `json:"model"`
	ReasoningEffort string `json:"reasoningEffort"`
}

type patchSkillRequest struct {
	Enabled bool `json:"enabled"`
}

func (s *Server) createConversation(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.workspaceFromRequest(w, r)
	if !ok {
		return
	}
	var req conversationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Request body must be valid JSON", nil)
		return
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = defaultConversationTitle
	}
	conversation, err := s.store.CreateConversation(r.Context(), database.ConversationRecord{
		WorkspaceID: ws.ID,
		Title:       title,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "conversation_create_failed", "Conversation could not be created", nil)
		return
	}
	writeJSON(w, http.StatusCreated, conversationResponseFromRecord(conversation))
}

func (s *Server) listConversations(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.workspaceFromRequest(w, r)
	if !ok {
		return
	}
	pagination, ok := paginationFromRequest(w, r)
	if !ok {
		return
	}
	conversations, err := s.store.ListConversations(
		r.Context(),
		ws.ID,
		r.URL.Query().Get("q"),
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "conversation_list_failed", "Conversations could not be listed", nil)
		return
	}
	out := make([]conversationResponse, 0, len(conversations))
	for _, conversation := range conversations {
		out = append(out, conversationResponseFromRecord(conversation))
	}
	page, nextCursor, ok := paginateItems(w, out, pagination, func(conversation conversationResponse) string {
		return conversation.ID
	})
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"conversations": page, "nextCursor": nextCursor})
}

func (s *Server) getConversation(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.workspaceFromRequest(w, r)
	if !ok {
		return
	}
	detail, err := s.conversationDetail(r.Context(), ws.ID, r.PathValue("conversationId"))
	if err != nil {
		writeConversationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (s *Server) updateConversation(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.workspaceFromRequest(w, r)
	if !ok {
		return
	}
	var req conversationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Request body must be valid JSON", nil)
		return
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		writeError(w, http.StatusBadRequest, "invalid_conversation_title", "Conversation title is required", nil)
		return
	}
	conversation, err := s.store.UpdateConversation(r.Context(), ws.ID, r.PathValue("conversationId"), map[string]any{"title": title})
	if err != nil {
		writeConversationError(w, err)
		return
	}
	response := conversationResponseFromRecord(conversation)
	s.publishConversationUpdated(ws.ID, response)
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) createMessage(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.workspaceFromRequest(w, r)
	if !ok {
		return
	}
	if s.agent == nil {
		writeError(w, http.StatusServiceUnavailable, "agent_unavailable", "Agent runtime is unavailable", nil)
		return
	}
	conversationID := r.PathValue("conversationId")
	conversation, err := s.store.GetConversation(r.Context(), ws.ID, conversationID)
	if err != nil {
		writeConversationError(w, err)
		return
	}
	shouldGenerateTitle := isDefaultConversationTitle(conversation.Title)
	if shouldGenerateTitle {
		messages, err := s.store.ListMessages(r.Context(), ws.ID, conversationID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "message_list_failed", "Conversation messages could not be listed", nil)
			return
		}
		shouldGenerateTitle = len(messages) == 0
	}
	var req createMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Request body must be valid JSON", nil)
		return
	}
	message, err := s.store.CreateMessage(r.Context(), database.MessageRecord{
		WorkspaceID:    ws.ID,
		ConversationID: conversationID,
		Role:           "user",
		Content:        strings.TrimSpace(req.Content),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "message_create_failed", "Message could not be created", nil)
		return
	}
	run, err := s.agent.Create(r.Context(), ws.ID, ws.RootPath, agent.CreateRunInput{
		Prompt:           message.Content,
		ConversationID:   conversationID,
		TriggerMessageID: message.ID,
		Model:            req.Model,
		ReasoningEffort:  req.ReasoningEffort,
	})
	if err != nil {
		writeAgentError(w, err)
		return
	}
	updatedMessage, err := s.store.UpdateMessageRun(r.Context(), ws.ID, conversationID, message.ID, run.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "message_update_failed", "Message could not be linked to the run", nil)
		return
	}
	if shouldGenerateTitle {
		s.generateConversationTitleAsync(ws.ID, conversationID, message.Content)
	}
	writeJSON(w, http.StatusAccepted, map[string]any{
		"conversation": conversationResponseFromRecord(conversation),
		"message":      messageResponseFromRecord(updatedMessage),
		"run":          run,
	})
}

func (s *Server) cancelAgentRun(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.workspaceFromRequest(w, r)
	if !ok {
		return
	}
	if s.agent == nil {
		writeError(w, http.StatusServiceUnavailable, "agent_unavailable", "Agent runtime is unavailable", nil)
		return
	}
	run, err := s.agent.Cancel(r.Context(), ws.ID, r.PathValue("conversationId"), r.PathValue("runId"))
	if err != nil {
		writeAgentError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, run)
}

func (s *Server) approveAgentToolCall(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.workspaceFromRequest(w, r)
	if !ok {
		return
	}
	if s.agent == nil {
		writeError(w, http.StatusServiceUnavailable, "agent_unavailable", "Agent runtime is unavailable", nil)
		return
	}
	call, err := s.agent.ApproveToolCall(r.Context(), ws.ID, r.PathValue("runId"), r.PathValue("toolCallId"))
	if err != nil {
		writeAgentError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"toolCall": call})
}

func (s *Server) rejectAgentToolCall(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.workspaceFromRequest(w, r)
	if !ok {
		return
	}
	if s.agent == nil {
		writeError(w, http.StatusServiceUnavailable, "agent_unavailable", "Agent runtime is unavailable", nil)
		return
	}
	call, err := s.agent.RejectToolCall(r.Context(), ws.ID, r.PathValue("runId"), r.PathValue("toolCallId"))
	if err != nil {
		writeAgentError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"toolCall": call})
}

func (s *Server) getAgentContext(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.workspaceFromRequest(w, r)
	if !ok {
		return
	}
	if s.agent == nil {
		writeError(w, http.StatusServiceUnavailable, "agent_unavailable", "Agent runtime is unavailable", nil)
		return
	}
	snapshot, err := s.agent.RefreshContext(r.Context(), ws.RootPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "agent_context_refresh_failed", "Agent context could not be refreshed", nil)
		return
	}
	writeJSON(w, http.StatusOK, snapshot)
}

func (s *Server) patchAgentSkill(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.workspaceFromRequest(w, r)
	if !ok {
		return
	}
	_ = ws
	var req patchSkillRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Request body must be valid JSON", nil)
		return
	}
	skill, err := s.agent.SetSkillEnabled(r.Context(), r.PathValue("skillKey"), req.Enabled)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "skill_config_failed", "Skill config could not be updated", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"skill": skill})
}

func (s *Server) listAgentSkills(w http.ResponseWriter, r *http.Request) {
	snapshot, ok := s.agentContextFromRequest(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"skills": snapshot.Skills})
}

func (s *Server) listMCPServers(w http.ResponseWriter, r *http.Request) {
	snapshot, ok := s.agentContextFromRequest(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"servers": snapshot.MCPServers})
}

func (s *Server) listMCPServerTools(w http.ResponseWriter, r *http.Request) {
	snapshot, ok := s.agentContextFromRequest(w, r)
	if !ok {
		return
	}
	serverID := r.PathValue("serverId")
	tools := make([]any, 0)
	for _, tool := range snapshot.MCPTools {
		if tool.ServerID == serverID {
			tools = append(tools, tool)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"tools": tools})
}

func (s *Server) agentContextFromRequest(w http.ResponseWriter, r *http.Request) (agent.ContextSnapshot, bool) {
	ws, ok := s.workspaceFromRequest(w, r)
	if !ok {
		return agent.ContextSnapshot{}, false
	}
	if s.agent == nil {
		writeError(w, http.StatusServiceUnavailable, "agent_unavailable", "Agent runtime is unavailable", nil)
		return agent.ContextSnapshot{}, false
	}
	snapshot, err := s.agent.RefreshContext(r.Context(), ws.RootPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "agent_context_refresh_failed", "Agent context could not be refreshed", nil)
		return agent.ContextSnapshot{}, false
	}
	return snapshot, true
}

func (s *Server) conversationDetail(ctx context.Context, workspaceID, conversationID string) (map[string]any, error) {
	conversation, err := s.store.GetConversation(ctx, workspaceID, conversationID)
	if err != nil {
		return nil, err
	}
	messages, err := s.store.ListMessages(ctx, workspaceID, conversationID)
	if err != nil {
		return nil, err
	}
	runs, err := s.agent.List(ctx, workspaceID, conversationID)
	if err != nil {
		return nil, err
	}
	messageResponses := make([]messageResponse, 0, len(messages))
	for _, message := range messages {
		messageResponses = append(messageResponses, messageResponseFromRecord(message))
	}
	toolCalls := make([]agent.ToolCall, 0)
	runEvents := make([]agent.RunEvent, 0)
	for _, run := range runs {
		records, err := s.store.ListAgentToolCalls(ctx, workspaceID, run.ID)
		if err != nil {
			return nil, err
		}
		toolCalls = append(toolCalls, agent.ToolCallsFromRecords(records)...)
		if run.Status == string(agent.StatusDone) || run.Status == string(agent.StatusFailed) {
			continue
		}
		eventRecords, err := s.store.ListAgentRunEvents(ctx, workspaceID, run.ID)
		if err != nil {
			return nil, err
		}
		runEvents = append(runEvents, agent.EventsFromRecords(eventRecords)...)
	}
	return map[string]any{
		"conversation": conversationResponseFromRecord(conversation),
		"events":       runEvents,
		"messages":     messageResponses,
		"runs":         runs,
		"toolCalls":    toolCalls,
	}, nil
}

func (s *Server) generateConversationTitleAsync(workspaceID, conversationID, prompt string) {
	if s.agent == nil {
		return
	}
	lightModel := strings.TrimSpace(s.lightModel)
	if lightModel == "" {
		lightModel = defaultLightModel
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), titleGenerationTimeout)
		defer cancel()
		title, err := s.agent.GenerateTitle(ctx, prompt, lightModel)
		if err != nil {
			return
		}
		title = strings.TrimSpace(title)
		if title == "" {
			return
		}
		conversation, err := s.store.GetConversation(ctx, workspaceID, conversationID)
		if err != nil || !isDefaultConversationTitle(conversation.Title) {
			return
		}
		updated, err := s.store.UpdateConversation(ctx, workspaceID, conversationID, map[string]any{"title": title})
		if err != nil {
			return
		}
		s.publishConversationUpdated(workspaceID, conversationResponseFromRecord(updated))
	}()
}

func (s *Server) publishConversationUpdated(workspaceID string, conversation conversationResponse) {
	s.events.Publish(events.Event{
		WorkspaceID: workspaceID,
		Type:        "conversation.updated",
		Payload:     conversation,
	})
}

func isDefaultConversationTitle(title string) bool {
	return strings.TrimSpace(title) == defaultConversationTitle
}
