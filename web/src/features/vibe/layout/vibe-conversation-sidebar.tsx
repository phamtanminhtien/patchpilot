import {
  Code2,
  FolderOpen,
  Loader2,
  MessageSquarePlus,
  Search,
} from "lucide-react";
import { Link } from "react-router";

import type { Conversation } from "@/shared/api";
import { Button, cn } from "@/shared/ui";

import { timeAgo } from "../lib/time";

export function VibeConversationSidebar({
  activeConversationId,
  conversations,
  isLoading,
  onNewConversation,
  onOpenSkills,
  onSearchConversations,
  onSelectConversation,
  workspaceId,
}: {
  activeConversationId: string;
  conversations: Conversation[];
  isLoading: boolean;
  onNewConversation: () => void;
  onOpenSkills: () => void;
  onSearchConversations: () => void;
  onSelectConversation: (conversationId: string) => void;
  workspaceId: string;
}) {
  const navItems = [
    {
      icon: <MessageSquarePlus />,
      label: "New chat",
      onClick: onNewConversation,
    },
    { icon: <Search />, label: "Search", onClick: onSearchConversations },
    { icon: <Code2 />, label: "Skills", onClick: onOpenSkills },
    // { icon: <Clock3 />, label: "Automations" },
  ];

  return (
    <aside className="bg-panel hidden min-h-0 min-w-0 grid-rows-[auto_minmax(0,1fr)_auto] md:grid">
      <nav className="grid gap-1 p-2" aria-label="Vibe navigation">
        <div className="px-2 py-2">
          <p className="text-muted text-xs font-semibold tracking-wide uppercase">
            Vibe
          </p>
          <h2 className="text-ink truncate text-sm font-semibold">
            Conversations
          </h2>
        </div>
        {navItems.map((item) => (
          <button
            className={cn(
              "text-muted hover:bg-hover hover:text-ink flex min-h-9 min-w-0 cursor-pointer items-center gap-2 rounded-xl px-2.5 text-left text-sm transition",
              "disabled:hover:text-muted disabled:cursor-not-allowed disabled:opacity-50 disabled:hover:border-none disabled:hover:bg-transparent",
            )}
            key={item.label}
            onClick={item.onClick}
            type="button"
          >
            <span className="grid size-4 shrink-0 place-items-center [&>svg]:size-4">
              {item.icon}
            </span>
            <span className="truncate">{item.label}</span>
          </button>
        ))}
      </nav>

      <div className="grid min-h-0 gap-4 overflow-hidden py-2">
        <div
          aria-label="Agent conversations"
          className="min-h-0 min-w-0 overflow-auto px-2"
          role="region"
        >
          {conversations.length === 0 ? (
            <p className="text-muted px-2 py-2 text-center text-xs">
              {isLoading ? "Loading conversations" : "No conversations yet."}
            </p>
          ) : (
            <div className="grid gap-1">
              {conversations.map((conversation) => (
                <button
                  aria-current={
                    conversation.id === activeConversationId
                      ? "page"
                      : undefined
                  }
                  className="hover:bg-hover aria-[current=page]:bg-accent-soft aria-[current=page]:text-accent flex min-h-10 min-w-0 cursor-pointer items-center gap-2 rounded-xl px-2.5 py-2 text-left transition"
                  key={conversation.id}
                  onClick={() => onSelectConversation(conversation.id)}
                  type="button"
                >
                  <span className="text-ink min-w-0 flex-1 truncate text-sm">
                    {conversation.title}
                  </span>
                  <span className="text-muted flex w-10 shrink-0 justify-end text-xs">
                    {conversation.hasRunningRun ? (
                      <Loader2
                        aria-label="Conversation run in progress"
                        className="size-3.5 animate-spin"
                      />
                    ) : (
                      timeAgo(conversation.lastMessageAt)
                    )}
                  </span>
                </button>
              ))}
            </div>
          )}
        </div>
      </div>

      <div className="grid gap-2 p-2">
        <Button asChild size="compact" variant="surface" width="full">
          <Link
            to={`/workspace?workspaceId=${encodeURIComponent(workspaceId)}`}
          >
            <FolderOpen aria-hidden="true" className="size-4" />
            Open workspace
          </Link>
        </Button>
      </div>
    </aside>
  );
}
