import { apiClient } from "./client";
import type { Command } from "./types";

export async function queueCommand(
  workspaceId: string,
  command: string,
): Promise<Command> {
  const response = await apiClient.post<Command>(
    `/workspaces/${workspaceId}/commands`,
    { command },
  );
  return response.data;
}
