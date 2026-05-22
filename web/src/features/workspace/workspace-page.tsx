import { AppShell } from "@/app/app-shell";
import { useThemePreference } from "@/app/theme";
import { StarterScreen, ThemeSwitcher } from "@/shared/ui";

import { useWorkspaceController } from "./hooks/use-workspace-controller";
import { useWorkspaceUrlState } from "./hooks/use-workspace-url-state";
import { ActivityRail } from "./layout/activity-rail";
import { WorkspaceBottomPanel } from "./layout/workspace-bottom-panel";
import { WorkspaceLayout } from "./layout/workspace-layout";
import { WorkspaceSidebar } from "./layout/workspace-sidebar";
import { WorkspaceMainPanels } from "./panels/workspace-main-panels";

export function WorkspacePage() {
  const urlState = useWorkspaceUrlState();
  const { panel, selectedPath, workspaceId } = urlState;
  const controller = useWorkspaceController(urlState);
  const { preference, setPreference } = useThemePreference();

  if (workspaceId.length === 0) {
    return (
      <StarterScreen
        createError={controller.starter.createError}
        isCreating={controller.starter.isCreating}
        isLoadingRecent={controller.starter.isLoadingRecent}
        onRootPathChange={controller.starter.onRootPathChange}
        onSelectWorkspace={controller.starter.onSelectWorkspace}
        onSubmit={controller.starter.onSubmit}
        recentError={controller.starter.recentError}
        recentWorkspaces={controller.starter.recentWorkspaces}
        rootPath={controller.starter.rootPath}
        themeControl={
          <ThemeSwitcher onChange={setPreference} value={preference} />
        }
      />
    );
  }

  return (
    <AppShell
      mode="workspace"
      workspace={controller.workspace.data}
      workspaceId={workspaceId}
    >
      <WorkspaceLayout
        activityRail={
          <ActivityRail
            activePanel={panel}
            onPanelChange={controller.workspace.onPanelChange}
            workspaceId={workspaceId}
          />
        }
        bottomPanel={
          <WorkspaceBottomPanel
            activeCommand={controller.command.activeCommand}
            activePanel={panel}
            commandOutput={controller.command.output}
            gitRawStatus={controller.git.rawStatus}
            isGitLoading={controller.git.isLoading}
            previewPorts={controller.preview.ports}
            selectedPath={selectedPath}
          />
        }
        mainPanels={
          <WorkspaceMainPanels
            activePanel={panel}
            command={controller.command}
            files={controller.files}
            git={controller.git}
            preview={controller.preview}
            selectedPath={selectedPath}
          />
        }
        sidebar={
          <WorkspaceSidebar
            activePanel={panel}
            files={controller.files.entries}
            filesError={controller.files.error}
            gitChanges={controller.git.changes}
            gitCommitError={controller.git.commitError}
            gitCommitMessage={controller.git.commitMessage}
            gitError={controller.git.error}
            gitLastCommitHash={controller.git.lastCommitHash}
            gitStageError={controller.git.stageError}
            isDiscardingChanges={controller.git.isDiscardingChanges}
            isExposingPort={controller.preview.isExposing}
            isFilesLoading={controller.files.isLoading}
            isGitCommitPending={controller.git.isCommitPending}
            isGitLoading={controller.git.isLoading}
            isLoadingPorts={controller.preview.isLoading}
            exposingPort={controller.preview.exposingPort}
            isRefreshingFiles={controller.files.isRefreshing}
            isStagingChanges={controller.git.isStagingChanges}
            isUnstagingChanges={controller.git.isUnstagingChanges}
            onChangesDiscard={controller.git.onChangesDiscard}
            onChangesStage={controller.git.onChangesStage}
            onFileIndexRefresh={controller.files.onRefresh}
            onGitCommitMessageChange={controller.git.onCommitMessageChange}
            onGitCommitSubmit={controller.git.onCommitSubmit}
            onPathSelect={controller.workspace.onPathSelect}
            onPortExpose={controller.preview.onExpose}
            onStagedChangesUnstage={controller.git.onStagedChangesUnstage}
            portExposeError={controller.preview.exposeError}
            ports={controller.preview.ports}
            portsError={controller.preview.error}
            selectedPath={selectedPath}
            workspace={controller.workspace.data}
            workspaceError={controller.workspace.error}
          />
        }
      />
    </AppShell>
  );
}
