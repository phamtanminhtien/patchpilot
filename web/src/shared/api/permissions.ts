import { apiClient } from "./client";
import type {
  PatchWorkspacePermissionsRequest,
  WorkspacePermissions,
  WorkspacePermissionsResponse,
} from "./types";

export async function getWorkspacePermissions(
  workspaceId: string,
): Promise<WorkspacePermissions> {
  const response = await apiClient.get<WorkspacePermissionsResponse>(
    `/workspaces/${workspaceId}/permissions`,
  );
  return response.data.permissions;
}

export async function patchWorkspacePermissions(
  workspaceId: string,
  permissions: PatchWorkspacePermissionsRequest,
): Promise<WorkspacePermissions> {
  const response = await apiClient.patch<WorkspacePermissionsResponse>(
    `/workspaces/${workspaceId}/permissions`,
    permissions,
  );
  return response.data.permissions;
}
