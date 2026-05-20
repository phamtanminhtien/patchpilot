import { apiClient } from "./client";
import type { FileContent, FileListResponse } from "./types";

export async function listFiles(
  workspaceId: string,
  path = ".",
): Promise<FileListResponse> {
  const response = await apiClient.get<FileListResponse>(
    `/workspaces/${workspaceId}/files`,
    {
      params: { path },
    },
  );
  return response.data;
}

export async function readFile(
  workspaceId: string,
  path: string,
): Promise<FileContent> {
  const response = await apiClient.get<FileContent>(
    `/workspaces/${workspaceId}/file`,
    {
      params: { path },
    },
  );
  return response.data;
}
