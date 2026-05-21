import { apiClient } from "./client";
import type { Command, CommandDetail, CommandListResponse } from "./types";

export async function queueCommand(
  workspaceId: string,
  command: string,
  confirmed = false,
): Promise<Command> {
  const response = await apiClient.post<Command>(
    `/workspaces/${workspaceId}/commands`,
    { command, confirmed },
  );
  return response.data;
}

export async function listProcesses(
  workspaceId: string,
): Promise<CommandListResponse> {
  const response = await apiClient.get<CommandListResponse>(
    `/workspaces/${workspaceId}/processes`,
  );
  return response.data;
}

export async function getProcess(
  workspaceId: string,
  processId: string,
): Promise<CommandDetail> {
  const response = await apiClient.get<CommandDetail>(
    `/workspaces/${workspaceId}/processes/${processId}`,
  );
  return response.data;
}

export async function stopProcess(
  workspaceId: string,
  processId: string,
): Promise<Command> {
  const response = await apiClient.post<Command>(
    `/workspaces/${workspaceId}/processes/${processId}/stop`,
  );
  return response.data;
}
