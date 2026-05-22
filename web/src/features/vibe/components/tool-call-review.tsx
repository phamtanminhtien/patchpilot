import { Check, X } from "lucide-react";

import type { AgentToolCall } from "@/shared/api";
import { Button, StatusPill } from "@/shared/ui";

import { parseToolInput } from "../lib/tool-calls";

export function ToolCallReview({
  approvalError,
  isApproving,
  isCurrentApproval,
  isRejecting,
  onApprove,
  onReject,
  toolCall,
  rejectError,
}: {
  approvalError?: string;
  isApproving: boolean;
  isCurrentApproval: boolean;
  isRejecting: boolean;
  onApprove: () => void;
  onReject: () => void;
  toolCall: AgentToolCall;
  rejectError?: string;
}) {
  const canDecide =
    isCurrentApproval &&
    toolCall.requiresApproval &&
    toolCall.status === "waiting_approval";
  const error = approvalError ?? rejectError;
  const input = parseToolInput(toolCall.input);
  const diff = typeof input.diff === "string" ? input.diff : "";
  const summary = typeof input.summary === "string" ? input.summary : "";

  return (
    <details className="bg-panel group grid min-w-0 rounded-md shadow-sm">
      <summary className="hover:bg-hover flex min-h-10 cursor-pointer list-none items-center justify-between gap-3 rounded-md px-3 py-2">
        <div className="min-w-0">
          <p className="text-ink truncate text-sm font-semibold">
            {toolCall.name}
          </p>
          <p className="text-muted text-xs">
            Batch {toolCall.sequence + 1} · {toolCall.status}
          </p>
        </div>
        <StatusPill status={toolCall.status} />
      </summary>
      <div className="grid gap-3 px-3 pb-3">
        {summary ? (
          <p className="text-muted text-sm whitespace-pre-wrap">{summary}</p>
        ) : null}
        <pre className="bg-hover text-ink max-h-64 overflow-auto rounded-sm p-3 text-xs whitespace-pre-wrap">
          {diff || toolCall.output || toolCall.input}
        </pre>
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
    </details>
  );
}
