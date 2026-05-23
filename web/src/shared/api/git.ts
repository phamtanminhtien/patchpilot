import { apiClient } from "./client";
import type {
  GitCommitRequest,
  GitCommitResponse,
  GitDiff,
  GitDiscardRequest,
  GitStageRequest,
  GitStatus,
  GitUnstageRequest,
} from "./types";

export async function commitGitChanges(
  workspaceId: string,
  request: GitCommitRequest,
): Promise<GitCommitResponse> {
  const response = await apiClient.post<GitCommitResponse>(
    `/workspaces/${workspaceId}/git/commit`,
    request,
  );
  return response.data;
}

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

export interface GitStatusOptions {
  ignored?: boolean;
  untracked?: "all" | "normal" | "no";
  ignore_submodules?: "none" | "untracked" | "dirty" | "all";
  paths?: string[];
}

export async function getGitStatus(
  workspaceId: string,
  options?: GitStatusOptions,
): Promise<GitStatus> {
  const response = await apiClient.get<GitStatus>(
    `/workspaces/${workspaceId}/git/status`,
    {
      params: options,
    },
  );
  return response.data;
}

export async function discardGitChanges(
  workspaceId: string,
  request: GitDiscardRequest,
): Promise<GitStatus> {
  const response = await apiClient.post<GitStatus>(
    `/workspaces/${workspaceId}/git/discard`,
    request,
  );
  return response.data;
}

export async function stageGitFiles(
  workspaceId: string,
  request: GitStageRequest,
): Promise<GitStatus> {
  const response = await apiClient.post<GitStatus>(
    `/workspaces/${workspaceId}/git/stage`,
    request,
  );
  return response.data;
}

export async function unstageGitFiles(
  workspaceId: string,
  request: GitUnstageRequest,
): Promise<GitStatus> {
  const response = await apiClient.post<GitStatus>(
    `/workspaces/${workspaceId}/git/unstage`,
    request,
  );
  return response.data;
}
