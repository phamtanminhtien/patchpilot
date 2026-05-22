import type { FormEvent, ReactNode } from "react";

import type { Command, CommandOutput, Port } from "@/shared/api";
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
    activeCommand: Command | null;
    activeCommandId: string;
    confirmationCommand: string;
    isPending: boolean;
    isLoadingProcesses: boolean;
    isStopping: boolean;
    onCancelConfirmation: () => void;
    onCommandChange: (value: string) => void;
    onCommandConfirm: () => void;
    onCommandSelect: (commandId: string) => void;
    onCommandShortcut: (command: string) => void;
    onCommandStop: () => void;
    onSubmit: (event: FormEvent<HTMLFormElement>) => void;
    output: CommandOutput[];
    processes: Command[];
    queuedCommand: Command | null;
    stopError?: string;
    text: string;
  };
  files: {
    file?: string;
    fileError?: string;
    isFileLoading: boolean;
  };
  git: {
    changes: GitChange[];
    diff?: string;
    diffError?: string;
    error?: string;
    isDiffLoading: boolean;
    isLoading: boolean;
  };
  preview: {
    error?: string;
    exposeError?: string;
    exposingPort?: number;
    isExposing: boolean;
    isLoading: boolean;
    onExpose: (port: number) => void;
    ports: Port[];
  };
  selectedPath: string;
}

export function WorkspaceMainPanels({
  activePanel,
  command,
  files,
  git,
  preview,
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
          diff={git.diff}
          diffError={git.diffError}
          gitError={git.error}
          hasChanges={git.changes.length > 0}
          isLoading={git.isDiffLoading || git.isLoading}
          selectedPath={selectedPath}
        />
      </WorkspaceMainPanelFrame>

      <WorkspaceMainPanelFrame activePanel={activePanel} panel="commands">
        <CommandsPanel
          activeCommand={command.activeCommand}
          activeCommandId={command.activeCommandId}
          commandText={command.text}
          confirmationCommand={command.confirmationCommand}
          error={command.error}
          isLoadingProcesses={command.isLoadingProcesses}
          isPending={command.isPending}
          isStopping={command.isStopping}
          onCancelConfirmation={command.onCancelConfirmation}
          onCommandChange={command.onCommandChange}
          onCommandConfirm={command.onCommandConfirm}
          onCommandSelect={command.onCommandSelect}
          onCommandShortcut={command.onCommandShortcut}
          onCommandStop={command.onCommandStop}
          onSubmit={command.onSubmit}
          output={command.output}
          processes={command.processes}
          queuedCommand={command.queuedCommand}
          stopError={command.stopError}
        />
      </WorkspaceMainPanelFrame>

      <WorkspaceMainPanelFrame activePanel={activePanel} panel="preview">
        <PreviewPanel
          error={preview.error}
          exposeError={preview.exposeError}
          exposingPort={preview.exposingPort}
          isExposing={preview.isExposing}
          isLoading={preview.isLoading}
          onExpose={preview.onExpose}
          ports={preview.ports}
        />
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
