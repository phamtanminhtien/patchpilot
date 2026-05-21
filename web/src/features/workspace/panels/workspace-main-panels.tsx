import type { FormEvent, ReactNode } from "react";

import type { Command } from "@/shared/api";
import { cn } from "@/shared/ui";

import type { GitChange } from "../git/workspace-git";
import type { WorkspacePanel } from "../workspace-panels";
import { CommandsPanel } from "./commands-panel";
import { FilesPanel } from "./files-panel";
import { GitPanel } from "./git-panel";
import { PreviewPanel } from "./preview-panel";

interface WorkspaceMainPanelsProps {
  activePanel: WorkspacePanel;
  command: {
    error?: string;
    isPending: boolean;
    onCommandChange: (value: string) => void;
    onSubmit: (event: FormEvent<HTMLFormElement>) => void;
    queuedCommand: Command | null;
    text: string;
  };
  files: {
    file?: string;
    fileError?: string;
    isFileLoading: boolean;
  };
  git: {
    changes: GitChange[];
    commitError?: string;
    commitMessage: string;
    diff?: string;
    diffError?: string;
    error?: string;
    isCommitPending: boolean;
    isDiffLoading: boolean;
    isLoading: boolean;
    isStagingChanges: boolean;
    lastCommitHash?: string;
    onCommitMessageChange: (value: string) => void;
    onCommitSubmit: (event: FormEvent<HTMLFormElement>) => void;
    onStageChanges: () => void;
    stagedPathCount: number;
    stageError?: string;
    unstagedPathCount: number;
  };
  selectedPath: string;
}

export function WorkspaceMainPanels({
  activePanel,
  command,
  files,
  git,
  selectedPath,
}: WorkspaceMainPanelsProps) {
  return (
    <section className="workspace-main-scroll bg-panel min-h-0 overflow-hidden shadow-md">
      <WorkspaceMainPanelFrame activePanel={activePanel} panel="files">
        <FilesPanel
          file={files.file}
          fileError={files.fileError}
          isLoading={files.isFileLoading}
          selectedPath={selectedPath}
        />
      </WorkspaceMainPanelFrame>

      <WorkspaceMainPanelFrame activePanel={activePanel} panel="git">
        <GitPanel
          commitError={git.commitError}
          commitMessage={git.commitMessage}
          diff={git.diff}
          diffError={git.diffError}
          gitError={git.error}
          hasChanges={git.changes.length > 0}
          isCommitPending={git.isCommitPending}
          isLoading={git.isDiffLoading || git.isLoading}
          isStagePending={git.isStagingChanges}
          lastCommitHash={git.lastCommitHash}
          onCommitMessageChange={git.onCommitMessageChange}
          onCommitSubmit={git.onCommitSubmit}
          onStageChanges={git.onStageChanges}
          selectedPath={selectedPath}
          stagedGitPathCount={git.stagedPathCount}
          stageError={git.stageError}
          unstagedGitPathCount={git.unstagedPathCount}
        />
      </WorkspaceMainPanelFrame>

      <WorkspaceMainPanelFrame activePanel={activePanel} panel="commands">
        <CommandsPanel
          commandText={command.text}
          error={command.error}
          isPending={command.isPending}
          onCommandChange={command.onCommandChange}
          onSubmit={command.onSubmit}
          queuedCommand={command.queuedCommand}
        />
      </WorkspaceMainPanelFrame>

      <WorkspaceMainPanelFrame activePanel={activePanel} panel="preview">
        <PreviewPanel />
      </WorkspaceMainPanelFrame>
    </section>
  );
}

function WorkspaceMainPanelFrame({
  activePanel,
  children,
  panel,
}: {
  activePanel: WorkspacePanel;
  children: ReactNode;
  panel: WorkspacePanel;
}) {
  const isActive = activePanel === panel;

  return (
    <section
      aria-hidden={isActive ? undefined : true}
      className={cn("h-full min-h-0", isActive ? "grid" : "hidden")}
      hidden={!isActive}
      role="tabpanel"
    >
      {children}
    </section>
  );
}
