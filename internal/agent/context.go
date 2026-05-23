package agent

import (
	"strings"

	"github.com/phamtanminhtien/patchpilot/internal/database"
)

const (
	defaultContextTokenBudget = 24000
	instructionTokenReserve   = 6000
	activeRunTokenReserve     = 4000
	minRecentContextMessages  = 6
)

type ProviderMessage struct {
	Role    string
	Content string
}

type SummaryRequest struct {
	Run             Run
	ExistingSummary string
	Messages        []ProviderMessage
}

type conversationContext struct {
	Summary          string
	Messages         []ProviderMessage
	SummarizeRecords []database.MessageRecord
}

func buildConversationContext(conversation database.ConversationRecord, records []database.MessageRecord, triggerMessageID string) conversationContext {
	return buildConversationContextWithBudget(conversation, records, triggerMessageID, defaultContextTokenBudget)
}

func buildConversationContextWithBudget(conversation database.ConversationRecord, records []database.MessageRecord, triggerMessageID string, tokenBudget int) conversationContext {
	messages := messagesThroughTrigger(records, triggerMessageID)
	if len(messages) == 0 {
		return conversationContext{Summary: strings.TrimSpace(conversation.ContextSummary)}
	}

	available := tokenBudget - instructionTokenReserve - activeRunTokenReserve - estimateTokens(conversation.ContextSummary)
	if available < 1 {
		available = 1
	}

	cut := len(messages)
	used := 0
	for i := len(messages) - 1; i >= 0; i-- {
		cost := estimateMessageTokens(messages[i])
		requiredRecent := len(messages)-i <= minRecentContextMessages
		if !requiredRecent && used+cost > available {
			break
		}
		used += cost
		cut = i
	}

	contextMessages := make([]ProviderMessage, 0, len(messages)-cut)
	for _, record := range messages[cut:] {
		contextMessages = append(contextMessages, providerMessageFromRecord(record))
	}
	summarize := append([]database.MessageRecord(nil), messages[:cut]...)
	return conversationContext{
		Summary:          strings.TrimSpace(conversation.ContextSummary),
		Messages:         contextMessages,
		SummarizeRecords: summarize,
	}
}

func messagesThroughTrigger(records []database.MessageRecord, triggerMessageID string) []database.MessageRecord {
	if triggerMessageID == "" {
		return append([]database.MessageRecord(nil), records...)
	}
	for i, record := range records {
		if record.ID == triggerMessageID {
			return append([]database.MessageRecord(nil), records[:i+1]...)
		}
	}
	return append([]database.MessageRecord(nil), records...)
}

func providerMessageFromRecord(record database.MessageRecord) ProviderMessage {
	role := record.Role
	if role != "assistant" {
		role = "user"
	}
	return ProviderMessage{Role: role, Content: strings.TrimSpace(record.Content)}
}

func providerMessagesFromRecords(records []database.MessageRecord) []ProviderMessage {
	messages := make([]ProviderMessage, 0, len(records))
	for _, record := range records {
		if strings.TrimSpace(record.Content) == "" {
			continue
		}
		messages = append(messages, providerMessageFromRecord(record))
	}
	return messages
}

func estimateMessageTokens(record database.MessageRecord) int {
	return estimateTokens(record.Role) + estimateTokens(record.Content) + 8
}

func estimateTokens(text string) int {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0
	}
	return (len(text) + 3) / 4
}
