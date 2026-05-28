import {
  type QueryClient,
  useMutation,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";
import { useQueryState } from "nuqs";
import { type FormEvent, useCallback, useRef, useState } from "react";

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
  getWorkspacePermissions,
  listConversations,
  listFileIndex,
  listWorkspaces,
  type Message,
  patchWorkspacePermissions,
  type PatchWorkspacePermissionsRequest,
  refreshAgentContext,
  rejectAgentToolCall,
  setAgentSkillEnabled,
  type WorkspaceEvent,
  type WorkspacePermissions,
} from "@/shared/api";
import { useRunEvents, useWorkspaceEvents } from "@/shared/events";
import { conversationIdParser, workspaceIdParser } from "@/shared/url";

import {
  updateConversationRunState,
  updateToolCallCache,
  upsertConversation,
} from "../lib/conversation-cache";
import { transientAssistantEvent } from "../lib/run-text";
import { newConversationId } from "../vibe-options";

const defaultWorkspacePermissions: WorkspacePermissions = {
  editFiles: true,
  gitOperations: true,
  mode: "balanced",
  runCommands: true,
};

export function useVibeController() {
  const [workspaceId, setWorkspaceId] = useQueryState(
    "workspaceId",
    workspaceIdParser,
  );
  const [rootPath, setRootPath] = useState("");
  const [prompt, setPrompt] = useState("");
  const promptRef = useRef("");
  const [promptResetKey, setPromptResetKey] = useState(0);
  const defaults = readAgentDefaults();
  const [model, setModel] = useState<AgentModel>(defaults.model);
  const [reasoningEffort, setReasoningEffort] = useState<AgentReasoningEffort>(
    defaults.reasoningEffort,
  );
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

  const fileIndexQuery = useQuery({
    enabled: workspaceId.length > 0,
    queryFn: () => listFileIndex(workspaceId, { limit: 100 }),
    queryKey: ["composer-file-index", workspaceId],
  });

  const permissionsQuery = useQuery({
    enabled: workspaceId.length > 0,
    queryFn: () => getWorkspacePermissions(workspaceId),
    queryKey: ["workspace-permissions", workspaceId],
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

  const permissionsMutation = useMutation({
    mutationFn: (input: PatchWorkspacePermissionsRequest) =>
      patchWorkspacePermissions(workspaceId, input),
    onMutate: async (input) => {
      await queryClient.cancelQueries({
        queryKey: ["workspace-permissions", workspaceId],
      });
      const previous = queryClient.getQueryData<WorkspacePermissions>([
        "workspace-permissions",
        workspaceId,
      ]);
      if (previous) {
        queryClient.setQueryData<WorkspacePermissions>(
          ["workspace-permissions", workspaceId],
          { ...previous, ...input },
        );
      }
      return { previous };
    },
    onError: (_error, _input, context) => {
      if (context?.previous) {
        queryClient.setQueryData(
          ["workspace-permissions", workspaceId],
          context.previous,
        );
      }
    },
    onSuccess: (permissions) =>
      queryClient.setQueryData(
        ["workspace-permissions", workspaceId],
        permissions,
      ),
  });

  const createMessageMutation = useMutation({
    mutationFn: async () => {
      const content = promptRef.current.trim();
      const conversation =
        conversationDetailQuery.data?.conversation ??
        (currentConversationId.length > 0
          ? conversationsQuery.data?.conversations.find(
              (item) => item.id === currentConversationId,
            )
          : undefined) ??
        (await createConversation(workspaceId, {}));
      const created = await createMessage(workspaceId, conversation.id, {
        content,
        model,
        reasoningEffort,
      });
      return {
        conversation: created.conversation ?? conversation,
        message: created.message,
        run: created.run,
      };
    },
    onSuccess: ({ conversation, message, run }) => {
      promptRef.current = "";
      setPrompt("");
      setPromptResetKey((current) => current + 1);
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
      if (event.type === "conversation.updated") {
        upsertConversation(
          queryClient,
          workspaceId,
          event.payload as ConversationDetail["conversation"],
        );
        return;
      }
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
      promptRef.current.trim().length === 0 ||
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
  const handlePromptChange = useCallback((nextPrompt: string) => {
    promptRef.current = nextPrompt;
  }, []);

  return {
    composer: {
      activeRun: activeRun !== undefined,
      error: error ? apiErrorMessage(error) : undefined,
      fileIndexEntries: fileIndexQuery.data?.entries ?? [],
      fileIndexError: fileIndexQuery.error
        ? apiErrorMessage(fileIndexQuery.error)
        : undefined,
      isFileIndexLoading: fileIndexQuery.isPending,
      isSkillsLoading: agentContextQuery.isPending,
      isPending: createMessageMutation.isPending,
      isStopping: cancelRunMutation.isPending,
      model,
      onModelChange: setModel,
      onPromptChange: handlePromptChange,
      onPermissionsChange: (permissions: PatchWorkspacePermissionsRequest) =>
        permissionsMutation.mutate(permissions),
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
      promptResetKey,
      permissions: permissionsQuery.data ?? defaultWorkspacePermissions,
      permissionsError:
        permissionsQuery.error || permissionsMutation.error
          ? apiErrorMessage(permissionsQuery.error ?? permissionsMutation.error)
          : undefined,
      permissionsLoading: permissionsQuery.isPending,
      permissionsSaving: permissionsMutation.isPending,
      reasoningEffort,
      skills: agentContextQuery.data?.skills ?? [],
      skillsError: agentContextQuery.error
        ? apiErrorMessage(agentContextQuery.error)
        : undefined,
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

function readAgentDefaults(): {
  model: AgentModel;
  reasoningEffort: AgentReasoningEffort;
} {
  try {
    const parsed = JSON.parse(
      globalThis.localStorage?.getItem("patchpilot.agentDefaults") ?? "{}",
    ) as Partial<{ model: AgentModel; reasoningEffort: AgentReasoningEffort }>;
    return {
      model: parsed.model ?? "gpt-5.5",
      reasoningEffort: parsed.reasoningEffort ?? "medium",
    };
  } catch {
    return { model: "gpt-5.5", reasoningEffort: "medium" };
  }
}
