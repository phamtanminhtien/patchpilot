import { apiClient } from "./client";
import type {
  AgentTask,
  AgentTaskDetail,
  AgentTaskListResponse,
  CreateAgentTaskRequest,
} from "./types";

export async function createAgentTask(
  workspaceId: string,
  request: CreateAgentTaskRequest,
): Promise<AgentTask> {
  const response = await apiClient.post<AgentTask>(
    `/workspaces/${workspaceId}/agent/tasks`,
    request,
  );
  return response.data;
}

export async function listAgentTasks(
  workspaceId: string,
): Promise<AgentTaskListResponse> {
  const response = await apiClient.get<AgentTaskListResponse>(
    `/workspaces/${workspaceId}/agent/tasks`,
  );
  return response.data;
}

export async function getAgentTask(
  workspaceId: string,
  taskId: string,
): Promise<AgentTaskDetail> {
  const response = await apiClient.get<AgentTaskDetail>(
    `/workspaces/${workspaceId}/agent/tasks/${taskId}`,
  );
  return response.data;
}

export async function cancelAgentTask(
  workspaceId: string,
  taskId: string,
): Promise<AgentTask> {
  const response = await apiClient.post<AgentTask>(
    `/workspaces/${workspaceId}/agent/tasks/${taskId}/cancel`,
  );
  return response.data;
}
