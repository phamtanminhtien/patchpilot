import { apiClient } from "./client";
import type {
  CreateTerminalSessionRequest,
  PaginationParams,
  PatchTerminalSessionRequest,
  TerminalSession,
  TerminalSessionListResponse,
  TerminalSessionResponse,
} from "./types";

export async function listTerminalSessions(
  workspaceId: string,
  params?: PaginationParams,
): Promise<TerminalSessionListResponse> {
  const response = await apiClient.get<TerminalSessionListResponse>(
    `/workspaces/${workspaceId}/terminal/sessions`,
    { params },
  );
  return response.data;
}

export async function createTerminalSession(
  workspaceId: string,
  request: CreateTerminalSessionRequest = {},
): Promise<TerminalSession> {
  const response = await apiClient.post<TerminalSessionResponse>(
    `/workspaces/${workspaceId}/terminal/sessions`,
    request,
  );
  return response.data.session;
}

export async function patchTerminalSession(
  workspaceId: string,
  sessionId: string,
  request: PatchTerminalSessionRequest,
): Promise<TerminalSession> {
  const response = await apiClient.patch<TerminalSessionResponse>(
    `/workspaces/${workspaceId}/terminal/sessions/${sessionId}`,
    request,
  );
  return response.data.session;
}

export async function closeTerminalSession(
  workspaceId: string,
  sessionId: string,
): Promise<TerminalSession> {
  const response = await apiClient.post<TerminalSessionResponse>(
    `/workspaces/${workspaceId}/terminal/sessions/${sessionId}/close`,
  );
  return response.data.session;
}

export function terminalSocketUrl(workspaceId: string, sessionId: string) {
  const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
  return `${protocol}//${window.location.host}/api/workspaces/${workspaceId}/terminal/sessions/${sessionId}/socket`;
}
