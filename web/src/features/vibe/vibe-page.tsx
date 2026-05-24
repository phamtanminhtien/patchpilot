import { useState } from "react";

import { useThemePreference } from "@/app/theme";
import { StarterScreen, ThemeSwitcher } from "@/shared/ui";

import { AgentRunThread } from "./components/agent-run-thread";
import { Composer } from "./components/composer";
import { ConversationSearchDialog } from "./components/conversation-search-dialog";
import { useSmartAutoScroll } from "./hooks/use-smart-auto-scroll";
import { useVibeController } from "./hooks/use-vibe-controller";
import { VibeConversationSidebar } from "./layout/vibe-conversation-sidebar";
import { VibeWorkspaceLayout } from "./layout/vibe-workspace-layout";

export function VibePage() {
  const controller = useVibeController();
  const [isConversationSearchOpen, setIsConversationSearchOpen] =
    useState(false);
  const { preference, setPreference } = useThemePreference();
  const scroll = useSmartAutoScroll({
    contentKey: [
      controller.conversation.activeConversationId,
      controller.thread.messages.length,
      controller.thread.messages
        .map((item) => `${item.id}:${item.runId ?? ""}`)
        .join("|"),
      controller.thread.toolCalls
        .map(
          (item) =>
            `${item.id}:${item.status}:${item.decision ?? ""}:${item.finishedAt ?? ""}`,
        )
        .join("|"),
      controller.thread.runs
        .map((item) => `${item.id}:${item.status}:${item.updatedAt}`)
        .join("|"),
      controller.thread.events.map((item) => item.id).join("|"),
    ].join("::"),
    resetKey: controller.conversation.activeConversationId,
  });

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
      onJumpToLatest={() => scroll.scrollToLatest()}
      onSearchConversations={() => setIsConversationSearchOpen(true)}
      onScroll={scroll.handleScroll}
      scrollContainerRef={scroll.scrollContainerRef}
      sidebar={
        <VibeConversationSidebar
          activeConversationId={controller.conversation.activeConversationId}
          conversations={controller.conversation.conversations}
          isLoading={controller.conversation.isListLoading}
          onNewConversation={controller.conversation.onNewConversation}
          onSearchConversations={() => setIsConversationSearchOpen(true)}
          onSelectConversation={controller.conversation.onSelectConversation}
          workspaceId={controller.workspace.id}
        />
      }
      showJumpToLatest={scroll.showJumpToLatest}
      title={controller.conversation.title}
      workspaceId={controller.workspace.id}
    >
      {controller.conversation.hasConversation ? (
        <AgentRunThread
          {...controller.thread}
          bottomAnchorRef={scroll.bottomAnchorRef}
          isLoading={controller.conversation.isLoading}
        />
      ) : (
        <div className="grid min-h-full place-items-center">
          <div className="w-full max-w-3xl">
            <Composer {...controller.composer} />
          </div>
        </div>
      )}
      <ConversationSearchDialog
        activeConversationId={controller.conversation.activeConversationId}
        onOpenChange={setIsConversationSearchOpen}
        onSelectConversation={controller.conversation.onSelectConversation}
        open={isConversationSearchOpen}
        workspaceId={controller.workspace.id}
      />
    </VibeWorkspaceLayout>
  );
}
