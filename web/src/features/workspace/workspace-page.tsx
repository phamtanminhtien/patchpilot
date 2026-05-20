import { useMutation, useQuery } from "@tanstack/react-query";
import {
  Bot,
  CheckCircle2,
  FileCode2,
  Files,
  Folder,
  GitBranch,
  Loader2,
  MonitorUp,
  Play,
  Send,
} from "lucide-react";
import { useQueryState } from "nuqs";
import { type FormEvent, type ReactNode, useMemo, useState } from "react";
import { Link } from "react-router";

import { AppShell } from "@/app/app-shell";
import { useThemePreference } from "@/app/theme";
import {
  apiErrorMessage,
  type Command,
  createWorkspace,
  type FileEntry,
  getGitDiff,
  getGitStatus,
  getWorkspace,
  listFiles,
  listWorkspaces,
  queueCommand,
  readFile,
} from "@/shared/api";
import {
  Button,
  classNames,
  StarterScreen,
  StatusPill,
  TextField,
  ThemeSwitcher,
} from "@/shared/ui";
import { panelParser, pathParser, workspaceIdParser } from "@/shared/url";

import { type GitChange, parseGitPorcelain } from "./workspace-git";

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
  const [queuedCommand, setQueuedCommand] = useState<Command | null>(null);
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

  const filesQuery = useQuery({
    enabled: workspaceId.length > 0 && panel === "files",
    queryFn: () => listFiles(workspaceId),
    queryKey: ["workspace-files", workspaceId],
  });

  const selectedFileEntry = filesQuery.data?.entries.find(
    (entry) => entry.path === selectedPath,
  );

  const fileQuery = useQuery({
    enabled:
      workspaceId.length > 0 &&
      panel === "files" &&
      selectedPath.length > 0 &&
      selectedFileEntry?.isDir !== true,
    queryFn: () => readFile(workspaceId, selectedPath),
    queryKey: ["workspace-file", workspaceId, selectedPath],
  });

  const gitQuery = useQuery({
    enabled: workspaceId.length > 0 && panel === "git",
    queryFn: () => getGitStatus(workspaceId),
    queryKey: ["workspace-git-status", workspaceId],
  });

  const gitChanges = useMemo(
    () => parseGitPorcelain(gitQuery.data?.porcelain ?? ""),
    [gitQuery.data?.porcelain],
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

  const workspace = workspaceQuery.data;

  function handlePanelChange(nextPanel: WorkspacePanel) {
    void setPanel(nextPanel);
  }

  function handlePathSelect(path: string) {
    void setSelectedPath(path);
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

        <div className="grid min-h-0 overflow-auto lg:contents">
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
            isFilesLoading={filesQuery.isPending}
            isGitLoading={gitQuery.isPending}
            onPathSelect={handlePathSelect}
            selectedPath={selectedPath}
            workspace={workspace}
            workspaceError={
              workspaceQuery.error
                ? apiErrorMessage(workspaceQuery.error)
                : undefined
            }
          />

          <main className="grid min-h-[34rem] lg:min-h-0 lg:grid-rows-[minmax(0,1fr)_10rem] lg:overflow-hidden">
            <section className="workspace-main-scroll bg-panel min-h-0 overflow-auto shadow-md">
              {panel === "files" ? (
                <FilesPanel
                  file={fileQuery.data?.content}
                  fileError={
                    fileQuery.error
                      ? apiErrorMessage(fileQuery.error)
                      : undefined
                  }
                  isDirectorySelected={selectedFileEntry?.isDir === true}
                  isLoading={fileQuery.isPending}
                  selectedPath={selectedPath}
                />
              ) : null}

              {panel === "git" ? (
                <GitPanel
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
                  selectedPath={selectedPath}
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
          className={classNames(
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
  isFilesLoading,
  isGitLoading,
  onPathSelect,
  selectedPath,
  workspace,
  workspaceError,
}: {
  activePanel: WorkspacePanel;
  files: FileEntry[];
  filesError?: string;
  gitChanges: GitChange[];
  gitError?: string;
  isFilesLoading: boolean;
  isGitLoading: boolean;
  onPathSelect: (path: string) => void;
  selectedPath: string;
  workspace?: {
    name: string;
  };
  workspaceError?: string;
}) {
  return (
    <aside className="bg-canvas grid min-h-0 gap-2 shadow-sm lg:grid-rows-[auto_minmax(0,1fr)] lg:overflow-hidden">
      <div className="bg-panel grid gap-1.5 p-2 px-1">
        <div className="flex min-w-0 items-center gap-2">
          <ToolIcon panel={activePanel} />
          <div className="min-w-0">
            <p className="text-ink truncate text-xs font-semibold">
              {panelLabel(activePanel)}
            </p>
            <p className="text-muted truncate text-xs">
              {workspace?.name ?? panelShortDescription(activePanel)}
            </p>
          </div>
        </div>
        {!workspace && workspaceError ? (
          <ErrorState message={workspaceError} />
        ) : null}
      </div>

      <div className="min-h-0 overflow-auto">
        {activePanel === "files" ? (
          <FileList
            entries={files}
            error={filesError}
            isLoading={isFilesLoading}
            onSelect={onPathSelect}
            selectedPath={selectedPath}
          />
        ) : null}

        {activePanel === "git" ? (
          <GitChangeList
            changes={gitChanges}
            error={gitError}
            isLoading={isGitLoading}
            onSelect={onPathSelect}
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

function FileList({
  entries,
  error,
  isLoading,
  onSelect,
  selectedPath,
}: {
  entries: FileEntry[];
  error?: string;
  isLoading: boolean;
  onSelect: (path: string) => void;
  selectedPath: string;
}) {
  if (error) {
    return <ErrorState message={error} />;
  }

  if (isLoading) {
    return <LoadingState label="Loading files" />;
  }

  if (entries.length === 0) {
    return <EmptyState message="No files found at the workspace root." />;
  }

  return (
    <div className="grid">
      {entries.map((entry) => (
        <button
          aria-current={selectedPath === entry.path ? "true" : undefined}
          className={classNames(
            "text-muted hover:bg-hover hover:text-ink flex min-h-6 min-w-0 cursor-pointer items-center gap-1 px-2 text-left text-xs transition",
            selectedPath === entry.path ? "bg-hover text-ink" : undefined,
          )}
          key={entry.path}
          onClick={() => onSelect(entry.path)}
          type="button"
        >
          {entry.isDir ? (
            <Folder
              aria-hidden="true"
              className="text-accent size-3 shrink-0"
            />
          ) : (
            <FileCode2 aria-hidden="true" className="size-3 shrink-0" />
          )}
          <span className="min-w-0 flex-1 truncate">{entry.name}</span>
          {entry.isDir ? (
            <span className="text-muted shrink-0 text-xs">dir</span>
          ) : null}
        </button>
      ))}
    </div>
  );
}

function GitChangeList({
  changes,
  error,
  isLoading,
  onSelect,
  selectedPath,
}: {
  changes: GitChange[];
  error?: string;
  isLoading: boolean;
  onSelect: (path: string) => void;
  selectedPath: string;
}) {
  if (error) {
    return <ErrorState message={error} />;
  }

  if (isLoading) {
    return <LoadingState label="Loading Git status" />;
  }

  if (changes.length === 0) {
    return <EmptyState message="Working tree is clean." />;
  }

  return (
    <div className="grid gap-0.5">
      {changes.map((change) => (
        <button
          aria-current={selectedPath === change.path ? "true" : undefined}
          className={classNames(
            "text-muted hover:bg-hover hover:text-ink grid min-h-9 min-w-0 cursor-pointer grid-cols-[auto_minmax(0,1fr)] gap-x-1.5 rounded-sm px-1.5 py-1 text-left text-xs transition",
            selectedPath === change.path ? "bg-hover text-ink" : undefined,
          )}
          key={change.id}
          onClick={() => onSelect(change.path)}
          type="button"
        >
          <span
            className={classNames(
              "mt-0.5 flex aspect-square min-w-7 items-center justify-center rounded-sm px-1 py-0.5 text-xs font-semibold shadow-sm",
              changeTone(change.status),
            )}
          >
            {change.code.trim() || "--"}
          </span>
          <span className="min-w-0">
            <span className="block truncate">{change.displayPath}</span>
            <span className="text-muted block truncate text-xs">
              {change.status}
            </span>
          </span>
        </button>
      ))}
    </div>
  );
}

function FilesPanel({
  file,
  fileError,
  isDirectorySelected,
  isLoading,
  selectedPath,
}: {
  file?: string;
  fileError?: string;
  isDirectorySelected: boolean;
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

  if (isDirectorySelected) {
    return (
      <MainEmptyState
        icon={<Folder aria-hidden="true" className="size-6" />}
        message="Directory drill-down is outside this UI slice. Select a text file to preview."
        title="Directory selected"
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
    <pre className="workspace-main-scroll text-ink min-h-full overflow-auto p-3 text-xs leading-5 break-words whitespace-pre-wrap">
      {file ?? "File content will appear here."}
    </pre>
  );
}

function GitPanel({
  diff,
  diffError,
  gitError,
  hasChanges,
  isLoading,
  selectedPath,
}: {
  diff?: string;
  diffError?: string;
  gitError?: string;
  hasChanges: boolean;
  isLoading: boolean;
  selectedPath: string;
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

  if (!diff) {
    return (
      <MainEmptyState
        icon={<GitBranch aria-hidden="true" className="size-6" />}
        message={
          selectedPath
            ? "No diff is available for the selected path. Untracked files may not have diff output yet."
            : "Select a changed file to inspect its diff."
        }
        title="No diff output"
      />
    );
  }

  return (
    <pre className="workspace-main-scroll text-ink min-h-full overflow-auto p-3 text-xs leading-5 break-words whitespace-pre-wrap">
      {diff}
    </pre>
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
      className={classNames(
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
    <p className={classNames("text-warning text-xs font-medium", className)}>
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
      className={classNames(
        "text-muted flex min-h-9 items-center gap-1.5 text-xs",
        className,
      )}
    >
      <Loader2 aria-hidden="true" className="size-4 shrink-0 animate-spin" />
      <span>{label}...</span>
    </div>
  );
}

function ToolIcon({ panel }: { panel: WorkspacePanel }) {
  const item =
    panels.find((candidate) => candidate.value === panel) ?? panels[0];
  return (
    <span className="text-accent grid size-7 shrink-0 place-items-center">
      <item.icon aria-hidden="true" className="size-4" />
    </span>
  );
}

function panelLabel(panel: WorkspacePanel) {
  return panels.find((item) => item.value === panel)?.label ?? "Workspace";
}

function panelShortDescription(panel: WorkspacePanel) {
  switch (panel) {
    case "files":
      return "Root files";
    case "git":
      return "Working tree";
    case "commands":
      return "Queue status";
    case "preview":
      return "Port preview";
  }
}

function changeTone(status: string) {
  switch (status) {
    case "Added":
    case "Renamed":
      return "bg-accent-soft text-accent";
    case "Deleted":
    case "Conflict":
      return "bg-panel text-warning";
    default:
      return "bg-hover text-muted";
  }
}
