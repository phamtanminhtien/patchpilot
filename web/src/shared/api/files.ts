import { apiClient } from "./client";
import type {
  FileContent,
  FileIndexResponse,
  FileListResponse,
  FileSearchResponse,
  FileWriteRequest,
  PaginationParams,
} from "./types";

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

export async function writeFile(
  workspaceId: string,
  request: FileWriteRequest,
): Promise<FileContent> {
  const response = await apiClient.put<FileContent>(
    `/workspaces/${workspaceId}/file`,
    request,
  );
  return response.data;
}

export async function listFileIndex(
  workspaceId: string,
  params?: PaginationParams,
): Promise<FileIndexResponse> {
  const response = await apiClient.get<FileIndexResponse>(
    `/workspaces/${workspaceId}/files/index`,
    { params },
  );
  return response.data;
}

export async function refreshFileIndex(
  workspaceId: string,
): Promise<FileIndexResponse> {
  const response = await apiClient.post<FileIndexResponse>(
    `/workspaces/${workspaceId}/files/index/refresh`,
  );
  return response.data;
}

export async function searchFiles(
  workspaceId: string,
  query: string,
  params?: PaginationParams,
): Promise<FileSearchResponse> {
  const response = await apiClient.get<FileSearchResponse>(
    `/workspaces/${workspaceId}/search`,
    {
      params: { ...params, q: query },
    },
  );
  return response.data;
}
