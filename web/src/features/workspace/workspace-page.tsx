import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  Bot,
  CheckCircle2,
  ChevronDown,
  Files,
  GitBranch,
  GitCommit,
  Loader2,
  MonitorUp,
  Play,
  RefreshCw,
  Send,
  Trash2,
  Undo2,
} from "lucide-react";
import { useQueryState } from "nuqs";
import { type FormEvent, type ReactNode, useMemo, useState } from "react";
import { Link } from "react-router";

import { AppShell } from "@/app/app-shell";
import { useThemePreference } from "@/app/theme";
import {
  apiErrorMessage,
  type Command,
  commitGitChanges,
  createWorkspace,
  discardGitChanges,
  type FileIndexEntry,
  getGitDiff,
  getGitStatus,
  getWorkspace,
  type GitCommitResponse,
  listFileIndex,
  listWorkspaces,
  queueCommand,
  readFile,
  refreshFileIndex,
  stageGitFiles,
  unstageGitFiles,
} from "@/shared/api";
import {
  Button,
  cn,
  StarterScreen,
  StatusPill,
  TextField,
  ThemeSwitcher,
} from "@/shared/ui";
import { panelParser, pathParser, workspaceIdParser } from "@/shared/url";

import { WorkspaceFileTree } from "./workspace-file-tree";
import { type GitChange, parseGitPorcelain } from "./workspace-git";
import {
  GitChangeActionButton,
  WorkspaceGitChangeItem,
} from "./workspace-git-change-item";

const panels = [
  {
    description: "Browse files and inspect small text content.",
    icon: Files,
    label: "Files",
    value: "files",
  },
  {
    description: "Review working tree status and selected diffs.",
    icon: GitBranch,
    label: "Git",
    value: "git",
  },
  {
    description: "Queue a direct command from the workspace root.",
    icon: Play,
    label: "Commands",
    value: "commands",
  },
  {
    description: "Prepare same-host preview controls for detected ports.",
    icon: MonitorUp,
    label: "Preview",
    value: "preview",
  },
] as const;

type WorkspacePanel = (typeof panels)[number]["value"];

export function WorkspacePage() {
  const [workspaceId, setWorkspaceId] = useQueryState(
    "workspaceId",
    workspaceIdParser,
  );
  const [panel, setPanel] = useQueryState("panel", panelParser);
  const [selectedPath, setSelectedPath] = useQueryState("path", pathParser);
  const [rootPath, setRootPath] = useState("");
  const [commandText, setCommandText] = useState("");
  const [commitMessage, setCommitMessage] = useState("");
  const [lastCommit, setLastCommit] = useState<GitCommitResponse | null>(null);
  const [queuedCommand, setQueuedCommand] = useState<Command | null>(null);
  const { preference, setPreference } = useThemePreference();
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

  const filesQuery = useQuery({
    enabled: workspaceId.length > 0 && panel === "files",
    queryFn: () => listFileIndex(workspaceId),
    queryKey: ["workspace-file-index", workspaceId],
  });

  const selectedFileEntry = filesQuery.data?.entries.find(
    (entry) => entry.path === selectedPath,
  );

  const fileQuery = useQuery({
    enabled:
      workspaceId.length > 0 &&
      panel === "files" &&
      selectedPath.length > 0 &&
      selectedFileEntry !== undefined,
    queryFn: () => readFile(workspaceId, selectedPath),
    queryKey: ["workspace-file", workspaceId, selectedPath],
  });

  const gitQuery = useQuery({
    enabled: workspaceId.length > 0 && (panel === "files" || panel === "git"),
    queryFn: () => getGitStatus(workspaceId),
    queryKey: ["workspace-git-status", workspaceId],
  });

  const gitChanges = useMemo(
    () => parseGitPorcelain(gitQuery.data?.porcelain ?? ""),
    [gitQuery.data?.porcelain],
  );

  const stagedGitPaths = useMemo(
    () =>
      gitChanges
        .filter((change) => isStagedGitChange(change))
        .map((change) => change.path),
    [gitChanges],
  );

  const unstagedGitPaths = useMemo(
    () =>
      gitChanges
        .filter(
          (change) =>
            isUnstagedGitChange(change) && isGitChangeStageable(change),
        )
        .map((change) => change.path),
    [gitChanges],
  );

  const gitDiffQuery = useQuery({
    enabled: workspaceId.length > 0 && panel === "git",
    queryFn: () => getGitDiff(workspaceId, selectedPath || undefined),
    queryKey: ["workspace-git-diff", workspaceId, selectedPath],
  });

  const commandMutation = useMutation({
    mutationFn: (command: string) => queueCommand(workspaceId, command),
    onSuccess: (command) => {
      setQueuedCommand(command);
      setCommandText("");
    },
  });

  const stageMutation = useMutation({
    mutationFn: (paths: string[]) => stageGitFiles(workspaceId, { paths }),
    onSuccess: (status) => {
      queryClient.setQueryData(["workspace-git-status", workspaceId], status);
      void queryClient.invalidateQueries({
        queryKey: ["workspace-git-diff", workspaceId],
      });
      setLastCommit(null);
    },
  });

  const unstageMutation = useMutation({
    mutationFn: (paths: string[]) => unstageGitFiles(workspaceId, { paths }),
    onSuccess: (status) => {
      queryClient.setQueryData(["workspace-git-status", workspaceId], status);
      void queryClient.invalidateQueries({
        queryKey: ["workspace-git-diff", workspaceId],
      });
      setLastCommit(null);
    },
  });

  const discardMutation = useMutation({
    mutationFn: (paths: string[]) => discardGitChanges(workspaceId, { paths }),
    onSuccess: (status) => {
      queryClient.setQueryData(["workspace-git-status", workspaceId], status);
      void queryClient.invalidateQueries({
        queryKey: ["workspace-git-diff", workspaceId],
      });
      setLastCommit(null);
    },
  });

  const commitMutation = useMutation({
    mutationFn: (paths: string[]) =>
      commitGitChanges(workspaceId, { message: commitMessage, paths }),
    onSuccess: (commit) => {
      setLastCommit(commit);
      setCommitMessage("");
      void queryClient.invalidateQueries({
        queryKey: ["workspace-git-status", workspaceId],
      });
      void queryClient.invalidateQueries({
        queryKey: ["workspace-git-diff", workspaceId],
      });
    },
  });

  const refreshFilesMutation = useMutation({
    mutationFn: () => refreshFileIndex(workspaceId),
    onSuccess: (data) => {
      queryClient.setQueryData(["workspace-file-index", workspaceId], data);
    },
  });

  const workspace = workspaceQuery.data;

  function handlePanelChange(nextPanel: WorkspacePanel) {
    void setPanel(nextPanel);
  }

  function handlePathSelect(path: string) {
    void setSelectedPath(path);
  }

  function handleStageChanges() {
    if (unstagedGitPaths.length === 0 || stageMutation.isPending) {
      return;
    }
    stageMutation.mutate(unstagedGitPaths);
  }

  function handleStageSelectedChanges(paths: string[]) {
    if (paths.length === 0 || stageMutation.isPending) {
      return;
    }
    stageMutation.mutate(paths);
  }

  function handleUnstageChanges(paths: string[]) {
    if (paths.length === 0 || unstageMutation.isPending) {
      return;
    }
    unstageMutation.mutate(paths);
  }

  function handleDiscardChanges(paths: string[]) {
    if (paths.length === 0 || discardMutation.isPending) {
      return;
    }
    discardMutation.mutate(paths);
  }

  function handleCommitSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (
      stagedGitPaths.length === 0 ||
      commitMessage.trim().length === 0 ||
      commitMutation.isPending
    ) {
      return;
    }
    commitMutation.mutate(stagedGitPaths);
  }

  function handleCommandSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const command = commandText.trim();
    if (command.length === 0 || commandMutation.isPending) {
      return;
    }
    commandMutation.mutate(command);
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
    <AppShell mode="workspace" workspace={workspace} workspaceId={workspaceId}>
      <section className="grid h-[calc(100vh-2.5rem)] min-h-0 grid-rows-[auto_minmax(0,1fr)] overflow-hidden lg:grid-cols-[3.5rem_15.5rem_minmax(0,1fr)] lg:grid-rows-1">
        <ActivityRail
          activePanel={panel}
          onPanelChange={handlePanelChange}
          workspaceId={workspaceId}
        />

        <div className="grid min-h-0 grid-rows-[auto_minmax(0,1fr)] overflow-hidden lg:contents">
          <WorkspaceSidebar
            activePanel={panel}
            files={filesQuery.data?.entries ?? []}
            filesError={
              filesQuery.error ? apiErrorMessage(filesQuery.error) : undefined
            }
            gitChanges={gitChanges}
            gitError={
              gitQuery.error ? apiErrorMessage(gitQuery.error) : undefined
            }
            isDiscardingChanges={discardMutation.isPending}
            isFilesLoading={filesQuery.isPending}
            isGitLoading={gitQuery.isPending}
            isRefreshingFiles={refreshFilesMutation.isPending}
            isStagingChanges={stageMutation.isPending}
            isUnstagingChanges={unstageMutation.isPending}
            onChangesDiscard={handleDiscardChanges}
            onChangesStage={handleStageSelectedChanges}
            onFileIndexRefresh={() => refreshFilesMutation.mutate()}
            onPathSelect={handlePathSelect}
            onStagedChangesUnstage={handleUnstageChanges}
            selectedPath={selectedPath}
            workspace={workspace}
            workspaceError={
              workspaceQuery.error
                ? apiErrorMessage(workspaceQuery.error)
                : undefined
            }
          />

          <main className="grid min-h-0 grid-rows-[minmax(0,1fr)_10rem] overflow-hidden">
            <section className="workspace-main-scroll bg-panel min-h-0 overflow-hidden shadow-md">
              {panel === "files" ? (
                <FilesPanel
                  file={fileQuery.data?.content}
                  fileError={
                    fileQuery.error
                      ? apiErrorMessage(fileQuery.error)
                      : undefined
                  }
                  isLoading={fileQuery.isPending}
                  selectedPath={selectedPath}
                />
              ) : null}

              {panel === "git" ? (
                <GitPanel
                  commitError={
                    commitMutation.error
                      ? apiErrorMessage(commitMutation.error)
                      : undefined
                  }
                  commitMessage={commitMessage}
                  diff={gitDiffQuery.data?.diff}
                  diffError={
                    gitDiffQuery.error
                      ? apiErrorMessage(gitDiffQuery.error)
                      : undefined
                  }
                  gitError={
                    gitQuery.error ? apiErrorMessage(gitQuery.error) : undefined
                  }
                  hasChanges={gitChanges.length > 0}
                  isLoading={gitDiffQuery.isPending || gitQuery.isPending}
                  isCommitPending={commitMutation.isPending}
                  isStagePending={stageMutation.isPending}
                  lastCommitHash={lastCommit?.hash}
                  onCommitMessageChange={setCommitMessage}
                  onCommitSubmit={handleCommitSubmit}
                  onStageChanges={handleStageChanges}
                  selectedPath={selectedPath}
                  stagedGitPathCount={stagedGitPaths.length}
                  stageError={
                    stageMutation.error
                      ? apiErrorMessage(stageMutation.error)
                      : undefined
                  }
                  unstagedGitPathCount={unstagedGitPaths.length}
                />
              ) : null}

              {panel === "commands" ? (
                <CommandsPanel
                  commandText={commandText}
                  error={
                    commandMutation.error
                      ? apiErrorMessage(commandMutation.error)
                      : undefined
                  }
                  isPending={commandMutation.isPending}
                  onCommandChange={setCommandText}
                  onSubmit={handleCommandSubmit}
                  queuedCommand={queuedCommand}
                />
              ) : null}

              {panel === "preview" ? <PreviewPanel /> : null}
            </section>

            <BottomPanel
              activePanel={panel}
              gitRawStatus={gitQuery.data?.porcelain}
              isGitLoading={gitQuery.isPending}
              queuedCommand={queuedCommand}
              selectedPath={selectedPath}
            />
          </main>
        </div>
      </section>
    </AppShell>
  );
}

function ActivityRail({
  activePanel,
  onPanelChange,
  workspaceId,
}: {
  activePanel: WorkspacePanel;
  onPanelChange: (panel: WorkspacePanel) => void;
  workspaceId: string;
}) {
  return (
    <nav
      aria-label="Workspace tools"
      className="bg-panel flex min-w-0 gap-1 overflow-x-auto px-1.5 py-1.5 shadow-sm lg:min-h-0 lg:flex-col lg:items-center lg:overflow-visible"
    >
      {panels.map((item) => (
        <button
          aria-label={item.label}
          aria-pressed={activePanel === item.value}
          className={cn(
            "text-muted hover:bg-hover hover:text-ink grid aspect-square min-w-14 cursor-pointer place-items-center gap-0.5 rounded-md px-2 text-xs font-medium transition lg:size-10 lg:min-w-0",
            activePanel === item.value
              ? "bg-accent-soft text-accent shadow-sm"
              : undefined,
          )}
          key={item.value}
          onClick={() => onPanelChange(item.value)}
          title={item.label}
          type="button"
        >
          <item.icon aria-hidden="true" className="size-4" />
          <span className="truncate lg:sr-only">{item.label}</span>
        </button>
      ))}

      <Button
        asChild
        className="aspect-square min-w-14 px-2 text-xs lg:mt-auto lg:size-10 lg:min-w-0 lg:px-0"
        size="small"
        variant="ghost"
      >
        <Link
          aria-label="Open Vibe Mode"
          title="Open Vibe Mode"
          to={`/vibe?workspaceId=${encodeURIComponent(workspaceId)}`}
        >
          <Bot aria-hidden="true" className="size-4" />
          <span className="truncate lg:sr-only">Vibe</span>
        </Link>
      </Button>
    </nav>
  );
}

function WorkspaceSidebar({
  activePanel,
  files,
  filesError,
  gitChanges,
  gitError,
  isDiscardingChanges,
  isFilesLoading,
  isGitLoading,
  isRefreshingFiles,
  isStagingChanges,
  isUnstagingChanges,
  onChangesDiscard,
  onChangesStage,
  onFileIndexRefresh,
  onPathSelect,
  onStagedChangesUnstage,
  selectedPath,
  workspace,
  workspaceError,
}: {
  activePanel: WorkspacePanel;
  files: FileIndexEntry[];
  filesError?: string;
  gitChanges: GitChange[];
  gitError?: string;
  isDiscardingChanges: boolean;
  isFilesLoading: boolean;
  isGitLoading: boolean;
  isRefreshingFiles: boolean;
  isStagingChanges: boolean;
  isUnstagingChanges: boolean;
  onChangesDiscard: (paths: string[]) => void;
  onChangesStage: (paths: string[]) => void;
  onFileIndexRefresh: () => void;
  onPathSelect: (path: string) => void;
  onStagedChangesUnstage: (paths: string[]) => void;
  selectedPath: string;
  workspace?: {
    name: string;
  };
  workspaceError?: string;
}) {
  return (
    <aside className="bg-canvas grid min-h-0 gap-1 shadow-sm lg:grid-rows-[auto_minmax(0,1fr)_auto] lg:overflow-hidden">
      <WorkspaceSidebarHeader
        activePanel={activePanel}
        isRefreshingFiles={isRefreshingFiles}
        onFileIndexRefresh={onFileIndexRefresh}
        workspace={workspace}
        workspaceError={workspaceError}
      />

      <div className="min-h-0 overflow-auto">
        {activePanel === "files" ? (
          <WorkspaceFileTree
            entries={files}
            error={filesError}
            gitChanges={gitChanges}
            isLoading={isFilesLoading}
            onSelect={onPathSelect}
            selectedPath={selectedPath}
          />
        ) : null}

        {activePanel === "git" ? (
          <GitChangeList
            changes={gitChanges}
            error={gitError}
            isDiscardingChanges={isDiscardingChanges}
            isLoading={isGitLoading}
            isStagingChanges={isStagingChanges}
            isUnstagingChanges={isUnstagingChanges}
            onChangesDiscard={onChangesDiscard}
            onChangesStage={onChangesStage}
            onSelect={onPathSelect}
            onStagedChangesUnstage={onStagedChangesUnstage}
            selectedPath={selectedPath}
          />
        ) : null}

        {activePanel === "commands" ? (
          <SidebarHint
            icon={<Play aria-hidden="true" className="size-4" />}
            message="Commands are queued through the API. Output replay is waiting on process endpoints."
            title="Command queue"
          />
        ) : null}

        {activePanel === "preview" ? (
          <SidebarHint
            icon={<MonitorUp aria-hidden="true" className="size-4" />}
            message="Preview controls will connect to detected ports once port APIs are available."
            title="Preview"
          />
        ) : null}
      </div>
    </aside>
  );
}

function WorkspaceSidebarHeader({
  activePanel,
  isRefreshingFiles,
  onFileIndexRefresh,
  workspace,
  workspaceError,
}: {
  activePanel: WorkspacePanel;
  isRefreshingFiles: boolean;
  onFileIndexRefresh: () => void;
  workspace?: {
    name: string;
  };
  workspaceError?: string;
}) {
  return (
    <div className="grid gap-1 p-2">
      <div className="grid min-w-0 grid-cols-[minmax(0,1fr)_auto] items-center gap-1.5">
        <div className="flex min-w-0 items-center gap-1.5">
          <div className="min-w-0">
            <p className="text-muted truncate text-xs font-bold uppercase">
              {workspace?.name ?? panelShortDescription(activePanel)}
            </p>
          </div>
        </div>

        {activePanel === "files" ? (
          <Button
            aria-label="Refresh index"
            className="aspect-square px-0"
            disabled={isRefreshingFiles}
            icon={
              <RefreshCw
                className={cn(
                  isRefreshingFiles ? "animate-spin" : "",
                  "size-4",
                )}
              />
            }
            onClick={onFileIndexRefresh}
            size="small"
            title="Refresh index"
            type="button"
            variant="ghost"
          >
            <span className="sr-only">Refresh index</span>
          </Button>
        ) : null}
      </div>
      {!workspace && workspaceError ? (
        <ErrorState message={workspaceError} />
      ) : null}
    </div>
  );
}

function GitChangeList({
  changes,
  error,
  isDiscardingChanges,
  isLoading,
  isStagingChanges,
  isUnstagingChanges,
  onChangesDiscard,
  onChangesStage,
  onSelect,
  onStagedChangesUnstage,
  selectedPath,
}: {
  changes: GitChange[];
  error?: string;
  isDiscardingChanges: boolean;
  isLoading: boolean;
  isStagingChanges: boolean;
  isUnstagingChanges: boolean;
  onChangesDiscard: (paths: string[]) => void;
  onChangesStage: (paths: string[]) => void;
  onSelect: (path: string) => void;
  onStagedChangesUnstage: (paths: string[]) => void;
  selectedPath: string;
}) {
  const visibleChanges = changes.filter(
    (change) => !isIgnoredGitChange(change),
  );
  const stagedChanges = visibleChanges.filter((change) =>
    isStagedGitChange(change),
  );
  const unstagedChanges = visibleChanges.filter((change) =>
    isUnstagedGitChange(change),
  );

  if (error) {
    return <ErrorState message={error} />;
  }

  if (isLoading) {
    return <LoadingState label="Loading Git status" />;
  }

  if (visibleChanges.length === 0) {
    return <EmptyState message="Working tree is clean." />;
  }

  return (
    <div className="grid gap-1">
      <GitChangeSection
        actionKind="staged"
        changes={stagedChanges}
        isDiscardingChanges={isDiscardingChanges}
        isStagingChanges={isStagingChanges}
        isUnstagingChanges={isUnstagingChanges}
        label="Staged Changes"
        onChangesDiscard={onChangesDiscard}
        onChangesStage={onChangesStage}
        onSelect={onSelect}
        onStagedChangesUnstage={onStagedChangesUnstage}
        selectedPath={selectedPath}
      />
      <GitChangeSection
        actionKind="changes"
        changes={unstagedChanges}
        isDiscardingChanges={isDiscardingChanges}
        isStagingChanges={isStagingChanges}
        isUnstagingChanges={isUnstagingChanges}
        label="Changes"
        onChangesDiscard={onChangesDiscard}
        onChangesStage={onChangesStage}
        onSelect={onSelect}
        onStagedChangesUnstage={onStagedChangesUnstage}
        selectedPath={selectedPath}
      />
    </div>
  );
}

function GitChangeSection({
  actionKind,
  changes,
  isDiscardingChanges,
  isStagingChanges,
  isUnstagingChanges,
  label,
  onChangesDiscard,
  onChangesStage,
  onSelect,
  onStagedChangesUnstage,
  selectedPath,
}: {
  actionKind: "changes" | "staged";
  changes: GitChange[];
  isDiscardingChanges: boolean;
  isStagingChanges: boolean;
  isUnstagingChanges: boolean;
  label: string;
  onChangesDiscard: (paths: string[]) => void;
  onChangesStage: (paths: string[]) => void;
  onSelect: (path: string) => void;
  onStagedChangesUnstage: (paths: string[]) => void;
  selectedPath: string;
}) {
  const actionablePaths = changes
    .filter((change) => isGitChangeStageable(change))
    .map((change) => change.path);

  return (
    <section className="grid gap-px">
      <div className="text-ink grid min-h-8 grid-cols-[auto_minmax(0,1fr)_auto_auto] items-center gap-1 px-1 text-xs font-semibold">
        <ChevronDown aria-hidden="true" className="text-muted size-4" />
        <span className="truncate">{label}</span>
        <span
          aria-label={`${changes.length} ${label.toLowerCase()} paths`}
          className="bg-hover text-muted grid min-w-6 place-items-center rounded-full px-1.5 py-0.5 text-xs font-semibold"
        >
          {changes.length}
        </span>
        <GitChangeSectionActions
          actionKind={actionKind}
          isDiscardingChanges={isDiscardingChanges}
          isStagingChanges={isStagingChanges}
          isUnstagingChanges={isUnstagingChanges}
          onDiscard={() => onChangesDiscard(actionablePaths)}
          onStage={() => onChangesStage(actionablePaths)}
          onUnstage={() => onStagedChangesUnstage(actionablePaths)}
          pathCount={actionablePaths.length}
        />
      </div>

      {changes.length === 0 ? (
        <p className="text-muted px-6 py-1 text-xs">No paths.</p>
      ) : (
        changes.map((change) => (
          <WorkspaceGitChangeItem
            actionKind={actionKind}
            change={change}
            isDiscardingChanges={isDiscardingChanges}
            isSelected={selectedPath === change.path}
            isStagingChanges={isStagingChanges}
            isUnstagingChanges={isUnstagingChanges}
            key={`${label}-${change.id}`}
            onDiscard={() => onChangesDiscard([change.path])}
            onSelect={onSelect}
            onStage={() => onChangesStage([change.path])}
            onUnstage={() => onStagedChangesUnstage([change.path])}
          />
        ))
      )}
    </section>
  );
}

function GitChangeSectionActions({
  actionKind,
  isDiscardingChanges,
  isStagingChanges,
  isUnstagingChanges,
  onDiscard,
  onStage,
  onUnstage,
  pathCount,
}: {
  actionKind: "changes" | "staged";
  isDiscardingChanges: boolean;
  isStagingChanges: boolean;
  isUnstagingChanges: boolean;
  onDiscard: () => void;
  onStage: () => void;
  onUnstage: () => void;
  pathCount: number;
}) {
  if (pathCount === 0) {
    return null;
  }

  return (
    <div className="flex shrink-0 items-center gap-0.5">
      {actionKind === "changes" ? (
        <>
          <GitChangeActionButton
            disabled={isDiscardingChanges}
            icon={<Trash2 aria-hidden="true" className="size-3.5" />}
            label="Discard all changes"
            onClick={onDiscard}
          />
          <GitChangeActionButton
            disabled={isStagingChanges}
            icon={<GitBranch aria-hidden="true" className="size-3.5" />}
            label="Stage all changes"
            onClick={onStage}
          />
        </>
      ) : (
        <GitChangeActionButton
          disabled={isUnstagingChanges}
          icon={<Undo2 aria-hidden="true" className="size-3.5" />}
          label="Unstage all staged changes"
          onClick={onUnstage}
        />
      )}
    </div>
  );
}

function FilesPanel({
  file,
  fileError,
  isLoading,
  selectedPath,
}: {
  file?: string;
  fileError?: string;
  isLoading: boolean;
  selectedPath: string;
}) {
  if (selectedPath.length === 0) {
    return (
      <MainEmptyState
        icon={<Files aria-hidden="true" className="size-6" />}
        message="Select a file from the workspace list to inspect its text content."
        title="No file selected"
      />
    );
  }

  if (fileError) {
    return <ErrorState className="p-3" message={fileError} />;
  }

  if (isLoading) {
    return <LoadingState className="p-3" label="Loading file" />;
  }

  return (
    <pre className="workspace-main-scroll text-ink h-full min-h-0 overflow-auto p-3 text-xs leading-5 break-words whitespace-pre-wrap">
      {file ?? "File content will appear here."}
    </pre>
  );
}

function GitPanel({
  commitError,
  commitMessage,
  diff,
  diffError,
  gitError,
  hasChanges,
  isCommitPending,
  isLoading,
  isStagePending,
  lastCommitHash,
  onCommitMessageChange,
  onCommitSubmit,
  selectedPath,
  onStageChanges,
  stagedGitPathCount,
  stageError,
  unstagedGitPathCount,
}: {
  commitError?: string;
  commitMessage: string;
  diff?: string;
  diffError?: string;
  gitError?: string;
  hasChanges: boolean;
  isCommitPending: boolean;
  isLoading: boolean;
  isStagePending: boolean;
  lastCommitHash?: string;
  onCommitMessageChange: (value: string) => void;
  onCommitSubmit: (event: FormEvent<HTMLFormElement>) => void;
  onStageChanges: () => void;
  selectedPath: string;
  stagedGitPathCount: number;
  stageError?: string;
  unstagedGitPathCount: number;
}) {
  if (gitError) {
    return <ErrorState className="p-3" message={gitError} />;
  }

  if (diffError) {
    return <ErrorState className="p-3" message={diffError} />;
  }

  if (isLoading) {
    return <LoadingState className="p-3" label="Loading diff" />;
  }

  if (!hasChanges) {
    return (
      <MainEmptyState
        icon={<CheckCircle2 aria-hidden="true" className="size-6" />}
        message="Git reports no modified, staged, or untracked paths."
        title="Working tree clean"
      />
    );
  }

  return (
    <div className="grid h-full min-h-0 grid-rows-[auto_minmax(0,1fr)]">
      <div className="bg-canvas border-line grid gap-2 border-b p-2">
        <div className="flex min-w-0 flex-wrap items-center gap-2">
          <Button
            disabled={unstagedGitPathCount === 0 || isStagePending}
            icon={
              isStagePending ? (
                <Loader2 className="animate-spin" />
              ) : (
                <GitBranch />
              )
            }
            onClick={onStageChanges}
            size="small"
            type="button"
            variant="secondary"
          >
            Stage changes
          </Button>
          <span className="text-muted text-xs">
            {unstagedGitPathCount} unstaged · {stagedGitPathCount} staged
          </span>
          {lastCommitHash ? (
            <span className="text-muted min-w-0 truncate text-xs">
              Committed {lastCommitHash.slice(0, 12)}
            </span>
          ) : null}
        </div>

        <form
          className="grid gap-2 sm:grid-cols-[minmax(0,1fr)_auto]"
          onSubmit={onCommitSubmit}
        >
          <TextField
            label="Commit message"
            name="commit-message"
            onChange={(event) => onCommitMessageChange(event.target.value)}
            placeholder="Describe the selected changes"
            size="compact"
            value={commitMessage}
          />
          <Button
            disabled={
              stagedGitPathCount === 0 ||
              commitMessage.trim().length === 0 ||
              isCommitPending
            }
            icon={
              isCommitPending ? (
                <Loader2 className="animate-spin" />
              ) : (
                <GitCommit />
              )
            }
            size="compact"
          >
            Commit
          </Button>
        </form>

        {stageError ? <ErrorState message={stageError} /> : null}
        {commitError ? <ErrorState message={commitError} /> : null}
      </div>

      {diff ? (
        <pre className="workspace-main-scroll text-ink h-full min-h-0 overflow-auto p-3 text-xs leading-5 break-words whitespace-pre-wrap">
          {diff}
        </pre>
      ) : (
        <MainEmptyState
          icon={<GitBranch aria-hidden="true" className="size-6" />}
          message={
            selectedPath
              ? "No diff is available for the selected path."
              : "Select a changed file to inspect its diff."
          }
          title="No diff output"
        />
      )}
    </div>
  );
}

function CommandsPanel({
  commandText,
  error,
  isPending,
  onCommandChange,
  onSubmit,
  queuedCommand,
}: {
  commandText: string;
  error?: string;
  isPending: boolean;
  onCommandChange: (value: string) => void;
  onSubmit: (event: FormEvent<HTMLFormElement>) => void;
  queuedCommand: Command | null;
}) {
  return (
    <div className="grid gap-3 p-3">
      <form
        className="grid gap-2 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-end"
        onSubmit={onSubmit}
      >
        <TextField
          label="Command"
          name="workspace-command"
          onChange={(event) => onCommandChange(event.target.value)}
          placeholder="pnpm --dir web test"
          size="compact"
          value={commandText}
        />
        <Button
          disabled={isPending || commandText.trim().length === 0}
          icon={isPending ? <Loader2 className="animate-spin" /> : <Send />}
          size="compact"
        >
          Queue command
        </Button>
      </form>

      {error ? <ErrorState message={error} /> : null}

      {queuedCommand ? (
        <div className="bg-hover grid gap-1.5 rounded-md p-2.5">
          <div className="flex min-w-0 items-center justify-between gap-2">
            <p className="text-ink truncate text-xs font-semibold">
              Command accepted
            </p>
            <StatusPill status={queuedCommand.status} />
          </div>
          <p className="text-muted text-xs break-all">
            {queuedCommand.command}
          </p>
          <p className="text-muted text-xs">ID: {queuedCommand.id}</p>
        </div>
      ) : (
        <MainEmptyState
          icon={<Play aria-hidden="true" className="size-6" />}
          message="Submit a command to queue it. Streaming output will appear after process endpoints are available."
          title="No command queued"
        />
      )}
    </div>
  );
}

function PreviewPanel() {
  return (
    <MainEmptyState
      icon={<MonitorUp aria-hidden="true" className="size-6" />}
      message="Same-host preview controls will appear here after process and port APIs are implemented."
      title="Preview unavailable"
    />
  );
}

function BottomPanel({
  activePanel,
  gitRawStatus,
  isGitLoading,
  queuedCommand,
  selectedPath,
}: {
  activePanel: WorkspacePanel;
  gitRawStatus?: string;
  isGitLoading: boolean;
  queuedCommand: Command | null;
  selectedPath: string;
}) {
  return (
    <section className="bg-panel border-line grid min-h-0 grid-rows-[auto_minmax(0,1fr)] overflow-hidden border-t shadow-sm">
      <div className="bg-hover flex min-h-8 min-w-0 items-center gap-1 overflow-x-auto px-1.5">
        <BottomTab active={activePanel === "files"}>File</BottomTab>
        <BottomTab active={activePanel === "git"}>Git status</BottomTab>
        <BottomTab active={activePanel === "commands"}>Command</BottomTab>
        <BottomTab active={activePanel === "preview"}>Preview</BottomTab>
      </div>

      <div className="min-h-0 overflow-auto p-2">
        {activePanel === "files" ? (
          <p className="text-muted text-xs">
            {selectedPath
              ? `Selected path: ${selectedPath}`
              : "Select a file to populate the workspace output area."}
          </p>
        ) : null}

        {activePanel === "git" ? (
          isGitLoading ? (
            <LoadingState label="Loading raw Git status" />
          ) : (
            <pre className="text-ink text-xs leading-5 break-words whitespace-pre-wrap">
              {gitRawStatus || "Working tree clean."}
            </pre>
          )
        ) : null}

        {activePanel === "commands" ? (
          queuedCommand ? (
            <pre className="text-ink text-xs leading-5 break-words whitespace-pre-wrap">
              {`${queuedCommand.status} ${queuedCommand.id}\n${queuedCommand.command}`}
            </pre>
          ) : (
            <p className="text-muted text-xs">
              Queued command metadata will appear here after submission.
            </p>
          )
        ) : null}

        {activePanel === "preview" ? (
          <p className="text-muted text-xs">
            No detected port API is connected in this MVP slice.
          </p>
        ) : null}
      </div>
    </section>
  );
}

function BottomTab({
  active,
  children,
}: {
  active: boolean;
  children: ReactNode;
}) {
  return (
    <span
      className={cn(
        "text-muted inline-flex min-h-6 shrink-0 items-center rounded-sm px-1.5 text-xs font-medium",
        active ? "bg-panel text-ink shadow-sm" : undefined,
      )}
    >
      {children}
    </span>
  );
}

function SidebarHint({
  icon,
  message,
  title,
}: {
  icon: ReactNode;
  message: string;
  title: string;
}) {
  return (
    <div className="grid gap-1.5 rounded-md p-1.5">
      <div className="text-ink flex min-w-0 items-center gap-1.5 text-xs font-semibold">
        <span className="text-accent grid size-6 shrink-0 place-items-center">
          {icon}
        </span>
        <span className="truncate">{title}</span>
      </div>
      <p className="text-muted text-xs leading-5">{message}</p>
    </div>
  );
}

function MainEmptyState({
  icon,
  message,
  title,
}: {
  icon: ReactNode;
  message: string;
  title: string;
}) {
  return (
    <div className="grid min-h-56 place-items-center p-3">
      <div className="grid max-w-md justify-items-center gap-2 text-center">
        <span className="bg-accent-soft text-accent grid size-10 place-items-center rounded-md shadow-sm">
          {icon}
        </span>
        <div className="grid gap-1">
          <h3 className="text-ink text-sm font-semibold">{title}</h3>
          <p className="text-muted text-xs leading-5">{message}</p>
        </div>
      </div>
    </div>
  );
}

function EmptyState({ message }: { message: string }) {
  return <p className="text-muted text-xs leading-5">{message}</p>;
}

function ErrorState({
  className,
  message,
}: {
  className?: string;
  message: string;
}) {
  return (
    <p className={cn("text-warning text-xs font-medium", className)}>
      {message}
    </p>
  );
}

function LoadingState({
  className,
  label,
}: {
  className?: string;
  label: string;
}) {
  return (
    <div
      aria-label={label}
      className={cn(
        "text-muted flex min-h-9 items-center gap-1.5 text-xs",
        className,
      )}
    >
      <Loader2 aria-hidden="true" className="size-4 shrink-0 animate-spin" />
      <span>{label}...</span>
    </div>
  );
}

function panelShortDescription(panel: WorkspacePanel) {
  switch (panel) {
    case "files":
      return "File tree";
    case "git":
      return "Working tree";
    case "commands":
      return "Queue status";
    case "preview":
      return "Port preview";
  }
}

function isGitChangeStageable(change: GitChange) {
  return change.status !== "Ignored";
}

function isIgnoredGitChange(change: GitChange) {
  return change.status === "Ignored";
}

function isStagedGitChange(change: GitChange) {
  const indexStatus = change.code[0] ?? " ";
  return indexStatus !== " " && indexStatus !== "?" && indexStatus !== "!";
}

function isUnstagedGitChange(change: GitChange) {
  const worktreeStatus = change.code[1] ?? " ";
  return worktreeStatus !== " ";
}
