import { apiClient } from "./client";
import type { AgentPatch, PatchResponse } from "./types";

export async function getPatch(
  workspaceId: string,
  patchId: string,
): Promise<AgentPatch> {
  const response = await apiClient.get<PatchResponse>(
    `/workspaces/${workspaceId}/patches/${patchId}`,
  );
  return response.data.patch;
}

export async function applyPatch(
  workspaceId: string,
  patchId: string,
): Promise<AgentPatch> {
  const response = await apiClient.post<PatchResponse>(
    `/workspaces/${workspaceId}/patches/${patchId}/apply`,
  );
  return response.data.patch;
}

export async function rejectPatch(
  workspaceId: string,
  patchId: string,
): Promise<AgentPatch> {
  const response = await apiClient.post<PatchResponse>(
    `/workspaces/${workspaceId}/patches/${patchId}/reject`,
  );
  return response.data.patch;
}

export async function revertPatch(
  workspaceId: string,
  patchId: string,
): Promise<AgentPatch> {
  const response = await apiClient.post<PatchResponse>(
    `/workspaces/${workspaceId}/patches/${patchId}/revert`,
  );
  return response.data.patch;
}
