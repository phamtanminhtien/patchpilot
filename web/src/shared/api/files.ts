import { apiClient } from "./client";
import type {
  FileContent,
  FileIndexParams,
  FileIndexResponse,
  FileListResponse,
  FileSearchParams,
  FileSearchResponse,
  FileWriteRequest,
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
  params?: FileIndexParams,
): Promise<FileIndexResponse> {
  const response = await apiClient.get<FileIndexResponse>(
    `/workspaces/${workspaceId}/files/index`,
    { params },
  );
  return response.data;
}

export async function listFileIndexDirectory(
  workspaceId: string,
  dir = "",
  params?: Omit<FileIndexParams, "cursor" | "dir" | "limit" | "q">,
): Promise<FileIndexResponse> {
  return listFileIndex(workspaceId, {
    ...params,
    dir,
  });
}

export async function listAllFileIndex(
  workspaceId: string,
  params?: Omit<FileIndexParams, "cursor">,
): Promise<FileIndexResponse> {
  const entries: FileIndexResponse["entries"] = [];
  let cursor: string | undefined;
  let state: FileIndexResponse["state"];
  let total: number | undefined;

  do {
    const page = await listFileIndex(workspaceId, {
      ...params,
      cursor,
      limit: 100,
    });
    entries.push(...page.entries);
    cursor = page.nextCursor ?? undefined;
    state = page.state;
    total = page.total;
  } while (cursor);

  return {
    entries,
    nextCursor: null,
    state,
    total,
  };
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
  params?: FileSearchParams,
): Promise<FileSearchResponse> {
  const response = await apiClient.get<FileSearchResponse>(
    `/workspaces/${workspaceId}/search`,
    {
      params: { ...params, q: query },
    },
  );
  return response.data;
}
