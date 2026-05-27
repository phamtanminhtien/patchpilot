import {
  ExternalLink,
  GitCommit,
  Loader2,
  MonitorUp,
  RefreshCw,
  RotateCw,
} from "lucide-react";
import type { FormEvent } from "react";
import { useState } from "react";

import type { FileIndexEntry, FileSearchResult, Port } from "@/shared/api";
import { FileIcon } from "@/shared/file-icons";
import {
  Button,
  cn,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogRoot,
  DialogTitle,
  StatusPill,
  TextField,
} from "@/shared/ui";

import { ErrorState } from "../components/error-state";
import { LoadingState } from "../components/loading-state";
import { SidebarHint } from "../components/sidebar-hint";
import { GitChangeList } from "../git/git-change-list";
import {
  type GitChange,
  stagedGitPaths as selectStagedGitPaths,
  visibleGitChanges,
} from "../git/workspace-git";
import { WorkspaceFileTree } from "../workspace-file-tree";
import type { WorkspacePanel } from "../workspace-panels";
import { panelShortDescription } from "../workspace-panels";

export function WorkspaceSidebar({
  activePanel,
  files,
  filesError,
  gitChanges,
  gitCommitError,
  gitCommitMessage,
  gitError,
  gitLastCommitHash,
  gitStagedPathsForCommit,
  gitStageError,
  isDiscardingChanges,
  isGitCommitPending,
  isFilesLoading,
  isGitLoading,
  isSearchingFiles,
  isRefreshingFiles,
  isStagingChanges,
  isUnstagingChanges,
  onChangesDiscard,
  onChangesStage,
  onGitCommitMessageChange,
  onGitCommitSubmit,
  onFileIndexRefresh,
  onFileSearchQueryChange,
  onPortExpose,
  onPathSelect,
  onStagedChangesUnstage,
  ports,
  portsError,
  fileSearchError,
  fileSearchQuery,
  fileSearchResults,
  fileSearchTrimmedQuery,
  selectedPath,
  workspace,
  workspaceError,
  portExposeError,
  isExposingPort,
  isLoadingPorts,
  exposingPort,
}: {
  activePanel: WorkspacePanel;
  files: FileIndexEntry[];
  filesError?: string;
  gitChanges: GitChange[];
  gitCommitError?: string;
  gitCommitMessage: string;
  gitError?: string;
  gitLastCommitHash?: string;
  gitStagedPathsForCommit: string[];
  gitStageError?: string;
  isDiscardingChanges: boolean;
  isGitCommitPending: boolean;
  isFilesLoading: boolean;
  isGitLoading: boolean;
  isSearchingFiles: boolean;
  isExposingPort: boolean;
  isLoadingPorts: boolean;
  exposingPort?: number;
  isRefreshingFiles: boolean;
  isStagingChanges: boolean;
  isUnstagingChanges: boolean;
  onChangesDiscard: (paths: string[]) => void;
  onChangesStage: (paths: string[]) => void;
  onGitCommitMessageChange: (value: string) => void;
  onGitCommitSubmit: (event: FormEvent<HTMLFormElement>) => void;
  onFileIndexRefresh: () => void;
  onFileSearchQueryChange: (query: string) => void;
  onPortExpose: (port: number) => void;
  onPathSelect: (path: string) => void;
  onStagedChangesUnstage: (paths: string[]) => void;
  ports: Port[];
  portsError?: string;
  fileSearchError?: string;
  fileSearchQuery: string;
  fileSearchResults: FileSearchResult[];
  fileSearchTrimmedQuery: string;
  portExposeError?: string;
  selectedPath: string;
  workspace?: {
    name: string;
  };
  workspaceError?: string;
}) {
  return (
    <aside className="border-line/45 bg-panel grid min-h-0 gap-1 border-b lg:grid-rows-[auto_minmax(0,1fr)] lg:overflow-hidden lg:border-r lg:border-b-0">
      <WorkspaceSidebarHeader
        activePanel={activePanel}
        isRefreshingFiles={isRefreshingFiles}
        onFileIndexRefresh={onFileIndexRefresh}
        workspace={workspace}
        workspaceError={workspaceError}
      />

      <div className="min-h-0 overflow-auto">
        {activePanel === "files" ? (
          <div className="grid gap-2 pb-2">
            <WorkspaceFileSearch
              error={fileSearchError}
              isLoading={isSearchingFiles}
              onQueryChange={onFileSearchQueryChange}
              onSelect={onPathSelect}
              query={fileSearchQuery}
              results={fileSearchResults}
              trimmedQuery={fileSearchTrimmedQuery}
            />
            {fileSearchTrimmedQuery.length > 0 ? null : (
              <WorkspaceFileTree
                entries={files}
                error={filesError}
                gitChanges={gitChanges}
                isLoading={isFilesLoading}
                onSelect={onPathSelect}
                selectedPath={selectedPath}
              />
            )}
          </div>
        ) : null}

        {activePanel === "git" ? (
          <div className="grid gap-2 pb-2">
            <GitCommitBox
              commitError={gitCommitError}
              commitMessage={gitCommitMessage}
              isCommitPending={isGitCommitPending}
              lastCommitHash={gitLastCommitHash}
              onCommitMessageChange={onGitCommitMessageChange}
              onCommitSubmit={onGitCommitSubmit}
              pathsForCommit={gitStagedPathsForCommit}
              stagedCount={stagedGitPathCount(gitChanges)}
              stageError={gitStageError}
            />
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
          </div>
        ) : null}

        {activePanel === "preview" ? (
          <PreviewServerList
            error={portsError}
            exposeError={portExposeError}
            exposingPort={exposingPort}
            isExposing={isExposingPort}
            isLoading={isLoadingPorts}
            onExpose={onPortExpose}
            ports={ports}
          />
        ) : null}
      </div>
    </aside>
  );
}

function WorkspaceFileSearch({
  error,
  isLoading,
  onQueryChange,
  onSelect,
  query,
  results,
  trimmedQuery,
}: {
  error?: string;
  isLoading: boolean;
  onQueryChange: (query: string) => void;
  onSelect: (path: string) => void;
  query: string;
  results: FileSearchResult[];
  trimmedQuery: string;
}) {
  return (
    <section className="grid gap-1 pb-1">
      <div className="px-2">
        <TextField
          className="bg-surface"
          id="workspace-file-search"
          label="Search files"
          labelHidden
          onChange={(event) => onQueryChange(event.target.value)}
          placeholder="Search files"
          size="small"
          value={query}
        />
      </div>
      {trimmedQuery.length === 0 ? null : (
        <div className="grid gap-1 pt-1">
          {error ? <ErrorState message={error} /> : null}
          {isLoading ? <LoadingState label="Searching files" /> : null}
          {!error && !isLoading && results.length === 0 ? (
            <p className="text-muted px-1 text-xs">No matching files.</p>
          ) : null}
          {!error && !isLoading && results.length > 0 ? (
            <WorkspaceFileSearchResults onSelect={onSelect} results={results} />
          ) : null}
        </div>
      )}
    </section>
  );
}

function WorkspaceFileSearchResults({
  onSelect,
  results,
}: {
  onSelect: (path: string) => void;
  results: FileSearchResult[];
}) {
  const groups = groupSearchResultsByPath(results);

  return (
    <div className="grid gap-0.5">
      <p className="text-muted px-2 text-xs font-medium">
        {results.length} {results.length === 1 ? "result" : "results"} in{" "}
        {groups.length} {groups.length === 1 ? "file" : "files"}
      </p>
      <div className="grid gap-0.5">
        {groups.map((group) => (
          <WorkspaceFileSearchResult
            group={group}
            key={group.path}
            onSelect={onSelect}
          />
        ))}
      </div>
    </div>
  );
}

function WorkspaceFileSearchResult({
  group,
  onSelect,
}: {
  group: FileSearchResultGroup;
  onSelect: (path: string) => void;
}) {
  const { directory, filename } = splitSearchResultPath(group.path);

  return (
    <button
      aria-label={group.path}
      className="hover:bg-hover grid min-w-0 cursor-pointer gap-0.5 rounded-xl px-2 py-1 text-left text-xs"
      onClick={() => onSelect(group.path)}
      title={group.path}
      type="button"
    >
      <span className="grid min-w-0 grid-cols-[minmax(0,1fr)_auto] items-center gap-1">
        <span className="flex min-w-0 items-baseline gap-1">
          <FileIcon className="size-3" path={group.path} />
          <span className="text-ink max-w-full min-w-0 shrink-0 truncate font-medium">
            {filename}
          </span>
          {directory ? (
            <span className="text-muted min-w-0 truncate opacity-70">
              {directory}
            </span>
          ) : null}
        </span>
        <span
          aria-hidden="true"
          className="bg-accent-soft text-accent min-w-4 shrink-0 rounded-xl px-1 text-center text-[10px] leading-4 font-semibold"
          title={`${group.results.length} ${
            group.results.length === 1 ? "result" : "results"
          }`}
        >
          {group.results.length}
        </span>
      </span>
      <span className="grid min-w-0 gap-0.5 pl-4">
        {group.results.map((result) => (
          <span
            className="text-muted min-w-0 truncate font-mono"
            key={`${result.kind}:${result.line ?? 0}:${result.preview ?? ""}`}
          >
            <span className="sr-only">{searchResultMeta(result)}</span>
            {searchResultPreview(result)}
          </span>
        ))}
      </span>
    </button>
  );
}

interface FileSearchResultGroup {
  path: string;
  results: FileSearchResult[];
}

function groupSearchResultsByPath(results: FileSearchResult[]) {
  const groups = new Map<string, FileSearchResultGroup>();

  for (const result of results) {
    const group = groups.get(result.path);
    if (group) {
      group.results.push(result);
    } else {
      groups.set(result.path, {
        path: result.path,
        results: [result],
      });
    }
  }

  return [...groups.values()];
}

function searchResultPreview(result: FileSearchResult) {
  if (result.preview) {
    return result.preview;
  }

  return searchResultMeta(result);
}

function searchResultMeta(result: FileSearchResult) {
  const kind = result.kind === "filename" ? "Filename" : "Content";
  return result.line ? `${kind} line ${result.line}` : kind;
}

function splitSearchResultPath(path: string) {
  const normalized = path.endsWith("/") ? path.slice(0, -1) : path;
  const separatorIndex = normalized.lastIndexOf("/");

  if (separatorIndex === -1) {
    return {
      directory: "",
      filename: normalized || path,
    };
  }

  return {
    directory: normalized.slice(0, separatorIndex),
    filename: normalized.slice(separatorIndex + 1) || normalized,
  };
}

function GitCommitBox({
  commitError,
  commitMessage,
  isCommitPending,
  lastCommitHash,
  onCommitMessageChange,
  onCommitSubmit,
  pathsForCommit,
  stagedCount,
  stageError,
}: {
  commitError?: string;
  commitMessage: string;
  isCommitPending: boolean;
  lastCommitHash?: string;
  onCommitMessageChange: (value: string) => void;
  onCommitSubmit: (event: FormEvent<HTMLFormElement>) => void;
  pathsForCommit: string[];
  stagedCount: number;
  stageError?: string;
}) {
  const [isReviewOpen, setIsReviewOpen] = useState(false);
  const canCommit =
    stagedCount > 0 && commitMessage.trim().length > 0 && !isCommitPending;

  function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!canCommit) {
      return;
    }
    setIsReviewOpen(true);
  }

  return (
    <form className="grid gap-2 px-2 pb-3" onSubmit={handleSubmit}>
      <DialogRoot onOpenChange={setIsReviewOpen} open={isReviewOpen}>
        <TextField
          className="bg-surface"
          id="commit-message"
          label="Commit message"
          labelHidden
          name="commit-message"
          onChange={(event) => onCommitMessageChange(event.target.value)}
          placeholder="Message"
          size="small"
          value={commitMessage}
        />
        <Button
          className="min-h-9"
          disabled={!canCommit}
          icon={
            isCommitPending ? (
              <Loader2 className="animate-spin" />
            ) : (
              <GitCommit />
            )
          }
          size="small"
          width="full"
        >
          Review commit
        </Button>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Review commit</DialogTitle>
            <DialogDescription>
              Commit {pathsForCommit.length} staged{" "}
              {pathsForCommit.length === 1 ? "path" : "paths"} with this
              message.
            </DialogDescription>
          </DialogHeader>
          <div className="grid gap-2">
            <div className="bg-surface grid gap-1 rounded-xl p-2">
              <p className="text-muted text-xs font-semibold">Message</p>
              <p className="text-ink text-sm break-words">{commitMessage}</p>
            </div>
            <div className="bg-surface grid max-h-48 gap-1 overflow-auto rounded-xl p-2">
              <p className="text-muted text-xs font-semibold">Paths</p>
              {pathsForCommit.map((path) => (
                <span
                  className="text-ink truncate font-mono text-xs"
                  key={path}
                  title={path}
                >
                  {path}
                </span>
              ))}
            </div>
          </div>
          <DialogFooter>
            <Button
              onClick={() => setIsReviewOpen(false)}
              size="small"
              type="button"
              variant="secondary"
            >
              Cancel
            </Button>
            <Button
              disabled={isCommitPending}
              icon={
                isCommitPending ? (
                  <Loader2 className="animate-spin" />
                ) : (
                  <GitCommit />
                )
              }
              onClick={(event) => {
                onCommitSubmit(event as unknown as FormEvent<HTMLFormElement>);
                setIsReviewOpen(false);
              }}
              size="small"
              type="button"
            >
              Commit
            </Button>
          </DialogFooter>
        </DialogContent>
      </DialogRoot>
      {stageError ? <ErrorState message={stageError} /> : null}
      {commitError ? <ErrorState message={commitError} /> : null}
      {lastCommitHash ? (
        <p className="text-muted truncate text-xs">
          Committed {lastCommitHash.slice(0, 12)}
        </p>
      ) : null}
    </form>
  );
}

function stagedGitPathCount(changes: GitChange[]) {
  return selectStagedGitPaths(visibleGitChanges(changes)).length;
}

function PreviewServerList({
  error,
  exposeError,
  exposingPort,
  isExposing,
  isLoading,
  onExpose,
  ports,
}: {
  error?: string;
  exposeError?: string;
  exposingPort?: number;
  isExposing: boolean;
  isLoading: boolean;
  onExpose: (port: number) => void;
  ports: Port[];
}) {
  return (
    <section className="grid gap-2 pb-2">
      <div className="grid min-h-11 items-center gap-2 px-2 pb-1">
        <div className="min-w-0">
          <h3 className="text-ink truncate text-xs font-semibold">
            Detected servers
          </h3>
          <p className="text-muted text-xs">
            {ports.length} {ports.length === 1 ? "port" : "ports"}
          </p>
        </div>
      </div>

      <div className="px-2">
        {exposeError ? <ErrorState message={exposeError} /> : null}
      </div>

      {isLoading ? (
        <SidebarHint
          icon={<MonitorUp aria-hidden="true" className="size-4" />}
          message="Loading detected ports."
          title="Preview"
        />
      ) : null}

      {!isLoading && ports.length === 0 ? (
        <SidebarHint
          icon={<MonitorUp aria-hidden="true" className="size-4" />}
          message={
            error ??
            "Run a dev server in Terminal to detect a local preview port."
          }
          title="Preview"
        />
      ) : null}

      {ports.length > 0 ? (
        <div className="grid">
          {ports.map((port) => (
            <PortRow
              isPending={isExposing && exposingPort === port.port}
              key={port.id}
              onExpose={onExpose}
              port={port}
            />
          ))}
        </div>
      ) : null}
    </section>
  );
}

function PortRow({
  isPending,
  onExpose,
  port,
}: {
  isPending: boolean;
  onExpose: (port: number) => void;
  port: Port;
}) {
  const description =
    port.status === "exposed"
      ? (port.exposedUrl ?? "Ready to open")
      : port.status === "closed"
        ? "Port closed before preview was opened."
        : "Detected and ready to expose.";

  return (
    <div className="hover:bg-hover grid min-h-14 grid-cols-[minmax(0,1fr)_4.75rem] items-center gap-2 rounded-xl px-3 py-2">
      <div className="min-w-0">
        <div className="flex min-w-0 items-center gap-2">
          <span
            className={cn(
              "h-3 w-1 shrink-0",
              port.status === "exposed"
                ? "bg-accent"
                : port.status === "closed"
                  ? "bg-warning"
                  : "bg-line",
            )}
          />
          <span className="text-ink truncate text-xs font-semibold">
            localhost:{port.port}
          </span>
        </div>
        <div className="mt-1 flex min-w-0 items-center gap-2">
          <StatusPill
            className="min-h-5 rounded-xl px-1.5 shadow-none"
            status={port.status}
          />
          <span className="text-muted truncate text-xs">{description}</span>
        </div>
      </div>

      {port.status === "exposed" && port.exposedUrl ? (
        <Button
          asChild
          className="text-accent min-h-8 px-2 shadow-none"
          size="small"
          variant="action"
        >
          <a href={port.exposedUrl} rel="noreferrer" target="_blank">
            <ExternalLink aria-hidden="true" className="size-3.5" />
            Open
          </a>
        </Button>
      ) : (
        <Button
          icon={
            port.status === "closed" ? (
              <RotateCw className={isPending ? "animate-spin" : undefined} />
            ) : (
              <ExternalLink />
            )
          }
          onClick={() => onExpose(port.port)}
          className="min-h-8 w-full px-2 shadow-none"
          size="small"
          type="button"
          variant={port.status === "closed" ? "action" : "primary"}
        >
          {port.status === "closed" ? "Retry" : "Expose"}
        </Button>
      )}
    </div>
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
            <p className="text-muted truncate text-xs font-bold tracking-wide uppercase">
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
            variant="action"
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
