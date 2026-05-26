import type { QueryClient } from "@tanstack/react-query";

import type {
  AgentRun,
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

  queryClient.setQueryData<ConversationDetail>(
    ["conversation", workspaceId, conversation.id],
    (current) =>
      current
        ? {
            ...current,
            conversation,
          }
        : current,
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

export function updateConversationRunState(
  queryClient: QueryClient,
  workspaceId: string,
  run: AgentRun,
) {
  const detailKey = ["conversation", workspaceId, run.conversationId] as const;
  const currentDetail =
    queryClient.getQueryData<ConversationDetail>(detailKey) ?? null;
  const nextRuns =
    currentDetail === null
      ? null
      : [...currentDetail.runs.filter((item) => item.id !== run.id), run];
  const hasRunningRun =
    nextRuns === null
      ? hasActiveRunStatus(run.status)
      : nextRuns.some((item) => hasActiveRunStatus(item.status));

  queryClient.setQueryData<{ conversations: Conversation[] }>(
    ["conversations", workspaceId],
    (current) =>
      current
        ? {
            conversations: current.conversations.map((conversation) =>
              conversation.id === run.conversationId
                ? {
                    ...conversation,
                    hasRunningRun,
                  }
                : conversation,
            ),
          }
        : current,
  );

  queryClient.setQueryData<ConversationDetail>(detailKey, (current) => {
    if (!current) {
      return current;
    }
    const runs = nextRuns ?? [
      ...current.runs.filter((item) => item.id !== run.id),
      run,
    ];
    return {
      ...current,
      conversation: {
        ...current.conversation,
        hasRunningRun,
      },
      runs,
    };
  });
}

function hasActiveRunStatus(status: AgentRun["status"]) {
  return ["queued", "running", "waiting_tool_approval"].includes(status);
}
