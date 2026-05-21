import { MonitorUp, Play, RefreshCw } from "lucide-react";

import type { FileIndexEntry } from "@/shared/api";
import { Button, cn } from "@/shared/ui";

import { ErrorState } from "../components/error-state";
import { SidebarHint } from "../components/sidebar-hint";
import { GitChangeList } from "../git/git-change-list";
import type { GitChange } from "../git/workspace-git";
import { WorkspaceFileTree } from "../workspace-file-tree";
import type { WorkspacePanel } from "../workspace-panels";
import { panelShortDescription } from "../workspace-panels";

export function WorkspaceSidebar({
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
