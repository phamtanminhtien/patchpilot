import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useQueryState } from "nuqs";
import {
  type FormEvent,
  useCallback,
  useEffect,
  useRef,
  useState,
} from "react";

import {
  type AgentModel,
  type AgentReasoningEffort,
  type AgentRun,
  apiErrorMessage,
  approveAgentToolCall,
  type ConversationDetail,
  createConversation,
  createMessage,
  createWorkspace,
  getConversation,
  getWorkspace,
  listConversations,
  listWorkspaces,
  rejectAgentToolCall,
  type WorkspaceEvent,
} from "@/shared/api";
import { useWorkspaceEvents } from "@/shared/events";
import { workspaceIdParser } from "@/shared/url";

import {
  updateToolCallCache,
  upsertConversation,
} from "../lib/conversation-cache";
import { appendRunEvent, eventRunId, isAgentRunEvent } from "../lib/events";
import { titleFromPrompt } from "../lib/run-text";
import { newConversationId } from "../vibe-options";

export function useVibeController() {
  const [workspaceId, setWorkspaceId] = useQueryState(
    "workspaceId",
    workspaceIdParser,
  );
  const [rootPath, setRootPath] = useState("");
  const [prompt, setPrompt] = useState("");
  const [model, setModel] = useState<AgentModel>("gpt-5.5");
  const [reasoningEffort, setReasoningEffort] =
    useState<AgentReasoningEffort>("medium");
  const [activeConversationId, setActiveConversationId] =
    useState(newConversationId);
  const queryClient = useQueryClient();

  const workspaceQuery = useQuery({
    enabled: workspaceId.length > 0,
    queryFn: () => getWorkspace(workspaceId),
    queryKey: ["workspace", workspaceId],
  });

  const workspacesQuery = useQuery({
    enabled: workspaceId.length === 0,
    queryFn: listWorkspaces,
    queryKey: ["workspaces"],
  });

  const createWorkspaceMutation = useMutation({
    mutationFn: createWorkspace,
    onSuccess: (workspace) => {
      void setWorkspaceId(workspace.id);
    },
  });

  const conversationsQuery = useQuery({
    enabled: workspaceId.length > 0,
    queryFn: () => listConversations(workspaceId),
    queryKey: ["conversations", workspaceId],
  });

  const isNewConversation = activeConversationId === newConversationId;
  const currentConversationId = isNewConversation ? "" : activeConversationId;
  const currentConversationIdRef = useRef(currentConversationId);

  useEffect(() => {
    currentConversationIdRef.current = currentConversationId;
  }, [currentConversationId]);

  const conversationDetailQuery = useQuery({
    enabled: workspaceId.length > 0 && currentConversationId.length > 0,
    queryFn: () => getConversation(workspaceId, currentConversationId),
    queryKey: ["conversation", workspaceId, currentConversationId],
  });

  const createMessageMutation = useMutation({
    mutationFn: async () => {
      const content = prompt.trim();
      const conversation =
        conversationDetailQuery.data?.conversation ??
        (currentConversationId.length > 0
          ? conversationsQuery.data?.conversations.find(
              (item) => item.id === currentConversationId,
            )
          : undefined) ??
        (await createConversation(workspaceId, {
          title: titleFromPrompt(content),
        }));
      const created = await createMessage(workspaceId, conversation.id, {
        content,
        model,
        reasoningEffort,
      });
      return { conversation, message: created.message, run: created.run };
    },
    onSuccess: ({ conversation, message, run }) => {
      setPrompt("");
      setActiveConversationId(conversation.id);
      queryClient.setQueryData<ConversationDetail>(
        ["conversation", workspaceId, conversation.id],
        (current) =>
          current
            ? {
                ...current,
                messages: [...current.messages, message],
                runs: [...current.runs, run],
              }
            : {
                conversation,
                events: [],
                messages: [message],
                runs: [run],
                toolCalls: [],
              },
      );
      upsertConversation(queryClient, workspaceId, conversation);
    },
  });

  const toolApproveMutation = useMutation({
    mutationFn: (input: {
      conversationId: string;
      runId: string;
      toolCallId: string;
    }) =>
      approveAgentToolCall(
        workspaceId,
        input.conversationId,
        input.runId,
        input.toolCallId,
      ),
    onSuccess: (toolCall, input) =>
      updateToolCallCache(
        queryClient,
        workspaceId,
        input.conversationId,
        toolCall,
      ),
  });

  const toolRejectMutation = useMutation({
    mutationFn: (input: {
      conversationId: string;
      runId: string;
      toolCallId: string;
    }) =>
      rejectAgentToolCall(
        workspaceId,
        input.conversationId,
        input.runId,
        input.toolCallId,
      ),
    onSuccess: (toolCall, input) =>
      updateToolCallCache(
        queryClient,
        workspaceId,
        input.conversationId,
        toolCall,
      ),
  });

  const workspace = workspaceQuery.data;
  const error = createWorkspaceMutation.error ?? workspaceQuery.error;

  const handleAgentEvent = useCallback(
    (event: WorkspaceEvent) => {
      if (event.type === "agent.run.status_changed") {
        const run = event.payload as AgentRun;
        queryClient.setQueryData<ConversationDetail>(
          ["conversation", workspaceId, run.conversationId],
          (current) =>
            current
              ? {
                  ...current,
                  events: appendRunEvent(current.events, event),
                  runs: [
                    ...current.runs.filter((item) => item.id !== run.id),
                    run,
                  ],
                }
              : current,
        );
        return;
      }
      if (!isAgentRunEvent(event)) {
        return;
      }
      const runId = eventRunId(event);
      if (runId.length === 0) {
        return;
      }
      const eventConversationId = currentConversationIdRef.current;
      if (eventConversationId.length === 0) {
        return;
      }
      queryClient.setQueryData<ConversationDetail>(
        ["conversation", workspaceId, eventConversationId],
        (current) =>
          current
            ? {
                ...current,
                events: appendRunEvent(current.events, event),
              }
            : current,
      );
      void queryClient.invalidateQueries({
        queryKey: ["conversation", workspaceId, eventConversationId],
      });
    },
    [queryClient, workspaceId],
  );

  useWorkspaceEvents(workspaceId, handleAgentEvent);

  function handleTaskSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (
      prompt.trim().length === 0 ||
      workspace === undefined ||
      createMessageMutation.isPending
    ) {
      return;
    }
    createMessageMutation.mutate();
  }

  const hasConversation = currentConversationId.length > 0;

  return {
    composer: {
      error: error ? apiErrorMessage(error) : undefined,
      isPending: createMessageMutation.isPending,
      model,
      onModelChange: setModel,
      onPromptChange: setPrompt,
      onReasoningEffortChange: setReasoningEffort,
      onSubmit: handleTaskSubmit,
      prompt,
      reasoningEffort,
      workspaceReady: workspace !== undefined,
    },
    conversation: {
      activeConversationId: currentConversationId,
      conversations: conversationsQuery.data?.conversations ?? [],
      hasConversation,
      isLoading:
        conversationsQuery.isPending || conversationDetailQuery.isPending,
      isListLoading: conversationsQuery.isPending,
      onNewConversation: () => setActiveConversationId(newConversationId),
      onSelectConversation: setActiveConversationId,
      title:
        conversationDetailQuery.data?.conversation.title ??
        "PatchPilot conversation",
    },
    starter: {
      createError: createWorkspaceMutation.error
        ? apiErrorMessage(createWorkspaceMutation.error)
        : undefined,
      isCreating: createWorkspaceMutation.isPending,
      isLoadingRecent: workspacesQuery.isPending,
      onRootPathChange: setRootPath,
      onSelectWorkspace: (selectedWorkspaceId: string) => {
        void setWorkspaceId(selectedWorkspaceId);
      },
      onSubmit: () => createWorkspaceMutation.mutate(rootPath),
      recentError: workspacesQuery.error
        ? apiErrorMessage(workspacesQuery.error)
        : undefined,
      recentWorkspaces: workspacesQuery.data?.workspaces ?? [],
      rootPath,
    },
    thread: {
      approvalError: toolApproveMutation.error
        ? apiErrorMessage(toolApproveMutation.error)
        : undefined,
      createError: createMessageMutation.error
        ? apiErrorMessage(createMessageMutation.error)
        : undefined,
      events: conversationDetailQuery.data?.events ?? [],
      isApproving: toolApproveMutation.isPending,
      isRejecting: toolRejectMutation.isPending,
      messages: conversationDetailQuery.data?.messages ?? [],
      onToolApprove: (runId: string, toolCallId: string) =>
        toolApproveMutation.mutate({
          conversationId: currentConversationId,
          runId,
          toolCallId,
        }),
      onToolReject: (runId: string, toolCallId: string) =>
        toolRejectMutation.mutate({
          conversationId: currentConversationId,
          runId,
          toolCallId,
        }),
      rejectError: toolRejectMutation.error
        ? apiErrorMessage(toolRejectMutation.error)
        : undefined,
      runs: conversationDetailQuery.data?.runs ?? [],
      toolCalls: conversationDetailQuery.data?.toolCalls ?? [],
    },
    workspace: {
      data: workspace,
      id: workspaceId,
    },
  };
}
