import type { Port } from "@/shared/api";

import type { GitChange } from "../git/workspace-git";
import type { WorkspacePanel } from "../workspace-panels";
import { FilesPanel } from "./files-panel";

interface WorkspaceMainPanelsProps {
  activePanel: WorkspacePanel;
  files: {
    activeDraft: string;
    dirtyPaths: string[];
    file?: string;
    fileError?: string;
    isFileLoading: boolean;
    isSaving: boolean;
    onCloseTab: (path: string) => void;
    onDraftChange: (path: string, content: string) => void;
    onSave: (path: string, content: string) => void;
    onSaveAll: () => void;
    onSelectTab: (path: string) => void;
    openTabs: string[];
    saveError?: string;
  };
  git: {
    changes: GitChange[];
    diff?: string;
    diffError?: string;
    error?: string;
    isDiffLoading: boolean;
    isLoading: boolean;
    isPatchStaging: boolean;
    onStagePatch: (patch: string) => void;
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
  files,
  selectedPath,
}: WorkspaceMainPanelsProps) {
  return (
    <section className="workspace-main-scroll bg-raised h-full min-h-0 overflow-hidden">
      <FilesPanel
        activeDraft={files.activeDraft}
        dirtyPaths={files.dirtyPaths}
        file={files.file}
        fileError={files.fileError}
        isLoading={files.isFileLoading}
        isSaving={files.isSaving}
        onCloseTab={files.onCloseTab}
        onDraftChange={files.onDraftChange}
        onSave={files.onSave}
        onSaveAll={files.onSaveAll}
        onSelectTab={files.onSelectTab}
        openTabs={files.openTabs}
        saveError={files.saveError}
        selectedPath={selectedPath}
      />
    </section>
  );
}
