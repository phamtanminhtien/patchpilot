import { GitCommit, Loader2, MonitorUp, Play, RefreshCw } from "lucide-react";
import type { FormEvent } from "react";

import type { FileIndexEntry } from "@/shared/api";
import { Button, cn } from "@/shared/ui";

import { ErrorState } from "../components/error-state";
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
  gitStageError,
  isDiscardingChanges,
  isGitCommitPending,
  isFilesLoading,
  isGitLoading,
  isRefreshingFiles,
  isStagingChanges,
  isUnstagingChanges,
  onChangesDiscard,
  onChangesStage,
  onGitCommitMessageChange,
  onGitCommitSubmit,
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
  gitCommitError?: string;
  gitCommitMessage: string;
  gitError?: string;
  gitLastCommitHash?: string;
  gitStageError?: string;
  isDiscardingChanges: boolean;
  isGitCommitPending: boolean;
  isFilesLoading: boolean;
  isGitLoading: boolean;
  isRefreshingFiles: boolean;
  isStagingChanges: boolean;
  isUnstagingChanges: boolean;
  onChangesDiscard: (paths: string[]) => void;
  onChangesStage: (paths: string[]) => void;
  onGitCommitMessageChange: (value: string) => void;
  onGitCommitSubmit: (event: FormEvent<HTMLFormElement>) => void;
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
    <aside className="bg-canvas grid min-h-0 gap-1 shadow-sm lg:grid-rows-[auto_minmax(0,1fr)] lg:overflow-hidden">
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
          <div className="grid gap-2 pb-2">
            <GitCommitBox
              commitError={gitCommitError}
              commitMessage={gitCommitMessage}
              isCommitPending={isGitCommitPending}
              lastCommitHash={gitLastCommitHash}
              onCommitMessageChange={onGitCommitMessageChange}
              onCommitSubmit={onGitCommitSubmit}
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

        {activePanel === "commands" ? (
          <SidebarHint
            icon={<Play aria-hidden="true" className="size-4" />}
            message="Run tests or builds from the workspace root and watch stdout and stderr in realtime."
            title="Command runner"
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

function GitCommitBox({
  commitError,
  commitMessage,
  isCommitPending,
  lastCommitHash,
  onCommitMessageChange,
  onCommitSubmit,
  stagedCount,
  stageError,
}: {
  commitError?: string;
  commitMessage: string;
  isCommitPending: boolean;
  lastCommitHash?: string;
  onCommitMessageChange: (value: string) => void;
  onCommitSubmit: (event: FormEvent<HTMLFormElement>) => void;
  stagedCount: number;
  stageError?: string;
}) {
  return (
    <form className="grid gap-2 px-2 pb-1" onSubmit={onCommitSubmit}>
      <label className="sr-only" htmlFor="commit-message">
        Commit message
      </label>
      <input
        className="bg-hover text-ink placeholder:text-muted min-h-9 w-full rounded-sm px-2.5 py-1.5 text-xs shadow-sm transition focus-visible:shadow-[inset_0_0_0_1px_var(--pp-color-focus)] focus-visible:!outline-none"
        id="commit-message"
        name="commit-message"
        onChange={(event) => onCommitMessageChange(event.target.value)}
        placeholder="Message"
        value={commitMessage}
      />
      <Button
        className="min-h-9"
        disabled={
          stagedCount === 0 ||
          commitMessage.trim().length === 0 ||
          isCommitPending
        }
        icon={
          isCommitPending ? <Loader2 className="animate-spin" /> : <GitCommit />
        }
        size="small"
        width="full"
      >
        Commit
      </Button>
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
