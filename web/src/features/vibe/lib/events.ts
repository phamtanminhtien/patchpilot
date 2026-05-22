import type { AgentRunEvent, WorkspaceEvent } from "@/shared/api";

export function appendRunEvent(events: AgentRunEvent[], event: WorkspaceEvent) {
  if (events.some((item) => item.id === event.id) || !isAgentRunEvent(event)) {
    return events;
  }
  return [
    ...events,
    {
      createdAt: event.createdAt,
      id: event.id,
      payload: event.payload,
      runId: eventRunId(event),
      type: event.type,
      workspaceId: event.workspaceId,
    },
  ];
}

export function isAgentRunEvent(
  event: WorkspaceEvent,
): event is WorkspaceEvent & {
  type: AgentRunEvent["type"];
} {
  return event.type.startsWith("agent.");
}

export function eventRunId(event: WorkspaceEvent) {
  const payload = event.payload as Record<string, unknown>;
  if (typeof payload.runId === "string") {
    return payload.runId;
  }
  if (typeof payload.id === "string" && payload.id.startsWith("run_")) {
    return payload.id;
  }
  return "";
}
