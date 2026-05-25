import { apiClient } from "./client";
import type { AgentContextSnapshot, AgentSkillResponse } from "./types";

export async function getAgentContext(
  workspaceId: string,
): Promise<AgentContextSnapshot> {
  const response = await apiClient.get<AgentContextSnapshot>(
    `/workspaces/${workspaceId}/agent/context`,
  );
  return response.data;
}

export async function refreshAgentContext(
  workspaceId: string,
): Promise<AgentContextSnapshot> {
  const response = await apiClient.post<AgentContextSnapshot>(
    `/workspaces/${workspaceId}/agent/context/refresh`,
  );
  return response.data;
}

export async function setAgentSkillEnabled(
  workspaceId: string,
  skillKey: string,
  enabled: boolean,
): Promise<AgentSkillResponse> {
  const response = await apiClient.patch<AgentSkillResponse>(
    `/workspaces/${workspaceId}/skills/${encodeURIComponent(skillKey)}`,
    { enabled },
  );
  return response.data;
}
