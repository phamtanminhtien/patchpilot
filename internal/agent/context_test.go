package agent

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/phamtanminhtien/patchpilot/internal/database"
)

func TestBuildConversationContextKeepsShortHistory(t *testing.T) {
	conversation := database.ConversationRecord{ContextSummary: "Earlier summary"}
	records := []database.MessageRecord{
		messageRecord("msg_1", "user", "first"),
		messageRecord("msg_2", "assistant", "second"),
		messageRecord("msg_3", "user", "current"),
	}

	context := buildConversationContextWithBudget(conversation, records, "msg_3", 12000)

	if context.Summary != "Earlier summary" {
		t.Fatalf("expected summary to be preserved, got %q", context.Summary)
	}
	if len(context.SummarizeRecords) != 0 {
		t.Fatalf("did not expect summarization, got %+v", context.SummarizeRecords)
	}
	if len(context.Messages) != 3 || context.Messages[2].Content != "current" {
		t.Fatalf("expected all messages through trigger, got %+v", context.Messages)
	}
}

func TestBuildConversationContextSummarizesOldMessages(t *testing.T) {
	var records []database.MessageRecord
	for i := 0; i < 10; i++ {
		records = append(records, messageRecord("msg_"+strconv.Itoa(i), "user", strings.Repeat("x", 80)))
	}

	context := buildConversationContextWithBudget(database.ConversationRecord{}, records, "msg_9", 10020)

	if len(context.SummarizeRecords) == 0 {
		t.Fatalf("expected old messages to be selected for summarization")
	}
	if len(context.Messages) < minRecentContextMessages {
		t.Fatalf("expected recent messages to be preserved, got %+v", context.Messages)
	}
	if context.Messages[len(context.Messages)-1].Content != strings.Repeat("x", 80) {
		t.Fatalf("expected trigger message to stay in context, got %+v", context.Messages)
	}
}

func TestBuildConversationContextStopsAtTriggerMessage(t *testing.T) {
	records := []database.MessageRecord{
		messageRecord("msg_1", "user", "first"),
		messageRecord("msg_2", "user", "trigger"),
		messageRecord("msg_3", "assistant", "future"),
	}

	context := buildConversationContextWithBudget(database.ConversationRecord{}, records, "msg_2", 12000)

	if len(context.Messages) != 2 {
		t.Fatalf("expected messages through trigger only, got %+v", context.Messages)
	}
	if context.Messages[1].Content != "trigger" {
		t.Fatalf("expected trigger as latest message, got %+v", context.Messages)
	}
}

func messageRecord(id, role, content string) database.MessageRecord {
	return database.MessageRecord{
		ID:             id,
		WorkspaceID:    "ws_1",
		ConversationID: "conv_1",
		Role:           role,
		Content:        content,
		CreatedAt:      time.Now().UTC(),
	}
}
