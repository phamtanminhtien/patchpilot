import type { AgentToolCall } from "@/shared/api";

export function parseToolInput(input: string): Record<string, unknown> {
  try {
    const parsed = JSON.parse(input) as unknown;
    return parsed && typeof parsed === "object"
      ? (parsed as Record<string, unknown>)
      : {};
  } catch {
    return {};
  }
}

export function nextApprovalToolCall(toolCalls: AgentToolCall[]) {
  return [...toolCalls]
    .filter(
      (toolCall) =>
        toolCall.requiresApproval && toolCall.status === "waiting_approval",
    )
    .sort((left, right) =>
      left.batchId === right.batchId
        ? left.sequence - right.sequence
        : left.createdAt.localeCompare(right.createdAt),
    )[0];
}
