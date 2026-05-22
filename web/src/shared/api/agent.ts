import { apiClient } from "./client";
import type {
  AgentTask,
  AgentTaskDetail,
  AgentTaskListResponse,
  AgentToolCall,
  AgentToolCallResponse,
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

export async function approveAgentToolCall(
  workspaceId: string,
  taskId: string,
  toolCallId: string,
): Promise<AgentToolCall> {
  const response = await apiClient.post<AgentToolCallResponse>(
    `/workspaces/${workspaceId}/agent/tasks/${taskId}/tool-calls/${toolCallId}/approve`,
  );
  return response.data.toolCall;
}

export async function rejectAgentToolCall(
  workspaceId: string,
  taskId: string,
  toolCallId: string,
): Promise<AgentToolCall> {
  const response = await apiClient.post<AgentToolCallResponse>(
    `/workspaces/${workspaceId}/agent/tasks/${taskId}/tool-calls/${toolCallId}/reject`,
  );
  return response.data.toolCall;
}
