import type { AgentRun, AgentRunEvent, Message } from "@/shared/api";

export function promptForRun(messages: Message[], run: AgentRun) {
  return (
    messages.find((message) => message.id === run.triggerMessageId)?.content ??
    ""
  );
}

export function assistantMessagesForRun(messages: Message[], run: AgentRun) {
  return messages
    .filter(
      (message) => message.role === "assistant" && message.runId === run.id,
    )
    .sort(
      (first, second) =>
        first.createdAt.localeCompare(second.createdAt) ||
        first.id.localeCompare(second.id),
    );
}

export function transientAssistantEvent(
  runId: string,
  text: string,
): AgentRunEvent {
  return {
    createdAt: new Date().toISOString(),
    id: `evt_transient_${runId}`,
    payload: { runId, text },
    runId,
    type: "agent.delta",
    workspaceId: "",
  };
}

export function assistantTextFromEvents(events: AgentRunEvent[]) {
  return assistantTextFromEventsAfter(events, "");
}

export function assistantTextFromEventsAfter(
  events: AgentRunEvent[],
  afterCreatedAt: string,
) {
  let text = "";
  for (const event of events) {
    if (
      event.type !== "agent.delta" &&
      event.type !== "agent.output.snapshot"
    ) {
      continue;
    }
    if (afterCreatedAt.length > 0 && event.createdAt <= afterCreatedAt) {
      continue;
    }
    const payload = event.payload as Record<string, unknown>;
    const eventText = typeof payload.text === "string" ? payload.text : "";
    if (event.type === "agent.output.snapshot") {
      text = eventText;
      continue;
    }
    text += eventText;
  }
  return text;
}
