import {
  Clock3,
  Code2,
  FolderOpen,
  MessageSquarePlus,
  PanelLeft,
  Search,
} from "lucide-react";
import { Link } from "react-router";

import type { Conversation } from "@/shared/api";
import { Button } from "@/shared/ui";

import { timeAgo } from "../lib/time";

export function VibeConversationSidebar({
  activeConversationId,
  conversations,
  isLoading,
  onNewConversation,
  onSelectConversation,
  workspaceId,
}: {
  activeConversationId: string;
  conversations: Conversation[];
  isLoading: boolean;
  onNewConversation: () => void;
  onSelectConversation: (conversationId: string) => void;
  workspaceId: string;
}) {
  const navItems = [
    {
      icon: <MessageSquarePlus />,
      label: "New chat",
      onClick: onNewConversation,
    },
    { icon: <Search />, label: "Search" },
    { icon: <Code2 />, label: "Skills" },
    { icon: <Clock3 />, label: "Automations" },
  ];

  return (
    <aside className="bg-panel hidden min-h-0 min-w-0 grid-rows-[auto_auto_minmax(0,1fr)_auto] gap-5 px-1.5 py-4 md:grid">
      <div className="flex items-center gap-2 px-1">
        <span className="bg-warning size-3 rounded-full" />
        <span className="bg-focus size-3 rounded-full" />
        <span className="bg-accent size-3 rounded-full" />
        <span className="text-muted ml-4">
          <PanelLeft aria-hidden="true" className="size-4" />
        </span>
      </div>

      <nav className="grid gap-1" aria-label="Vibe navigation">
        {navItems.map((item) => (
          <button
            className="text-muted hover:bg-hover hover:text-ink flex min-h-9 min-w-0 cursor-pointer items-center gap-2 rounded-full px-3 text-left text-sm transition"
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

      <div className="grid min-h-0 gap-4 overflow-hidden">
        <div
          aria-label="Agent conversations"
          className="min-h-0 min-w-0 overflow-auto"
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
                  className="hover:bg-hover aria-[current=page]:bg-hover flex min-h-10 min-w-0 cursor-pointer items-center gap-2 rounded-full px-3 py-2 text-left transition"
                  key={conversation.id}
                  onClick={() => onSelectConversation(conversation.id)}
                  type="button"
                >
                  <span className="text-ink min-w-0 flex-1 truncate text-sm">
                    {conversation.title}
                  </span>
                  <span className="text-muted shrink-0 text-xs">
                    {timeAgo(conversation.lastMessageAt)}
                  </span>
                </button>
              ))}
            </div>
          )}
        </div>
      </div>

      <div className="grid gap-2">
        <Button asChild size="compact" variant="ghost" width="full">
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
