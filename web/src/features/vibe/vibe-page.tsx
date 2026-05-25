import { RefreshCw } from "lucide-react";
import { useState } from "react";

import { useThemePreference } from "@/app/theme";
import {
  Button,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogRoot,
  DialogTitle,
  StarterScreen,
  ThemeSwitcher,
} from "@/shared/ui";

import { AgentRunThread } from "./components/agent-run-thread";
import { Composer } from "./components/composer";
import { ContextCockpit } from "./components/context-cockpit";
import { ConversationSearchDialog } from "./components/conversation-search-dialog";
import { SkillsDialog } from "./components/skills-dialog";
import { useSmartAutoScroll } from "./hooks/use-smart-auto-scroll";
import { useVibeController } from "./hooks/use-vibe-controller";
import { VibeConversationSidebar } from "./layout/vibe-conversation-sidebar";
import { VibeWorkspaceLayout } from "./layout/vibe-workspace-layout";

export function VibePage() {
  const controller = useVibeController();
  const [isConversationSearchOpen, setIsConversationSearchOpen] =
    useState(false);
  const [isContextOpen, setIsContextOpen] = useState(false);
  const [isSkillsOpen, setIsSkillsOpen] = useState(false);
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
      controller.thread.events.map(eventContentKey).join("|"),
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
      onOpenContext={() => setIsContextOpen(true)}
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
          onOpenSkills={() => setIsSkillsOpen(true)}
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
      <SkillsDialog
        context={controller.context.data}
        error={controller.context.error}
        isLoading={controller.context.isLoading}
        isOpen={isSkillsOpen}
        isRefreshing={controller.context.isRefreshing}
        isUpdatingSkill={controller.context.isUpdatingSkill}
        onOpenChange={setIsSkillsOpen}
        onRefresh={controller.context.onRefresh}
        onSkillEnabledChange={(skill, enabled) =>
          controller.context.onSkillEnabledChange(skill.key, enabled)
        }
      />
      <DialogRoot onOpenChange={setIsContextOpen} open={isContextOpen}>
        <DialogContent className="grid-rows-[auto_minmax(0,1fr)_auto] gap-0 overflow-hidden p-0 sm:max-h-[min(42rem,calc(100vh-2rem))] sm:max-w-2xl">
          <DialogHeader className="border-line/30 border-b px-5 py-4">
            <DialogTitle>Agent context</DialogTitle>
            <DialogDescription>
              {controller.context.data
                ? `Refreshed ${new Date(controller.context.data.refreshedAt).toLocaleTimeString()}`
                : "Not loaded"}
            </DialogDescription>
          </DialogHeader>
          <div className="min-h-0 overflow-auto px-5 py-4">
            <ContextCockpit
              context={controller.context.data}
              error={controller.context.error}
              isLoading={controller.context.isLoading}
              isUpdatingSkill={controller.context.isUpdatingSkill}
              onSkillEnabledChange={(skill, enabled) =>
                controller.context.onSkillEnabledChange(skill.key, enabled)
              }
            />
          </div>
          <DialogFooter className="border-line/30 border-t px-5 py-3">
            <Button
              disabled={controller.context.isRefreshing}
              icon={
                <RefreshCw
                  className={
                    controller.context.isRefreshing ? "animate-spin" : ""
                  }
                />
              }
              onClick={controller.context.onRefresh}
              size="small"
              type="button"
              variant="surface"
            >
              Refresh
            </Button>
          </DialogFooter>
        </DialogContent>
      </DialogRoot>
    </VibeWorkspaceLayout>
  );
}

function eventContentKey(event: {
  id: string;
  payload: unknown;
  runId?: string | null;
  type: string;
}) {
  const payload = event.payload as Record<string, unknown>;
  const text = typeof payload.text === "string" ? payload.text : "";
  return `${event.id}:${event.type}:${event.runId ?? ""}:${hashText(text)}`;
}

function hashText(text: string) {
  let hash = 0;
  for (let index = 0; index < text.length; index += 1) {
    hash = (hash * 31 + text.charCodeAt(index)) >>> 0;
  }
  return `${text.length}:${hash.toString(36)}`;
}
