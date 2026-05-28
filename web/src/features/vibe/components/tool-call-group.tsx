import { ChevronDown } from "lucide-react";
import type { ReactNode } from "react";
import { useEffect, useId, useState } from "react";

import type { AgentToolCall } from "@/shared/api";

import { toolCallDisplay, toolCallNeedsAttention } from "../lib/tool-calls";

export function ToolCallGroup({
  children,
  toolCalls,
}: {
  children: ReactNode;
  toolCalls: AgentToolCall[];
}) {
  const defaultOpen = toolCalls.some(toolCallNeedsAttention);
  const [isOpen, setIsOpen] = useState(defaultOpen);
  const contentId = useId();
  const status = groupStatus(toolCalls);
  const displays = toolCalls.map(toolCallDisplay);
  const firstDisplay = displays[0];
  const Icon = firstDisplay?.Icon;
  const isRunning = toolCalls.some((toolCall) => toolCall.status === "running");

  useEffect(() => {
    if (!defaultOpen) {
      return;
    }

    const timeoutId = window.setTimeout(() => {
      setIsOpen(true);
    }, 0);

    return () => window.clearTimeout(timeoutId);
  }, [defaultOpen]);

  return (
    <div
      className="bg-surface grid min-w-0 rounded-xl px-3 py-2"
      data-state={isOpen ? "open" : "closed"}
      data-tool-call-group
    >
      <button
        aria-controls={contentId}
        aria-expanded={isOpen}
        className="text-muted hover:text-message flex min-h-7 min-w-0 cursor-pointer items-center gap-2 overflow-hidden text-left transition-colors"
        onClick={() => setIsOpen((current) => !current)}
        type="button"
      >
        {Icon ? (
          <Icon aria-hidden="true" className="size-4 shrink-0 opacity-80" />
        ) : null}
        <div className="min-w-0 flex-1 truncate text-sm">
          <span
            className={
              isRunning ? "pp-shimmer-text font-medium" : "font-medium"
            }
          >
            {status}
          </span>{" "}
          {toolCalls.length} tool calls
        </div>
        <ChevronDown
          aria-hidden="true"
          className={`ml-auto size-3.5 shrink-0 transition ${
            isOpen ? "rotate-180" : ""
          }`}
        />
      </button>
      <div
        aria-hidden={!isOpen}
        className="grid overflow-hidden opacity-0 transition-[grid-template-rows,opacity] duration-200 ease-out data-[state=open]:opacity-100"
        data-state={isOpen ? "open" : "closed"}
        id={contentId}
        inert={!isOpen}
        style={{ gridTemplateRows: isOpen ? "1fr" : "0fr" }}
      >
        <div className="min-h-0 overflow-hidden">
          <div className="grid gap-1 pt-2 pb-0.5">{children}</div>
        </div>
      </div>
    </div>
  );
}

function groupStatus(toolCalls: AgentToolCall[]) {
  if (toolCalls.some((toolCall) => toolCall.status === "failed")) {
    return "Failed";
  }
  if (toolCalls.some((toolCall) => toolCall.status === "waiting_approval")) {
    return "Waiting approval";
  }
  if (toolCalls.some((toolCall) => toolCall.status === "running")) {
    return "Running";
  }
  if (toolCalls.every((toolCall) => toolCall.status === "finished")) {
    return "Finished";
  }
  if (toolCalls.every((toolCall) => toolCall.status === "rejected")) {
    return "Rejected";
  }
  return "Mixed";
}
