import type { ReactNode } from "react";

import type { Port } from "@/shared/api";
import { cn } from "@/shared/ui";

import type { GitChange } from "../git/workspace-git";
import type { WorkspacePanel } from "../workspace-panels";
import { FilesPanel } from "./files-panel";
import { GitPanel } from "./git-panel";
import { PreviewPanel } from "./preview-panel";

interface WorkspaceMainPanelsProps {
  activePanel: WorkspacePanel;
  files: {
    file?: string;
    fileError?: string;
    isFileLoading: boolean;
    isSaving: boolean;
    onSave: (content: string) => void;
    saveError?: string;
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
  files,
  git,
  preview,
  selectedPath,
}: WorkspaceMainPanelsProps) {
  return (
    <section className="workspace-main-scroll bg-raised min-h-0 overflow-hidden">
      <WorkspaceMainPanelFrame activePanel={activePanel} panel="files">
        <FilesPanel
          file={files.file}
          fileError={files.fileError}
          isLoading={files.isFileLoading}
          isSaving={files.isSaving}
          onSave={files.onSave}
          saveError={files.saveError}
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
