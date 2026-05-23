import { apiClient } from "./client";
import type {
  PaginationParams,
  Port,
  PortListResponse,
  PortResponse,
} from "./types";

export async function listPorts(
  workspaceId: string,
  params?: PaginationParams,
): Promise<PortListResponse> {
  const response = await apiClient.get<PortListResponse>(
    `/workspaces/${workspaceId}/ports`,
    { params },
  );
  return response.data;
}

export async function exposePort(
  workspaceId: string,
  port: number,
): Promise<Port> {
  const response = await apiClient.post<PortResponse>(
    `/workspaces/${workspaceId}/ports/${port}/expose`,
  );
  return response.data.port;
}
