import type { QueryClient } from "@tanstack/react-query";

import type {
  AgentToolCall,
  Conversation,
  ConversationDetail,
} from "@/shared/api";

export function upsertConversation(
  queryClient: QueryClient,
  workspaceId: string,
  conversation: Conversation,
) {
  queryClient.setQueryData<{ conversations: Conversation[] }>(
    ["conversations", workspaceId],
    (current) => ({
      conversations: [
        conversation,
        ...(current?.conversations.filter(
          (item) => item.id !== conversation.id,
        ) ?? []),
      ],
    }),
  );
}

export function updateToolCallCache(
  queryClient: QueryClient,
  workspaceId: string,
  conversationId: string,
  toolCall: AgentToolCall,
) {
  queryClient.setQueryData<ConversationDetail>(
    ["conversation", workspaceId, conversationId],
    (current) =>
      current
        ? {
            ...current,
            toolCalls: [
              ...current.toolCalls.filter((item) => item.id !== toolCall.id),
              toolCall,
            ],
          }
        : current,
  );
}
