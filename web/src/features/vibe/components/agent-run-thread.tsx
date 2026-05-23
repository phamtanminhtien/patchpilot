import type {
  AgentRun,
  AgentRunEvent,
  AgentToolCall,
  Message,
} from "@/shared/api";

import {
  assistantMessagesForRun,
  assistantTextFromEvents,
  promptForRun,
} from "../lib/run-text";
import { nextApprovalToolCall } from "../lib/tool-calls";
import { ThinkingIndicator } from "./thinking-indicator";
import { ToolCallReview } from "./tool-call-review";

export function AgentRunThread({
  approvalError,
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
  const activeApprovalId = nextApprovalToolCall(toolCalls)?.id ?? "";
  const sortedRuns = [...runs].sort(
    (first, second) =>
      new Date(first.createdAt).getTime() -
      new Date(second.createdAt).getTime(),
  );

  const renderToolCallReview = (toolCall: AgentToolCall) => (
    <ToolCallReview
      approvalError={approvalError}
      isApproving={isApproving}
      isCurrentApproval={toolCall.id === activeApprovalId}
      isRejecting={isRejecting}
      key={toolCall.id}
      onApprove={() => onToolApprove(toolCall.runId, toolCall.id)}
      onReject={() => onToolReject(toolCall.runId, toolCall.id)}
      rejectError={rejectError}
      toolCall={toolCall}
    />
  );

  return (
    <div className="mx-auto grid min-h-full w-full max-w-3xl content-end gap-6 pt-2 pb-6">
      <div className="grid gap-6">
        {createError ? (
          <p className="text-warning text-sm font-medium">{createError}</p>
        ) : null}
        {sortedRuns.length > 0 ? (
          sortedRuns.map((run) => {
            const runEvents = events.filter((event) => event.runId === run.id);
            const runToolCalls = toolCalls.filter(
              (toolCall) => toolCall.runId === run.id,
            );
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
            const fallbackAssistantText =
              assistantMessages.length === 0
                ? assistantTextFromEvents(runEvents) ||
                  (run.status === "done" ? run.summary.trim() : "")
                : "";
            const isThinking =
              run.status === "queued" ||
              run.status === "running" ||
              run.status === "waiting_tool_approval";
            const prompt = promptForRun(messages, run);

            return (
              <div className="grid gap-6" key={run.id}>
                {prompt ? (
                  <div className="flex justify-end">
                    <p className="bg-hover text-ink max-w-[75%] rounded-lg px-4 py-3 text-sm leading-6 whitespace-pre-wrap">
                      {prompt}
                    </p>
                  </div>
                ) : null}
                {timelineItems.map((item) =>
                  item.kind === "assistant" ? (
                    <div className="grid w-full gap-2" key={item.id}>
                      <p className="text-ink text-sm leading-6 whitespace-pre-wrap">
                        {item.item.content}
                      </p>
                    </div>
                  ) : (
                    <div className="grid w-full gap-2" key={item.id}>
                      {renderToolCallReview(item.item)}
                    </div>
                  ),
                )}
                {fallbackAssistantText ? (
                  <div className="grid w-full gap-2">
                    <p className="text-ink text-sm leading-6 whitespace-pre-wrap">
                      {fallbackAssistantText}
                    </p>
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
      </div>
    </div>
  );
}
