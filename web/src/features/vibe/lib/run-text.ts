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

export function titleFromPrompt(prompt: string) {
  return prompt.length > 80 ? `${prompt.slice(0, 77)}...` : prompt;
}

export function assistantTextFromEvents(events: AgentRunEvent[]) {
  return events
    .filter((event) => event.type === "agent.delta")
    .map((event) => {
      const payload = event.payload as Record<string, unknown>;
      return typeof payload.text === "string" ? payload.text.trim() : "";
    })
    .filter(Boolean)
    .join("\n\n");
}
