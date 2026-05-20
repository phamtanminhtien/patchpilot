import { apiClient } from "./client";
import type { GitDiff, GitStatus } from "./types";

export async function getGitDiff(
  workspaceId: string,
  path?: string,
): Promise<GitDiff> {
  const response = await apiClient.get<GitDiff>(
    `/workspaces/${workspaceId}/git/diff`,
    {
      params: path ? { path } : undefined,
    },
  );
  return response.data;
}

export async function getGitStatus(workspaceId: string): Promise<GitStatus> {
  const response = await apiClient.get<GitStatus>(
    `/workspaces/${workspaceId}/git/status`,
  );
  return response.data;
}
