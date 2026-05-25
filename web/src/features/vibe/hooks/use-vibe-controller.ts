import {
  type QueryClient,
  useMutation,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";
import { useQueryState } from "nuqs";
import { type FormEvent, useCallback, useState } from "react";

import {
  type AgentModel,
  type AgentReasoningEffort,
  type AgentRun,
  type AgentToolCall,
  apiErrorMessage,
  approveAgentToolCall,
  cancelAgentRun,
  type ConversationDetail,
  createConversation,
  createMessage,
  createWorkspace,
  getAgentContext,
  getConversation,
  getWorkspace,
  listConversations,
  listWorkspaces,
  type Message,
  refreshAgentContext,
  rejectAgentToolCall,
  setAgentSkillEnabled,
  type WorkspaceEvent,
} from "@/shared/api";
import { useRunEvents, useWorkspaceEvents } from "@/shared/events";
import { conversationIdParser, workspaceIdParser } from "@/shared/url";

import {
  updateConversationRunState,
  updateToolCallCache,
  upsertConversation,
} from "../lib/conversation-cache";
import { titleFromPrompt, transientAssistantEvent } from "../lib/run-text";
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
  const [transientRunText, setTransientRunText] = useState<{
    conversationId: string;
    textByRunId: Record<string, string>;
  }>({ conversationId: "", textByRunId: {} });
  const [activeConversationId, setActiveConversationId] = useQueryState(
    "conversationId",
    conversationIdParser,
  );
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
      void setActiveConversationId("");
    },
  });

  const conversationsQuery = useQuery({
    enabled: workspaceId.length > 0,
    queryFn: () => listConversations(workspaceId),
    queryKey: ["conversations", workspaceId],
  });

  const isNewConversation =
    activeConversationId.length === 0 ||
    activeConversationId === newConversationId;
  const currentConversationId = isNewConversation ? "" : activeConversationId;

  const conversationDetailQuery = useQuery({
    enabled: workspaceId.length > 0 && currentConversationId.length > 0,
    queryFn: () => getConversation(workspaceId, currentConversationId),
    queryKey: ["conversation", workspaceId, currentConversationId],
  });

  const agentContextQuery = useQuery({
    enabled: workspaceId.length > 0,
    queryFn: () => getAgentContext(workspaceId),
    queryKey: ["agent-context", workspaceId],
  });

  const refreshContextMutation = useMutation({
    mutationFn: () => refreshAgentContext(workspaceId),
    onSuccess: (context) =>
      queryClient.setQueryData(["agent-context", workspaceId], context),
  });

  const skillMutation = useMutation({
    mutationFn: (input: { key: string; enabled: boolean }) =>
      setAgentSkillEnabled(workspaceId, input.key, input.enabled),
    onSuccess: () =>
      void queryClient.invalidateQueries({
        queryKey: ["agent-context", workspaceId],
      }),
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
      void setActiveConversationId(conversation.id);
      const nextConversation = {
        ...conversation,
        hasRunningRun: isActiveRun(run),
      };
      queryClient.setQueryData<ConversationDetail>(
        ["conversation", workspaceId, conversation.id],
        (current) =>
          current
            ? {
                ...current,
                conversation: nextConversation,
                messages: [...current.messages, message],
                runs: [...current.runs, run],
              }
            : {
                conversation: nextConversation,
                events: [],
                messages: [message],
                runs: [run],
                toolCalls: [],
              },
      );
      upsertConversation(queryClient, workspaceId, nextConversation);
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

  const cancelRunMutation = useMutation({
    mutationFn: (input: { conversationId: string; runId: string }) =>
      cancelAgentRun(workspaceId, input.conversationId, input.runId),
    onSuccess: (run) =>
      updateConversationRunState(queryClient, workspaceId, run),
  });

  const workspace = workspaceQuery.data;
  const error = createWorkspaceMutation.error ?? workspaceQuery.error;
  const activeRun = latestActiveRun(conversationDetailQuery.data?.runs ?? []);

  const handleWorkspaceEvent = useCallback(
    (event: WorkspaceEvent) => {
      if (event.type === "conversation.message.created") {
        upsertMessageCache(queryClient, workspaceId, event.payload as Message);
        return;
      }
      if (event.type === "agent.run.status_changed") {
        updateConversationRunState(
          queryClient,
          workspaceId,
          event.payload as AgentRun,
        );
      }
    },
    [queryClient, workspaceId],
  );

  const handleRunEvent = useCallback(
    (event: WorkspaceEvent) => {
      if (event.type === "agent.delta") {
        const runId = runIdFromEvent(event);
        const text = textFromEvent(event);
        if (runId.length > 0 && text.length > 0) {
          setTransientRunText((current) => ({
            conversationId: currentConversationId,
            textByRunId: {
              ...(current.conversationId === currentConversationId
                ? current.textByRunId
                : {}),
              [runId]: `${
                current.conversationId === currentConversationId
                  ? (current.textByRunId[runId] ?? "")
                  : ""
              }${text}`,
            },
          }));
        }
        return;
      }
      if (event.type === "agent.output.snapshot") {
        const runId = runIdFromEvent(event);
        if (runId.length > 0) {
          setTransientRunText((current) => ({
            conversationId: currentConversationId,
            textByRunId: {
              ...(current.conversationId === currentConversationId
                ? current.textByRunId
                : {}),
              [runId]: textFromEvent(event),
            },
          }));
        }
        return;
      }
      if (event.type === "agent.run.status_changed") {
        const run = event.payload as AgentRun;
        updateConversationRunState(queryClient, workspaceId, run);
        if (isTerminalRun(run)) {
          setTransientRunText((current) =>
            omitRunText(current, currentConversationId, run.id),
          );
        }
        return;
      }
      if (event.type === "conversation.message.created") {
        const message = event.payload as Message;
        upsertMessageCache(queryClient, workspaceId, message);
        const messageRunId = message.runId ?? "";
        if (message.role === "assistant" && messageRunId.length > 0) {
          setTransientRunText((current) =>
            omitRunText(current, currentConversationId, messageRunId),
          );
        }
        return;
      }
      if (
        event.type === "agent.tool.started" ||
        event.type === "agent.tool.finished" ||
        event.type === "agent.approval_required"
      ) {
        upsertToolCallCache(
          queryClient,
          workspaceId,
          currentConversationId,
          event.payload as AgentToolCall,
        );
      }
    },
    [currentConversationId, queryClient, workspaceId],
  );

  useWorkspaceEvents(workspaceId, handleWorkspaceEvent);
  useRunEvents(
    workspaceId,
    currentConversationId,
    activeRun?.id ?? "",
    handleRunEvent,
  );

  function handleTaskSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (
      prompt.trim().length === 0 ||
      workspace === undefined ||
      createMessageMutation.isPending ||
      activeRun !== undefined
    ) {
      return;
    }
    createMessageMutation.mutate();
  }

  const hasConversation = currentConversationId.length > 0;
  const contextError =
    agentContextQuery.error ??
    refreshContextMutation.error ??
    skillMutation.error;

  return {
    composer: {
      activeRun: activeRun !== undefined,
      error: error ? apiErrorMessage(error) : undefined,
      isPending: createMessageMutation.isPending,
      isStopping: cancelRunMutation.isPending,
      model,
      onModelChange: setModel,
      onPromptChange: setPrompt,
      onReasoningEffortChange: setReasoningEffort,
      onStop: () => {
        if (activeRun === undefined || currentConversationId.length === 0) {
          return;
        }
        cancelRunMutation.mutate({
          conversationId: currentConversationId,
          runId: activeRun.id,
        });
      },
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
      onNewConversation: () => {
        void setActiveConversationId("");
      },
      onSelectConversation: (conversationId: string) => {
        void setActiveConversationId(conversationId);
      },
      title:
        conversationDetailQuery.data?.conversation.title ??
        "PatchPilot conversation",
    },
    context: {
      data: agentContextQuery.data,
      error: contextError ? apiErrorMessage(contextError) : undefined,
      isLoading: agentContextQuery.isPending,
      isRefreshing: refreshContextMutation.isPending,
      isUpdatingSkill: skillMutation.isPending,
      onRefresh: () => refreshContextMutation.mutate(),
      onSkillEnabledChange: (key: string, enabled: boolean) =>
        skillMutation.mutate({ key, enabled }),
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
        void setActiveConversationId("");
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
        : cancelRunMutation.error
          ? apiErrorMessage(cancelRunMutation.error)
          : undefined,
      events: [
        ...(conversationDetailQuery.data?.events ?? []),
        ...(transientRunText.conversationId === currentConversationId
          ? Object.entries(transientRunText.textByRunId).map(([runId, text]) =>
              transientAssistantEvent(runId, text),
            )
          : []),
      ],
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

function runIdFromEvent(event: WorkspaceEvent) {
  const payload = event.payload as Record<string, unknown>;
  return typeof payload.runId === "string" ? payload.runId : "";
}

function textFromEvent(event: WorkspaceEvent) {
  const payload = event.payload as Record<string, unknown>;
  return typeof payload.text === "string" ? payload.text : "";
}

function latestActiveRun(runs: AgentRun[]) {
  return [...runs]
    .filter((run) => isActiveRun(run))
    .sort(
      (first, second) =>
        second.createdAt.localeCompare(first.createdAt) ||
        second.id.localeCompare(first.id),
    )[0];
}

function isActiveRun(run: AgentRun) {
  return ["queued", "running", "waiting_tool_approval"].includes(run.status);
}

function isTerminalRun(run: AgentRun) {
  return ["done", "failed", "canceled"].includes(run.status);
}

function omitRunText(
  current: {
    conversationId: string;
    textByRunId: Record<string, string>;
  },
  conversationId: string,
  runId: string,
) {
  if (
    current.conversationId !== conversationId ||
    !(runId in current.textByRunId)
  ) {
    return current;
  }
  const next = { ...current.textByRunId };
  delete next[runId];
  return { conversationId, textByRunId: next };
}

function upsertMessageCache(
  queryClient: QueryClient,
  workspaceId: string,
  message: Message,
) {
  queryClient.setQueryData<ConversationDetail>(
    ["conversation", workspaceId, message.conversationId],
    (current) =>
      current
        ? {
            ...current,
            messages: [
              ...current.messages.filter((item) => item.id !== message.id),
              message,
            ],
          }
        : current,
  );
}

function upsertToolCallCache(
  queryClient: QueryClient,
  workspaceId: string,
  conversationId: string,
  toolCall: AgentToolCall,
) {
  if (conversationId.length > 0) {
    updateToolCallCache(queryClient, workspaceId, conversationId, toolCall);
  }
}
