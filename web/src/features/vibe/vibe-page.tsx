import { useThemePreference } from "@/app/theme";
import { StarterScreen, ThemeSwitcher } from "@/shared/ui";

import { AgentRunThread } from "./components/agent-run-thread";
import { Composer } from "./components/composer";
import { useVibeController } from "./hooks/use-vibe-controller";
import { VibeConversationSidebar } from "./layout/vibe-conversation-sidebar";
import { VibeWorkspaceLayout } from "./layout/vibe-workspace-layout";

export function VibePage() {
  const controller = useVibeController();
  const { preference, setPreference } = useThemePreference();

  if (controller.workspace.id.length === 0) {
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
    <VibeWorkspaceLayout
      composer={
        controller.conversation.hasConversation ? (
          <Composer {...controller.composer} />
        ) : (
          <div />
        )
      }
      sidebar={
        <VibeConversationSidebar
          activeConversationId={controller.conversation.activeConversationId}
          conversations={controller.conversation.conversations}
          isLoading={controller.conversation.isListLoading}
          onNewConversation={controller.conversation.onNewConversation}
          onSelectConversation={controller.conversation.onSelectConversation}
          workspaceId={controller.workspace.id}
        />
      }
      title={controller.conversation.title}
      workspaceId={controller.workspace.id}
    >
      {controller.conversation.hasConversation ? (
        <AgentRunThread
          {...controller.thread}
          isLoading={controller.conversation.isLoading}
        />
      ) : (
        <div className="grid min-h-full place-items-center">
          <div className="w-full max-w-3xl">
            <Composer {...controller.composer} />
          </div>
        </div>
      )}
    </VibeWorkspaceLayout>
  );
}
