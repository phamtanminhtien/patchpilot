import {
  type QueryClient,
  useMutation,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";
import {
  Check,
  ChevronDown,
  FolderOpen,
  Loader2,
  Send,
  ShieldCheck,
  Sparkles,
  X,
} from "lucide-react";
import { useQueryState } from "nuqs";
import {
  type FormEvent,
  type ReactNode,
  useEffect,
  useMemo,
  useState,
} from "react";
import { Link } from "react-router";

import { AppShell } from "@/app/app-shell";
import { useThemePreference } from "@/app/theme";
import {
  type AgentModel,
  type AgentReasoningEffort,
  type AgentTask,
  type AgentTaskDetail,
  type AgentTaskEvent,
  type AgentToolCall,
  apiErrorMessage,
  approveAgentToolCall,
  createAgentTask,
  createWorkspace,
  getAgentTask,
  getWorkspace,
  listAgentTasks,
  listWorkspaces,
  rejectAgentToolCall,
  type WorkspaceEvent,
} from "@/shared/api";
import {
  Button,
  StarterScreen,
  StatusPill,
  Surface,
  ThemeSwitcher,
} from "@/shared/ui";
import { workspaceIdParser } from "@/shared/url";

const agentModels: AgentModel[] = ["gpt-5.5", "gpt-5.4", "gpt-5.4-mini"];
const reasoningEfforts: AgentReasoningEffort[] = [
  "low",
  "medium",
  "high",
  "xhigh",
];

export function VibePage() {
  const [workspaceId, setWorkspaceId] = useQueryState(
    "workspaceId",
    workspaceIdParser,
  );
  const [rootPath, setRootPath] = useState("");
  const [prompt, setPrompt] = useState("");
  const [model, setModel] = useState<AgentModel>("gpt-5.5");
  const [reasoningEffort, setReasoningEffort] =
    useState<AgentReasoningEffort>("medium");
  const [activeTaskId, setActiveTaskId] = useState("");
  const queryClient = useQueryClient();
  const { preference, setPreference } = useThemePreference();

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

  const tasksQuery = useQuery({
    enabled: workspaceId.length > 0,
    queryFn: () => listAgentTasks(workspaceId),
    queryKey: ["agent-tasks", workspaceId],
  });

  const currentTaskId = activeTaskId || tasksQuery.data?.tasks[0]?.id || "";

  const taskDetailQuery = useQuery({
    enabled: workspaceId.length > 0 && currentTaskId.length > 0,
    queryFn: () => getAgentTask(workspaceId, currentTaskId),
    queryKey: ["agent-task", workspaceId, currentTaskId],
  });

  const createTaskMutation = useMutation({
    mutationFn: () =>
      createAgentTask(workspaceId, {
        model,
        prompt: prompt.trim(),
        reasoningEffort,
      }),
    onSuccess: (task) => {
      setPrompt("");
      setActiveTaskId(task.id);
      queryClient.setQueryData(["agent-task", workspaceId, task.id], {
        events: [],
        task,
        toolCalls: [],
      } satisfies AgentTaskDetail);
      upsertTask(queryClient, workspaceId, task);
    },
  });

  const toolApproveMutation = useMutation({
    mutationFn: (input: { taskId: string; toolCallId: string }) =>
      approveAgentToolCall(workspaceId, input.taskId, input.toolCallId),
    onSuccess: (toolCall) =>
      updateToolCallCache(queryClient, workspaceId, toolCall),
  });

  const toolRejectMutation = useMutation({
    mutationFn: (input: { taskId: string; toolCallId: string }) =>
      rejectAgentToolCall(workspaceId, input.taskId, input.toolCallId),
    onSuccess: (toolCall) =>
      updateToolCallCache(queryClient, workspaceId, toolCall),
  });

  const workspace = workspaceQuery.data;
  const error = createWorkspaceMutation.error ?? workspaceQuery.error;

  useEffect(() => {
    if (workspaceId.length === 0 || typeof EventSource === "undefined") {
      return;
    }
    const source = new EventSource(`/api/workspaces/${workspaceId}/events`, {
      withCredentials: true,
    });
    const handleAgentEvent = (message: MessageEvent<string>) => {
      const event = JSON.parse(message.data) as WorkspaceEvent;
      if (event.type === "agent.task.status_changed") {
        const task = event.payload as AgentTask;
        upsertTask(queryClient, workspaceId, task);
        queryClient.setQueryData<AgentTaskDetail>(
          ["agent-task", workspaceId, task.id],
          (current) =>
            current
              ? {
                  ...current,
                  events: appendTaskEvent(current.events, event),
                  task,
                }
              : current,
        );
        return;
      }
      if (!isAgentTaskEvent(event)) {
        return;
      }
      const taskId = eventTaskId(event);
      if (taskId.length === 0) {
        return;
      }
      queryClient.setQueryData<AgentTaskDetail>(
        ["agent-task", workspaceId, taskId],
        (current) =>
          current
            ? {
                ...current,
                events: appendTaskEvent(current.events, event),
              }
            : current,
      );
      void queryClient.invalidateQueries({
        queryKey: ["agent-task", workspaceId, taskId],
      });
    };
    source.addEventListener("agent.delta", handleAgentEvent);
    source.addEventListener("agent.tool.started", handleAgentEvent);
    source.addEventListener("agent.tool.finished", handleAgentEvent);
    source.addEventListener("agent.approval_required", handleAgentEvent);
    source.addEventListener("agent.task.status_changed", handleAgentEvent);
    return () => {
      source.close();
    };
  }, [queryClient, workspaceId]);

  const activeTask = taskDetailQuery.data?.task;
  const taskEvents = useMemo(
    () => taskDetailQuery.data?.events ?? [],
    [taskDetailQuery.data?.events],
  );
  const taskToolCalls = taskDetailQuery.data?.toolCalls ?? [];

  function handleTaskSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (
      prompt.trim().length === 0 ||
      workspace === undefined ||
      createTaskMutation.isPending
    ) {
      return;
    }
    createTaskMutation.mutate();
  }

  if (workspaceId.length === 0) {
    return (
      <StarterScreen
        createError={
          createWorkspaceMutation.error
            ? apiErrorMessage(createWorkspaceMutation.error)
            : undefined
        }
        isCreating={createWorkspaceMutation.isPending}
        isLoadingRecent={workspacesQuery.isPending}
        onRootPathChange={setRootPath}
        onSelectWorkspace={(selectedWorkspaceId) => {
          void setWorkspaceId(selectedWorkspaceId);
        }}
        onSubmit={() => createWorkspaceMutation.mutate(rootPath)}
        recentError={
          workspacesQuery.error
            ? apiErrorMessage(workspacesQuery.error)
            : undefined
        }
        recentWorkspaces={workspacesQuery.data?.workspaces ?? []}
        rootPath={rootPath}
        themeControl={
          <ThemeSwitcher onChange={setPreference} value={preference} />
        }
      />
    );
  }

  return (
    <AppShell mode="vibe" workspace={workspace} workspaceId={workspaceId}>
      <section className="grid h-[calc(100vh-2.5rem)] min-h-0 w-full overflow-hidden lg:grid-cols-[18rem_minmax(0,1fr)]">
        <aside className="bg-panel hidden min-h-0 px-3 py-3 shadow-sm lg:grid lg:grid-rows-[auto_minmax(0,1fr)_auto]">
          <VibeTaskSidebar
            activeTaskId={currentTaskId}
            isLoading={tasksQuery.isPending}
            onSelectTask={setActiveTaskId}
            tasks={tasksQuery.data?.tasks ?? []}
            workspaceName={workspace?.name}
          />

          <div className="grid gap-2">
            {workspace ? (
              <Button asChild size="compact" variant="ghost" width="full">
                <Link
                  to={`/workspace?workspaceId=${encodeURIComponent(workspace.id)}`}
                >
                  <FolderOpen aria-hidden="true" className="size-4" />
                  Open workspace
                </Link>
              </Button>
            ) : null}
          </div>
        </aside>

        <div className="grid min-h-0 min-w-0 overflow-hidden px-3 py-5 sm:px-4">
          <div className="mx-auto grid h-full min-h-0 w-full max-w-4xl grid-rows-[auto_auto_minmax(0,1fr)] gap-4 overflow-hidden">
            <div className="grid justify-items-center gap-3 text-center">
              <h1 className="text-ink text-2xl font-semibold text-balance sm:text-3xl">
                What should we build in PatchPilot?
              </h1>
            </div>

            <Surface
              className="bg-composer! gap-0 overflow-hidden shadow-md"
              layout="grid"
              padding="none"
            >
              <form className="grid" onSubmit={handleTaskSubmit}>
                <label className="sr-only" htmlFor="agent-prompt">
                  Ask AI
                </label>
                <textarea
                  className="text-ink placeholder:text-muted min-h-24 resize-none bg-transparent px-4 py-4 text-sm leading-6 transition disabled:cursor-not-allowed disabled:opacity-60"
                  disabled={!workspace}
                  id="agent-prompt"
                  onChange={(event) => setPrompt(event.target.value)}
                  placeholder="Ask PatchPilot to make a code change."
                  value={prompt}
                />
                <div className="bg-composer-bar! flex min-w-0 flex-col gap-2 px-3 py-3 sm:flex-row sm:items-center sm:justify-between">
                  <div className="flex min-w-0 flex-wrap items-center gap-2">
                    <span className="hover:bg-hover text-muted inline-flex min-h-10 min-w-0 cursor-pointer items-center gap-2 rounded-md px-3 text-xs font-medium">
                      <ShieldCheck aria-hidden="true" className="size-4" />
                      Default permissions
                      <ChevronDown aria-hidden="true" className="size-4" />
                    </span>
                    <SelectControl
                      label="Model"
                      onChange={(value) => setModel(value as AgentModel)}
                      options={agentModels}
                      value={model}
                    />
                    <SelectControl
                      label="Reasoning"
                      onChange={(value) =>
                        setReasoningEffort(value as AgentReasoningEffort)
                      }
                      options={reasoningEfforts}
                      value={reasoningEffort}
                    />
                    {workspace ? (
                      <Link
                        to={`/workspace?workspaceId=${encodeURIComponent(workspace.id)}`}
                        className="hover:bg-hover text-muted flex min-h-10 min-w-0 items-center gap-2 rounded-md px-3 text-xs font-medium"
                      >
                        <FolderOpen aria-hidden="true" className="size-4" />
                        {workspace.name}
                      </Link>
                    ) : (
                      <span className="text-warning text-sm font-medium">
                        {error
                          ? apiErrorMessage(error)
                          : "Workspace is loading."}
                      </span>
                    )}
                  </div>

                  <div className="flex min-w-0 items-center justify-between gap-2 sm:justify-end">
                    {workspace ? (
                      <StatusPill status={workspace.status} />
                    ) : null}
                    <Button
                      disabled={
                        !workspace ||
                        prompt.trim().length === 0 ||
                        createTaskMutation.isPending
                      }
                      icon={
                        createTaskMutation.isPending ? (
                          <Loader2 className="animate-spin" />
                        ) : (
                          <Send />
                        )
                      }
                      size="compact"
                    >
                      Start task
                    </Button>
                  </div>
                </div>
              </form>
            </Surface>

            <AgentTaskThread
              activeTask={activeTask}
              approvalError={
                toolApproveMutation.error
                  ? apiErrorMessage(toolApproveMutation.error)
                  : undefined
              }
              createError={
                createTaskMutation.error
                  ? apiErrorMessage(createTaskMutation.error)
                  : undefined
              }
              events={taskEvents}
              isApproving={toolApproveMutation.isPending}
              isLoading={tasksQuery.isPending || taskDetailQuery.isPending}
              isRejecting={toolRejectMutation.isPending}
              onToolApprove={(taskId, toolCallId) =>
                toolApproveMutation.mutate({ taskId, toolCallId })
              }
              onToolReject={(taskId, toolCallId) =>
                toolRejectMutation.mutate({ taskId, toolCallId })
              }
              onSelectTask={setActiveTaskId}
              rejectError={
                toolRejectMutation.error
                  ? apiErrorMessage(toolRejectMutation.error)
                  : undefined
              }
              tasks={tasksQuery.data?.tasks ?? []}
              toolCalls={taskToolCalls}
              workspaceRoot={workspace?.rootPath}
            />
          </div>
        </div>
      </section>
    </AppShell>
  );
}

function VibeTaskSidebar({
  activeTaskId,
  isLoading,
  onSelectTask,
  tasks,
  workspaceName,
}: {
  activeTaskId: string;
  isLoading: boolean;
  onSelectTask: (taskId: string) => void;
  tasks: AgentTask[];
  workspaceName?: string;
}) {
  return (
    <div className="grid min-h-0 min-w-0 grid-rows-[auto_minmax(0,1fr)] gap-3">
      <div className="grid gap-1 px-1">
        <p className="text-ink truncate text-xs font-semibold">
          {workspaceName ?? "Workspace"}
        </p>
        <p className="text-muted text-xs">Vibe task history</p>
      </div>
      <div className="min-h-0 overflow-auto">
        {tasks.length === 0 ? (
          <p className="text-muted px-1 py-2 text-xs">
            {isLoading ? "Loading tasks" : "No agent tasks yet."}
          </p>
        ) : (
          <div className="grid gap-1">
            {tasks.map((task) => (
              <button
                aria-current={task.id === activeTaskId ? "page" : undefined}
                className="hover:bg-hover aria-[current=page]:bg-hover grid min-h-12 min-w-0 gap-1 rounded-md px-2 py-2 text-left transition"
                key={task.id}
                onClick={() => onSelectTask(task.id)}
                type="button"
              >
                <span className="text-ink truncate text-xs font-semibold">
                  {task.prompt}
                </span>
                <span className="text-muted flex min-w-0 items-center justify-between gap-2 text-xs">
                  <span className="truncate">{task.model}</span>
                  <span className="shrink-0 truncate">{task.status}</span>
                </span>
              </button>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

function SelectControl({
  label,
  onChange,
  options,
  value,
}: {
  label: string;
  onChange: (value: string) => void;
  options: string[];
  value: string;
}) {
  return (
    <label className="text-muted flex min-h-10 min-w-0 items-center gap-2 rounded-md px-3 text-xs font-medium">
      <span>{label}</span>
      <select
        className="bg-hover text-ink rounded-sm px-2 py-1 text-xs"
        onChange={(event) => onChange(event.target.value)}
        value={value}
      >
        {options.map((option) => (
          <option key={option} value={option}>
            {option}
          </option>
        ))}
      </select>
    </label>
  );
}

function AgentTaskThread({
  activeTask,
  approvalError,
  createError,
  events,
  isApproving,
  isLoading,
  isRejecting,
  onToolApprove,
  onToolReject,
  onSelectTask,
  rejectError,
  tasks,
  toolCalls,
  workspaceRoot,
}: {
  activeTask?: AgentTask;
  approvalError?: string;
  createError?: string;
  events: AgentTaskEvent[];
  isApproving: boolean;
  isLoading: boolean;
  isRejecting: boolean;
  onToolApprove: (taskId: string, toolCallId: string) => void;
  onToolReject: (taskId: string, toolCallId: string) => void;
  onSelectTask: (taskId: string) => void;
  rejectError?: string;
  tasks: AgentTask[];
  toolCalls: AgentToolCall[];
  workspaceRoot?: string;
}) {
  const activeApprovalId = nextApprovalToolCall(toolCalls)?.id ?? "";
  const attachedToolCallIds = new Set<string>();
  const lastToolEventIdsByCall = latestToolEventIdsByCall(events);

  const renderToolCallReview = (toolCall: AgentToolCall) => (
    <ToolCallReview
      approvalError={approvalError}
      isApproving={isApproving}
      isCurrentApproval={toolCall.id === activeApprovalId}
      isRejecting={isRejecting}
      key={toolCall.id}
      onApprove={() => onToolApprove(toolCall.taskId, toolCall.id)}
      onReject={() => onToolReject(toolCall.taskId, toolCall.id)}
      rejectError={rejectError}
      toolCall={toolCall}
    />
  );

  return (
    <div className="grid h-full min-h-0 min-w-0 grid-rows-[minmax(0,16rem)_minmax(0,1fr)] gap-3 overflow-hidden lg:grid-cols-[16rem_minmax(0,1fr)] lg:grid-rows-1">
      <div className="bg-panel grid min-h-0 min-w-0 grid-rows-[auto_minmax(0,1fr)] overflow-hidden rounded-md shadow-sm">
        <div className="border-line min-w-0 border-b px-3 py-2">
          <p className="text-ink text-xs font-semibold">Tasks</p>
          {workspaceRoot ? (
            <p className="text-muted truncate text-xs">{workspaceRoot}</p>
          ) : null}
        </div>
        <div
          aria-label="Agent tasks"
          className="min-h-0 min-w-0 overflow-auto"
          role="region"
        >
          {tasks.length === 0 ? (
            <p className="text-muted p-3 text-xs">
              {isLoading ? "Loading tasks" : "No agent tasks yet."}
            </p>
          ) : (
            tasks.map((task) => (
              <button
                className="border-line hover:bg-hover grid min-h-14 w-full min-w-0 gap-1 border-b px-3 py-2 text-left transition"
                key={task.id}
                onClick={() => onSelectTask(task.id)}
                type="button"
              >
                <span className="text-ink truncate text-xs font-semibold">
                  {task.prompt}
                </span>
                <span className="text-muted flex max-w-full min-w-0 items-center justify-between gap-2 overflow-hidden text-xs">
                  <span className="min-w-0 truncate">
                    {task.model} · {task.reasoningEffort}
                  </span>
                  <span className="shrink-0 truncate">{task.status}</span>
                </span>
              </button>
            ))
          )}
        </div>
      </div>

      <div className="bg-panel grid min-h-0 min-w-0 grid-rows-[auto_minmax(0,1fr)] overflow-hidden rounded-md shadow-sm">
        <div className="border-line flex min-w-0 items-center justify-between gap-3 border-b px-3 py-2">
          <div className="flex min-w-0 items-center gap-2">
            <Sparkles aria-hidden="true" className="text-accent size-4" />
            <div className="min-w-0">
              <p className="text-ink truncate text-xs font-semibold">
                {activeTask?.prompt ?? "Agent activity"}
              </p>
              <p className="text-muted text-xs">
                {activeTask
                  ? `${activeTask.model} · ${activeTask.reasoningEffort}`
                  : "Create a task to watch progress here."}
              </p>
            </div>
          </div>
          {activeTask ? <StatusPill status={activeTask.status} /> : null}
        </div>

        <div className="min-h-0 overflow-auto p-3">
          {createError ? (
            <p className="text-warning text-sm font-medium">{createError}</p>
          ) : null}
          {activeTask ? (
            <div className="grid gap-3">
              {events.map((event) => {
                const toolCall = toolCallForEvent(event, toolCalls);
                const shouldAttachToolCall =
                  toolCall !== undefined &&
                  lastToolEventIdsByCall.get(toolCall.id) === event.id;
                if (shouldAttachToolCall) {
                  attachedToolCallIds.add(toolCall.id);
                }
                return (
                  <TaskEventRow event={event} key={event.id}>
                    {shouldAttachToolCall
                      ? renderToolCallReview(toolCall)
                      : null}
                  </TaskEventRow>
                );
              })}
              {toolCalls
                .filter((toolCall) => !attachedToolCallIds.has(toolCall.id))
                .map((toolCall) => renderToolCallReview(toolCall))}
              {activeTask.error ? (
                <p className="text-warning text-sm font-medium">
                  {activeTask.error}
                </p>
              ) : null}
            </div>
          ) : (
            <p className="text-muted text-sm">
              Agent task progress streams here in realtime.
            </p>
          )}
        </div>
      </div>
    </div>
  );
}

function ToolCallReview({
  approvalError,
  isApproving,
  isCurrentApproval,
  isRejecting,
  onApprove,
  onReject,
  toolCall,
  rejectError,
}: {
  approvalError?: string;
  isApproving: boolean;
  isCurrentApproval: boolean;
  isRejecting: boolean;
  onApprove: () => void;
  onReject: () => void;
  toolCall: AgentToolCall;
  rejectError?: string;
}) {
  const canDecide =
    isCurrentApproval &&
    toolCall.requiresApproval &&
    toolCall.status === "waiting_approval";
  const error = approvalError ?? rejectError;
  const input = parseToolInput(toolCall.input);
  const diff = typeof input.diff === "string" ? input.diff : "";
  const summary = typeof input.summary === "string" ? input.summary : "";

  return (
    <div className="bg-panel grid min-w-0 gap-3 rounded-md p-3 shadow-sm">
      <div className="flex min-w-0 items-center justify-between gap-3">
        <div className="min-w-0">
          <p className="text-ink truncate text-sm font-semibold">
            {toolCall.name}
          </p>
          <p className="text-muted text-xs">
            Batch {toolCall.sequence + 1} · {toolCall.status}
          </p>
        </div>
        <StatusPill status={toolCall.status} />
      </div>
      {summary ? (
        <p className="text-muted text-sm whitespace-pre-wrap">{summary}</p>
      ) : null}
      <pre className="bg-hover text-ink max-h-64 overflow-auto rounded-sm p-3 text-xs whitespace-pre-wrap">
        {diff || toolCall.output || toolCall.input}
      </pre>
      {error ? (
        <p className="text-warning text-sm font-medium">{error}</p>
      ) : null}
      {toolCall.requiresApproval &&
      (isCurrentApproval || toolCall.status !== "waiting_approval") ? (
        <div className="flex flex-wrap gap-2">
          <Button
            disabled={!canDecide || isApproving}
            icon={<Check />}
            onClick={onApprove}
            size="small"
            type="button"
          >
            Approve tool
          </Button>
          <Button
            disabled={!canDecide || isRejecting}
            icon={<X />}
            onClick={onReject}
            size="small"
            type="button"
            variant="secondary"
          >
            Reject
          </Button>
        </div>
      ) : null}
      {toolCall.requiresApproval &&
      toolCall.status === "waiting_approval" &&
      !isCurrentApproval ? (
        <p className="text-muted text-xs">
          Waiting for the previous tool decision.
        </p>
      ) : null}
    </div>
  );
}

function TaskEventRow({
  children,
  event,
}: {
  children?: ReactNode;
  event: AgentTaskEvent;
}) {
  const text = eventText(event);
  if (text.length === 0 && !children) {
    return null;
  }
  return (
    <div
      aria-label={event.type}
      className="border-line grid gap-2 border-l px-3"
      role="group"
    >
      <div className="grid gap-1">
        <p className="text-muted text-xs font-semibold">{event.type}</p>
        {text ? (
          <p className="text-ink text-sm break-words whitespace-pre-wrap">
            {text}
          </p>
        ) : null}
      </div>
      {children}
    </div>
  );
}

function eventText(event: AgentTaskEvent) {
  const payload = event.payload as Record<string, unknown>;
  if (typeof payload.text === "string") {
    return payload.text;
  }
  if (typeof payload.name === "string") {
    if (event.type === "agent.approval_required") {
      return `${payload.name} is waiting for approval.`;
    }
    return payload.name;
  }
  if (typeof payload.status === "string") {
    return payload.status;
  }
  return "";
}

function upsertTask(
  queryClient: QueryClient,
  workspaceId: string,
  task: AgentTask,
) {
  queryClient.setQueryData<{ tasks: AgentTask[] }>(
    ["agent-tasks", workspaceId],
    (current) => ({
      tasks: [
        task,
        ...(current?.tasks.filter((item) => item.id !== task.id) ?? []),
      ],
    }),
  );
}

function updateToolCallCache(
  queryClient: QueryClient,
  workspaceId: string,
  toolCall: AgentToolCall,
) {
  queryClient.setQueryData<AgentTaskDetail>(
    ["agent-task", workspaceId, toolCall.taskId],
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

function appendTaskEvent(events: AgentTaskEvent[], event: WorkspaceEvent) {
  if (events.some((item) => item.id === event.id) || !isAgentTaskEvent(event)) {
    return events;
  }
  return [
    ...events,
    {
      createdAt: event.createdAt,
      id: event.id,
      payload: event.payload,
      taskId: eventTaskId(event),
      type: event.type,
      workspaceId: event.workspaceId,
    },
  ];
}

function isAgentTaskEvent(event: WorkspaceEvent): event is WorkspaceEvent & {
  type: AgentTaskEvent["type"];
} {
  return event.type.startsWith("agent.");
}

function eventTaskId(event: WorkspaceEvent) {
  const payload = event.payload as Record<string, unknown>;
  if (typeof payload.taskId === "string") {
    return payload.taskId;
  }
  if (typeof payload.id === "string" && payload.id.startsWith("task_")) {
    return payload.id;
  }
  return "";
}

function latestToolEventIdsByCall(events: AgentTaskEvent[]) {
  const eventIds = new Map<string, string>();
  for (const event of events) {
    const toolCallId = eventToolCallId(event);
    if (toolCallId.length > 0) {
      eventIds.set(toolCallId, event.id);
    }
  }
  return eventIds;
}

function toolCallForEvent(event: AgentTaskEvent, toolCalls: AgentToolCall[]) {
  const toolCallId = eventToolCallId(event);
  if (toolCallId.length === 0) {
    return undefined;
  }
  return toolCalls.find((toolCall) => toolCall.id === toolCallId);
}

function eventToolCallId(event: AgentTaskEvent) {
  if (
    event.type !== "agent.tool.started" &&
    event.type !== "agent.tool.finished" &&
    event.type !== "agent.approval_required"
  ) {
    return "";
  }
  const payload = event.payload as Record<string, unknown>;
  return typeof payload.id === "string" ? payload.id : "";
}

function parseToolInput(input: string): Record<string, unknown> {
  try {
    const parsed = JSON.parse(input) as unknown;
    return parsed && typeof parsed === "object"
      ? (parsed as Record<string, unknown>)
      : {};
  } catch {
    return {};
  }
}

function nextApprovalToolCall(toolCalls: AgentToolCall[]) {
  return [...toolCalls]
    .filter(
      (toolCall) =>
        toolCall.requiresApproval && toolCall.status === "waiting_approval",
    )
    .sort((left, right) =>
      left.batchId === right.batchId
        ? left.sequence - right.sequence
        : left.createdAt.localeCompare(right.createdAt),
    )[0];
}
