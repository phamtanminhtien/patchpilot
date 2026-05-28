import type { Ref } from "react";

import type {
  AgentRun,
  AgentRunEvent,
  AgentToolCall,
  Message,
} from "@/shared/api";
import { Markdown } from "@/shared/ui";

import {
  assistantMessagesForRun,
  assistantTextFromEvents,
  assistantTextFromEventsAfter,
  promptForRun,
} from "../lib/run-text";
import { nextApprovalToolCall } from "../lib/tool-calls";
import { ThinkingIndicator } from "./thinking-indicator";
import { ToolCallGroup } from "./tool-call-group";
import { ToolCallReview } from "./tool-call-review";

type TimelineItem =
  | {
      createdAt: string;
      id: string;
      item: Message;
      kind: "assistant";
    }
  | {
      createdAt: string;
      id: string;
      item: AgentToolCall;
      kind: "tool_call";
    };

type TimelineDisplayItem =
  | TimelineItem
  | {
      id: string;
      items: AgentToolCall[];
      kind: "tool_call_group";
    };

export function AgentRunThread({
  approvalError,
  bottomAnchorRef,
  createError,
  events,
  isApproving,
  isLoading,
  isRejecting,
  messages,
  onToolApprove,
  onToolReject,
  rejectError,
  runs,
  toolCalls,
}: {
  approvalError?: string;
  bottomAnchorRef?: Ref<HTMLDivElement>;
  createError?: string;
  events: AgentRunEvent[];
  isApproving: boolean;
  isLoading: boolean;
  isRejecting: boolean;
  messages: Message[];
  onToolApprove: (runId: string, toolCallId: string) => void;
  onToolReject: (runId: string, toolCallId: string) => void;
  rejectError?: string;
  runs: AgentRun[];
  toolCalls: AgentToolCall[];
}) {
  const sortedRuns = [...runs].sort(
    (first, second) =>
      new Date(first.createdAt).getTime() -
      new Date(second.createdAt).getTime(),
  );

  const renderToolCallReview = (
    toolCall: AgentToolCall,
    activeApprovalId: string,
    options: { compact?: boolean; showIcon?: boolean } = {},
  ) => (
    <ToolCallReview
      approvalError={approvalError}
      isApproving={isApproving}
      isCurrentApproval={toolCall.id === activeApprovalId}
      isRejecting={isRejecting}
      key={toolCall.id}
      onApprove={() => onToolApprove(toolCall.runId, toolCall.id)}
      onReject={() => onToolReject(toolCall.runId, toolCall.id)}
      rejectError={rejectError}
      compact={options.compact}
      showIcon={options.showIcon}
      toolCall={toolCall}
    />
  );

  return (
    <div className="mx-auto grid min-h-full w-full max-w-3xl content-end gap-6 pt-2">
      <div className="grid gap-3">
        {createError ? (
          <p className="text-warning text-sm font-medium">{createError}</p>
        ) : null}
        {sortedRuns.length > 0 ? (
          sortedRuns.map((run) => {
            const runEvents = events.filter((event) => event.runId === run.id);
            const runToolCalls = toolCalls.filter(
              (toolCall) => toolCall.runId === run.id,
            );
            const activeApprovalId =
              nextApprovalToolCall(runToolCalls)?.id ?? "";
            const assistantMessages = assistantMessagesForRun(messages, run);
            const timelineItems = [
              ...assistantMessages.map((message) => ({
                createdAt: message.createdAt,
                id: message.id,
                item: message,
                kind: "assistant" as const,
              })),
              ...runToolCalls.map((toolCall) => ({
                createdAt: toolCall.createdAt,
                id: toolCall.id,
                item: toolCall,
                kind: "tool_call" as const,
              })),
            ].sort((first, second) => {
              const createdAtOrder = first.createdAt.localeCompare(
                second.createdAt,
              );
              if (createdAtOrder !== 0) {
                return createdAtOrder;
              }
              const kindOrder =
                first.kind === second.kind
                  ? 0
                  : first.kind === "assistant"
                    ? -1
                    : 1;
              if (kindOrder !== 0) {
                return kindOrder;
              }
              return first.id.localeCompare(second.id);
            });
            const displayItems = groupConsecutiveToolCalls(timelineItems);
            const isThinking =
              run.status === "queued" ||
              run.status === "running" ||
              run.status === "waiting_tool_approval";
            const lastAssistantMessageAt =
              assistantMessages.at(-1)?.createdAt ?? "";
            const streamingAssistantText = isThinking
              ? assistantTextFromEventsAfter(runEvents, lastAssistantMessageAt)
              : "";
            const fallbackAssistantText =
              assistantMessages.length === 0 && streamingAssistantText === ""
                ? assistantTextFromEvents(runEvents) ||
                  (run.status === "done" ? run.summary.trim() : "")
                : "";
            const prompt = promptForRun(messages, run);

            return (
              <div className="grid gap-3" key={run.id}>
                {prompt ? (
                  <div className="flex justify-end">
                    <p className="bg-surface text-message max-w-[80%] rounded-xl px-3 py-2 text-sm leading-6 whitespace-pre-wrap">
                      {prompt}
                    </p>
                  </div>
                ) : null}
                {displayItems.map((item) =>
                  item.kind === "assistant" ? (
                    <div
                      className="grid w-full gap-2 rounded-xl px-1 py-1"
                      key={item.id}
                    >
                      <Markdown className="text-message">
                        {item.item.content}
                      </Markdown>
                    </div>
                  ) : item.kind === "tool_call" ? (
                    <div className="grid w-full gap-2" key={item.id}>
                      {renderToolCallReview(item.item, activeApprovalId)}
                    </div>
                  ) : (
                    <div className="grid w-full gap-2" key={item.id}>
                      <ToolCallGroup toolCalls={item.items}>
                        {item.items.map((toolCall) =>
                          renderToolCallReview(toolCall, activeApprovalId, {
                            compact: true,
                            showIcon: false,
                          }),
                        )}
                      </ToolCallGroup>
                    </div>
                  ),
                )}
                {fallbackAssistantText ? (
                  <div className="grid w-full gap-2 rounded-xl px-1 py-1">
                    <Markdown className="text-message">
                      {fallbackAssistantText}
                    </Markdown>
                  </div>
                ) : null}
                {streamingAssistantText ? (
                  <div className="grid w-full gap-2 rounded-xl px-1 py-1">
                    <Markdown className="text-message">
                      {streamingAssistantText}
                    </Markdown>
                  </div>
                ) : null}
                {isThinking ? <ThinkingIndicator status={run.status} /> : null}
                {run.error ? (
                  <p className="text-warning text-sm font-medium">
                    {run.error}
                  </p>
                ) : null}
              </div>
            );
          })
        ) : (
          <>
            {isLoading ? (
              <p className="text-muted text-sm">Loading conversation.</p>
            ) : null}
          </>
        )}
        <div aria-hidden="true" ref={bottomAnchorRef} />
      </div>
    </div>
  );
}

function groupConsecutiveToolCalls(
  timelineItems: TimelineItem[],
): TimelineDisplayItem[] {
  const displayItems: TimelineDisplayItem[] = [];
  let currentGroup: AgentToolCall[] = [];

  const flushToolCalls = () => {
    if (currentGroup.length === 1) {
      const toolCall = currentGroup[0];
      if (toolCall) {
        displayItems.push({
          createdAt: toolCall.createdAt,
          id: toolCall.id,
          item: toolCall,
          kind: "tool_call",
        });
      }
    } else if (currentGroup.length > 1) {
      displayItems.push({
        id: currentGroup.map((toolCall) => toolCall.id).join("-"),
        items: currentGroup,
        kind: "tool_call_group",
      });
    }
    currentGroup = [];
  };

  for (const item of timelineItems) {
    if (item.kind === "tool_call") {
      currentGroup.push(item.item);
      continue;
    }
    flushToolCalls();
    displayItems.push(item);
  }

  flushToolCalls();

  return displayItems;
}
