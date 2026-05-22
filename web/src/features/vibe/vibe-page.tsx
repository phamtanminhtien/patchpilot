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
  MessageSquare,
  Plus,
  Search,
  Send,
  ShieldCheck,
  Sparkles,
  Undo2,
  X,
} from "lucide-react";
import { useQueryState } from "nuqs";
import { type FormEvent, useEffect, useMemo, useState } from "react";
import { Link } from "react-router";

import { AppShell } from "@/app/app-shell";
import { useThemePreference } from "@/app/theme";
import {
  type AgentModel,
  type AgentPatch,
  type AgentReasoningEffort,
  type AgentTask,
  type AgentTaskDetail,
  type AgentTaskEvent,
  apiErrorMessage,
  applyPatch,
  createAgentTask,
  createWorkspace,
  getAgentTask,
  getWorkspace,
  listAgentTasks,
  listWorkspaces,
  rejectPatch,
  revertPatch,
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
        patches: [],
        task,
        toolCalls: [],
      } satisfies AgentTaskDetail);
      upsertTask(queryClient, workspaceId, task);
    },
  });

  const patchApplyMutation = useMutation({
    mutationFn: (patchId: string) => applyPatch(workspaceId, patchId),
    onSuccess: (patch) => updatePatchCache(queryClient, workspaceId, patch),
  });

  const patchRejectMutation = useMutation({
    mutationFn: (patchId: string) => rejectPatch(workspaceId, patchId),
    onSuccess: (patch) => updatePatchCache(queryClient, workspaceId, patch),
  });

  const patchRevertMutation = useMutation({
    mutationFn: (patchId: string) => revertPatch(workspaceId, patchId),
    onSuccess: (patch) => updatePatchCache(queryClient, workspaceId, patch),
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
    source.addEventListener("patch.created", handleAgentEvent);
    source.addEventListener("patch.applied", handleAgentEvent);
    source.addEventListener("patch.rejected", handleAgentEvent);
    source.addEventListener("patch.reverted", handleAgentEvent);
    return () => {
      source.close();
    };
  }, [queryClient, workspaceId]);

  const activeTask = taskDetailQuery.data?.task;
  const taskEvents = useMemo(
    () => taskDetailQuery.data?.events ?? [],
    [taskDetailQuery.data?.events],
  );
  const taskPatches = taskDetailQuery.data?.patches ?? [];

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
      <section className="grid min-h-[calc(100vh-2.5rem)] w-full overflow-hidden lg:grid-cols-[18rem_minmax(0,1fr)]">
        <aside className="bg-panel hidden min-h-0 px-3 py-3 shadow-sm lg:grid lg:grid-rows-[auto_minmax(0,1fr)_auto]">
          <div className="grid gap-1.5">
            <button
              className="text-ink flex min-h-9 min-w-0 items-center gap-2 rounded-md px-2 text-left text-sm font-medium transition disabled:cursor-not-allowed disabled:opacity-55"
              disabled
              type="button"
            >
              <Plus aria-hidden="true" className="size-4 shrink-0" />
              <span className="truncate">New chat</span>
            </button>
            <button
              className="text-muted flex min-h-9 min-w-0 items-center gap-2 rounded-md px-2 text-left text-sm font-medium transition disabled:cursor-not-allowed disabled:opacity-55"
              disabled
              type="button"
            >
              <Search aria-hidden="true" className="size-4 shrink-0" />
              <span className="truncate">Search</span>
            </button>
          </div>

          <div className="min-h-0 overflow-auto py-5">
            <div className="grid gap-5">
              <ConversationGroup
                items={[
                  "Ask PatchPilot anything",
                  "Review next patch proposal",
                  "Run checks after approval",
                ]}
                title="Pinned"
              />
              <ConversationGroup
                items={[
                  workspace?.name ?? "Workspace loading",
                  "Agent task console",
                  "Open workspace tools",
                ]}
                title="PatchPilot"
              />
            </div>
          </div>

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

        <div className="grid min-w-0 px-3 py-5 sm:px-4">
          <div className="mx-auto grid h-full w-full max-w-4xl grid-rows-[auto_auto_minmax(0,1fr)] gap-4">
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
              applyError={
                patchApplyMutation.error
                  ? apiErrorMessage(patchApplyMutation.error)
                  : undefined
              }
              createError={
                createTaskMutation.error
                  ? apiErrorMessage(createTaskMutation.error)
                  : undefined
              }
              events={taskEvents}
              isApplying={patchApplyMutation.isPending}
              isLoading={tasksQuery.isPending || taskDetailQuery.isPending}
              isRejecting={patchRejectMutation.isPending}
              isReverting={patchRevertMutation.isPending}
              onPatchApply={(patchId) => patchApplyMutation.mutate(patchId)}
              onPatchReject={(patchId) => patchRejectMutation.mutate(patchId)}
              onPatchRevert={(patchId) => patchRevertMutation.mutate(patchId)}
              onSelectTask={setActiveTaskId}
              patches={taskPatches}
              rejectError={
                patchRejectMutation.error
                  ? apiErrorMessage(patchRejectMutation.error)
                  : undefined
              }
              revertError={
                patchRevertMutation.error
                  ? apiErrorMessage(patchRevertMutation.error)
                  : undefined
              }
              tasks={tasksQuery.data?.tasks ?? []}
              workspaceRoot={workspace?.rootPath}
            />
          </div>
        </div>
      </section>
    </AppShell>
  );
}

function ConversationGroup({
  items,
  title,
}: {
  items: string[];
  title: string;
}) {
  return (
    <section className="grid gap-1">
      <h2 className="text-muted px-2 text-xs font-semibold">{title}</h2>
      <div className="grid gap-0.5">
        {items.map((item, index) => (
          <div
            aria-current={index === 0 ? "page" : undefined}
            className="text-muted aria-[current=page]:bg-hover aria-[current=page]:text-ink flex min-h-9 min-w-0 items-center gap-2 rounded-md px-2 text-sm"
            key={item}
          >
            <MessageSquare aria-hidden="true" className="size-4 shrink-0" />
            <span className="truncate">{item}</span>
          </div>
        ))}
      </div>
    </section>
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
  applyError,
  createError,
  events,
  isApplying,
  isLoading,
  isRejecting,
  isReverting,
  onPatchApply,
  onPatchReject,
  onPatchRevert,
  onSelectTask,
  patches,
  rejectError,
  revertError,
  tasks,
  workspaceRoot,
}: {
  activeTask?: AgentTask;
  applyError?: string;
  createError?: string;
  events: AgentTaskEvent[];
  isApplying: boolean;
  isLoading: boolean;
  isRejecting: boolean;
  isReverting: boolean;
  onPatchApply: (patchId: string) => void;
  onPatchReject: (patchId: string) => void;
  onPatchRevert: (patchId: string) => void;
  onSelectTask: (taskId: string) => void;
  patches: AgentPatch[];
  rejectError?: string;
  revertError?: string;
  tasks: AgentTask[];
  workspaceRoot?: string;
}) {
  return (
    <div className="grid min-h-0 gap-3 lg:grid-cols-[16rem_minmax(0,1fr)]">
      <div className="bg-panel min-h-0 overflow-hidden rounded-md shadow-sm">
        <div className="border-line border-b px-3 py-2">
          <p className="text-ink text-xs font-semibold">Tasks</p>
          {workspaceRoot ? (
            <p className="text-muted truncate text-xs">{workspaceRoot}</p>
          ) : null}
        </div>
        <div className="min-h-0 overflow-auto">
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
                <span className="text-muted flex min-w-0 items-center justify-between gap-2 text-xs">
                  <span className="truncate">
                    {task.model} · {task.reasoningEffort}
                  </span>
                  <span>{task.status}</span>
                </span>
              </button>
            ))
          )}
        </div>
      </div>

      <div className="bg-panel grid min-h-0 grid-rows-[auto_minmax(0,1fr)] overflow-hidden rounded-md shadow-sm">
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
              {activeTask.plan ? (
                <TaskBlock label="Plan" text={activeTask.plan} />
              ) : null}
              {events.map((event) => (
                <TaskEventRow event={event} key={event.id} />
              ))}
              {activeTask.generatedPatch ? (
                <TaskBlock label="Patch created" text={activeTask.summary} />
              ) : null}
              {patches.map((patch) => (
                <PatchReview
                  applyError={applyError}
                  isApplying={isApplying}
                  isRejecting={isRejecting}
                  isReverting={isReverting}
                  key={patch.id}
                  onApply={() => onPatchApply(patch.id)}
                  onReject={() => onPatchReject(patch.id)}
                  onRevert={() => onPatchRevert(patch.id)}
                  patch={patch}
                  rejectError={rejectError}
                  revertError={revertError}
                />
              ))}
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

function TaskBlock({ label, text }: { label: string; text: string }) {
  return (
    <div className="bg-hover grid gap-1 rounded-sm p-3">
      <p className="text-muted text-xs font-semibold">{label}</p>
      <p className="text-ink text-sm whitespace-pre-wrap">{text}</p>
    </div>
  );
}

function PatchReview({
  applyError,
  isApplying,
  isRejecting,
  isReverting,
  onApply,
  onReject,
  onRevert,
  patch,
  rejectError,
  revertError,
}: {
  applyError?: string;
  isApplying: boolean;
  isRejecting: boolean;
  isReverting: boolean;
  onApply: () => void;
  onReject: () => void;
  onRevert: () => void;
  patch: AgentPatch;
  rejectError?: string;
  revertError?: string;
}) {
  const canDecide = patch.status === "created" || patch.status === "proposed";
  const canRevert = patch.status === "applied";
  const error = applyError ?? rejectError ?? revertError;

  return (
    <div className="bg-panel grid min-w-0 gap-3 rounded-md p-3 shadow-sm">
      <div className="flex min-w-0 items-center justify-between gap-3">
        <div className="min-w-0">
          <p className="text-ink truncate text-sm font-semibold">
            Patch {patch.id}
          </p>
          <p className="text-muted text-xs">{patch.status}</p>
        </div>
        <StatusPill status={patch.status} />
      </div>
      {patch.summary ? (
        <p className="text-muted text-sm whitespace-pre-wrap">
          {patch.summary}
        </p>
      ) : null}
      <pre className="bg-hover text-ink max-h-64 overflow-auto rounded-sm p-3 text-xs whitespace-pre-wrap">
        {patch.diff}
      </pre>
      {error ? (
        <p className="text-warning text-sm font-medium">{error}</p>
      ) : null}
      <div className="flex flex-wrap gap-2">
        <Button
          disabled={!canDecide || isApplying}
          icon={<Check />}
          onClick={onApply}
          size="small"
          type="button"
        >
          Apply patch
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
        <Button
          disabled={!canRevert || isReverting}
          icon={<Undo2 />}
          onClick={onRevert}
          size="small"
          type="button"
          variant="secondary"
        >
          Revert
        </Button>
      </div>
    </div>
  );
}

function TaskEventRow({ event }: { event: AgentTaskEvent }) {
  const text = eventText(event);
  if (text.length === 0) {
    return null;
  }
  return (
    <div className="border-line grid gap-1 border-l px-3">
      <p className="text-muted text-xs font-semibold">{event.type}</p>
      <p className="text-ink text-sm whitespace-pre-wrap">{text}</p>
    </div>
  );
}

function eventText(event: AgentTaskEvent) {
  const payload = event.payload as Record<string, unknown>;
  if (typeof payload.text === "string") {
    return payload.text;
  }
  if (typeof payload.name === "string") {
    return payload.name;
  }
  if (typeof payload.patchId === "string") {
    return `Patch ${payload.patchId} is waiting for approval.`;
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

function updatePatchCache(
  queryClient: QueryClient,
  workspaceId: string,
  patch: AgentPatch,
) {
  queryClient.setQueryData<AgentTaskDetail>(
    ["agent-task", workspaceId, patch.taskId],
    (current) =>
      current
        ? {
            ...current,
            patches: [
              patch,
              ...current.patches.filter((item) => item.id !== patch.id),
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
  return event.type.startsWith("agent.") || event.type.startsWith("patch.");
}

function eventTaskId(event: WorkspaceEvent) {
  const payload = event.payload as Record<string, unknown>;
  if (typeof payload.taskId === "string") {
    return payload.taskId;
  }
  if (typeof payload.id === "string" && payload.id.startsWith("patch_")) {
    return typeof payload.taskId === "string" ? payload.taskId : "";
  }
  if (typeof payload.id === "string" && payload.id.startsWith("task_")) {
    return payload.id;
  }
  return "";
}
