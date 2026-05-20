import { apiClient } from "./client";
import type { Workspace, WorkspaceListResponse } from "./types";

export async function createWorkspace(rootPath: string): Promise<Workspace> {
  const response = await apiClient.post<Workspace>("/workspaces", { rootPath });
  return response.data;
}

export async function getWorkspace(workspaceId: string): Promise<Workspace> {
  const response = await apiClient.get<Workspace>(`/workspaces/${workspaceId}`);
  return response.data;
}

export async function listWorkspaces(): Promise<WorkspaceListResponse> {
  const response = await apiClient.get<WorkspaceListResponse>("/workspaces");
  return response.data;
}
