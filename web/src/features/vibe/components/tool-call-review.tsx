import { Check, ChevronDown, CircleAlert, X } from "lucide-react";
import { useEffect, useId, useState } from "react";

import type { AgentToolCall } from "@/shared/api";
import { DiffViewer } from "@/shared/diff/diff-viewer";
import { Button } from "@/shared/ui";

import { humanizeSkillName } from "../lib/skills";
import {
  parseToolInput,
  toolCallDisplay,
  toolCallNeedsAttention,
} from "../lib/tool-calls";

export function ToolCallReview({
  approvalError,
  compact = false,
  isApproving,
  isCurrentApproval,
  isRejecting,
  onApprove,
  onReject,
  rejectError,
  showIcon = true,
  toolCall,
}: {
  approvalError?: string;
  compact?: boolean;
  isApproving: boolean;
  isCurrentApproval: boolean;
  isRejecting: boolean;
  onApprove: () => void;
  onReject: () => void;
  rejectError?: string;
  showIcon?: boolean;
  toolCall: AgentToolCall;
}) {
  const canDecide =
    isCurrentApproval &&
    toolCall.requiresApproval &&
    toolCall.status === "waiting_approval";
  const error = approvalError ?? rejectError;
  const input = parseToolInput(toolCall.input);
  const summary = typeof input.summary === "string" ? input.summary : "";
  const display = toolCallDisplay(toolCall);
  const source = toolCall.source ?? "builtin";
  const defaultOpen = toolCallNeedsAttention(toolCall);
  const [isOpen, setIsOpen] = useState(defaultOpen);
  const contentId = useId();
  const rowSizeClass = compact ? "min-h-5 py-0" : "min-h-6 py-0";

  useEffect(() => {
    if (!defaultOpen) {
      return;
    }

    const timeoutId = window.setTimeout(() => {
      setIsOpen(true);
    }, 0);

    return () => window.clearTimeout(timeoutId);
  }, [defaultOpen]);

  const details = (
    <div className={compact ? "grid gap-2 pt-1 pb-2" : "grid gap-3 pt-3 pb-3"}>
      {summary ? (
        <p className="text-muted text-sm whitespace-pre-wrap">{summary}</p>
      ) : null}
      {display.detail && toolCall.name === "apply_patch" ? (
        <div className="border-line/35 max-h-80 overflow-auto rounded-xl border">
          <DiffViewer diff={display.detail} wrapLines />
        </div>
      ) : display.detail ? (
        <pre className="bg-panel text-muted max-h-64 overflow-auto rounded-xl px-3 py-2 text-xs whitespace-pre-wrap">
          {display.detail}
        </pre>
      ) : null}
      {source !== "builtin" || toolCall.policyReason || display.sourceLabel ? (
        <div className="text-muted grid gap-1 text-xs">
          <p>
            Source:{" "}
            {display.sourceLabel ??
              toolCallSourceLabel(source, toolCall.sourceRef)}
          </p>
          {toolCall.policyReason ? <p>{toolCall.policyReason}</p> : null}
        </div>
      ) : null}
      {error ? (
        <p className="text-warning text-sm font-medium">{error}</p>
      ) : null}
      {toolCall.requiresApproval &&
      (isCurrentApproval || toolCall.status !== "waiting_approval") ? (
        <div className="flex flex-wrap gap-2">
          <Button
            disabled={!canDecide || isApproving}
            icon={<Check />}
            onClick={onApprove}
            size="small"
            type="button"
          >
            Approve tool
          </Button>
          <Button
            disabled={!canDecide || isRejecting}
            icon={<X />}
            onClick={onReject}
            size="small"
            type="button"
            variant="secondary"
          >
            Reject
          </Button>
        </div>
      ) : null}
      {toolCall.requiresApproval &&
      toolCall.status === "waiting_approval" &&
      !isCurrentApproval ? (
        <p className="text-muted text-xs">
          Waiting for the previous tool decision.
        </p>
      ) : null}
    </div>
  );

  if (!display.expandable) {
    return (
      <div className={`text-muted flex min-w-0 items-center ${rowSizeClass}`}>
        <ToolCallSummaryRow
          display={display}
          expandable={false}
          isRunning={toolCall.status === "running"}
          needsApprovalAttention={toolCall.status === "waiting_approval"}
          showIcon={showIcon}
        />
      </div>
    );
  }

  return (
    <div
      className={
        compact
          ? "grid min-w-0"
          : "bg-surface grid min-w-0 rounded-xl px-3 py-2"
      }
      data-state={isOpen ? "open" : "closed"}
      data-tool-call
    >
      <button
        aria-controls={contentId}
        aria-expanded={isOpen}
        className={`text-muted hover:text-message flex min-w-0 cursor-pointer items-center overflow-hidden text-left transition-colors ${rowSizeClass}`}
        onClick={() => setIsOpen((current) => !current)}
        type="button"
      >
        <ToolCallSummaryRow
          display={display}
          expandable
          isOpen={isOpen}
          isRunning={toolCall.status === "running"}
          needsApprovalAttention={toolCall.status === "waiting_approval"}
          showIcon={showIcon}
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
        <div className="min-h-0 overflow-hidden">{details}</div>
      </div>
    </div>
  );
}

function toolCallSourceLabel(source: string, sourceRef?: string | null) {
  if (!sourceRef) {
    return source;
  }
  if (source === "skill") {
    return `${source}/${humanizeSkillName(sourceRef)}`;
  }
  return `${source}/${sourceRef}`;
}

function ToolCallSummaryRow({
  display,
  expandable,
  isOpen = false,
  isRunning,
  needsApprovalAttention,
  showIcon,
}: {
  display: ReturnType<typeof toolCallDisplay>;
  expandable: boolean;
  isOpen?: boolean;
  isRunning: boolean;
  needsApprovalAttention: boolean;
  showIcon: boolean;
}) {
  const { Icon } = display;

  return (
    <div className="flex min-w-0 flex-1 items-center gap-2 overflow-hidden text-inherit">
      {showIcon ? (
        <Icon aria-hidden="true" className="size-4 shrink-0 opacity-80" />
      ) : null}
      <span className="min-w-0 flex-1 truncate text-sm">
        {needsApprovalAttention ? (
          <CircleAlert
            aria-hidden="true"
            className="text-warning mr-1 inline size-3.5 align-[-0.15em]"
            data-approval-attention-icon="true"
          />
        ) : null}
        <span
          className={isRunning ? "pp-shimmer-text font-medium" : "font-medium"}
        >
          {display.statusLabel}
        </span>{" "}
        {display.text}
      </span>
      {expandable ? (
        <ChevronDown
          aria-hidden="true"
          className={`ml-auto size-3.5 shrink-0 transition ${
            isOpen ? "rotate-180" : ""
          }`}
        />
      ) : null}
    </div>
  );
}
