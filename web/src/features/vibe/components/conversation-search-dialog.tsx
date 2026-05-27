import { useQuery } from "@tanstack/react-query";
import { Loader2 } from "lucide-react";
import { useState } from "react";

import {
  apiErrorMessage,
  type Conversation,
  listConversations,
} from "@/shared/api";
import {
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogRoot,
  DialogTitle,
  TextField,
} from "@/shared/ui";

import { timeAgo } from "../lib/time";

export function ConversationSearchDialog({
  activeConversationId,
  onOpenChange,
  onSelectConversation,
  open,
  workspaceId,
}: {
  activeConversationId: string;
  onOpenChange: (open: boolean) => void;
  onSelectConversation: (conversationId: string) => void;
  open: boolean;
  workspaceId: string;
}) {
  const [query, setQuery] = useState("");
  const trimmedQuery = query.trim();
  const conversationsQuery = useQuery({
    enabled: open && workspaceId.length > 0 && trimmedQuery.length > 0,
    queryFn: () =>
      listConversations(workspaceId, { limit: 50, q: trimmedQuery }),
    queryKey: ["conversation-search", workspaceId, trimmedQuery],
  });
  const conversations = conversationsQuery.data?.conversations ?? [];

  function handleSelect(conversation: Conversation) {
    onSelectConversation(conversation.id);
    onOpenChange(false);
  }

  return (
    <DialogRoot onOpenChange={onOpenChange} open={open}>
      <DialogContent className="max-h-[min(34rem,calc(100vh-2rem))] grid-rows-[auto_auto_minmax(0,1fr)] gap-0 overflow-hidden p-0">
        <DialogHeader className="px-3 py-3 sm:px-4">
          <DialogTitle>Search conversations</DialogTitle>
          <DialogDescription>
            Find a workspace conversation by title.
          </DialogDescription>
        </DialogHeader>

        <div className="px-3 pt-3 pb-2 sm:px-4">
          <TextField
            autoFocus
            className="bg-panel"
            label="Search conversations"
            labelHidden
            onChange={(event) => setQuery(event.target.value)}
            placeholder="Search conversations"
            size="compact"
            value={query}
          />
        </div>

        <div
          aria-label="Conversation search results"
          className="min-h-0 min-w-0 overflow-auto px-3 pb-3 sm:px-4 sm:pb-4"
          role="region"
        >
          {trimmedQuery.length === 0 ? (
            <p className="text-muted px-1 py-6 text-center text-sm">
              Search by conversation title.
            </p>
          ) : null}
          {trimmedQuery.length > 0 && conversationsQuery.isFetching ? (
            <p className="text-muted flex min-h-20 items-center justify-center gap-2 text-sm">
              <Loader2 aria-hidden="true" className="size-4 animate-spin" />
              Searching conversations
            </p>
          ) : null}
          {trimmedQuery.length > 0 && conversationsQuery.error ? (
            <p className="text-warning px-1 py-6 text-center text-sm">
              {apiErrorMessage(conversationsQuery.error)}
            </p>
          ) : null}
          {trimmedQuery.length > 0 &&
          !conversationsQuery.isFetching &&
          !conversationsQuery.error &&
          conversations.length === 0 ? (
            <p className="text-muted px-1 py-6 text-center text-sm">
              No matching conversations.
            </p>
          ) : null}
          {trimmedQuery.length > 0 &&
          !conversationsQuery.isFetching &&
          !conversationsQuery.error &&
          conversations.length > 0 ? (
            <div className="grid gap-1">
              {conversations.map((conversation) => (
                <button
                  aria-current={
                    conversation.id === activeConversationId
                      ? "page"
                      : undefined
                  }
                  className="hover:bg-hover aria-[current=page]:bg-surface grid min-h-11 min-w-0 cursor-pointer grid-cols-[minmax(0,1fr)_auto] items-center gap-3 rounded-xl px-3 py-2 text-left transition"
                  key={conversation.id}
                  onClick={() => handleSelect(conversation)}
                  type="button"
                >
                  <span className="text-ink min-w-0 truncate text-sm font-medium">
                    {conversation.title}
                  </span>
                  <span className="text-muted flex min-w-10 shrink-0 justify-end text-xs">
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
          ) : null}
        </div>
      </DialogContent>
    </DialogRoot>
  );
}
