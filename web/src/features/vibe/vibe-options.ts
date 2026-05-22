import type { AgentModel, AgentReasoningEffort } from "@/shared/api";

export const agentModels: AgentModel[] = ["gpt-5.5", "gpt-5.4", "gpt-5.4-mini"];

export const reasoningEfforts: AgentReasoningEffort[] = [
  "low",
  "medium",
  "high",
  "xhigh",
];

export const newConversationId = "__new__";
