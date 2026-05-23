import { apiClient } from "./client";
import type {
  AgentRun,
  AgentToolCall,
  AgentToolCallResponse,
  Conversation,
  ConversationDetail,
  ConversationListResponse,
  CreateConversationRequest,
  CreateMessageRequest,
  MessageRunResponse,
  PaginationParams,
} from "./types";

export async function createConversation(
  workspaceId: string,
  request: CreateConversationRequest,
): Promise<Conversation> {
  const response = await apiClient.post<Conversation>(
    `/workspaces/${workspaceId}/conversations`,
    request,
  );
  return response.data;
}

export async function listConversations(
  workspaceId: string,
  params?: PaginationParams,
): Promise<ConversationListResponse> {
  const response = await apiClient.get<ConversationListResponse>(
    `/workspaces/${workspaceId}/conversations`,
    { params },
  );
  return response.data;
}

export async function getConversation(
  workspaceId: string,
  conversationId: string,
): Promise<ConversationDetail> {
  const response = await apiClient.get<ConversationDetail>(
    `/workspaces/${workspaceId}/conversations/${conversationId}`,
  );
  return response.data;
}

export async function updateConversation(
  workspaceId: string,
  conversationId: string,
  request: CreateConversationRequest,
): Promise<Conversation> {
  const response = await apiClient.patch<Conversation>(
    `/workspaces/${workspaceId}/conversations/${conversationId}`,
    request,
  );
  return response.data;
}

export async function createMessage(
  workspaceId: string,
  conversationId: string,
  request: CreateMessageRequest,
): Promise<MessageRunResponse> {
  const response = await apiClient.post<MessageRunResponse>(
    `/workspaces/${workspaceId}/conversations/${conversationId}/messages`,
    request,
  );
  return response.data;
}

export async function cancelAgentRun(
  workspaceId: string,
  conversationId: string,
  runId: string,
): Promise<AgentRun> {
  const response = await apiClient.post<AgentRun>(
    `/workspaces/${workspaceId}/conversations/${conversationId}/runs/${runId}/cancel`,
  );
  return response.data;
}

export async function approveAgentToolCall(
  workspaceId: string,
  conversationId: string,
  runId: string,
  toolCallId: string,
): Promise<AgentToolCall> {
  const response = await apiClient.post<AgentToolCallResponse>(
    `/workspaces/${workspaceId}/conversations/${conversationId}/runs/${runId}/tool-calls/${toolCallId}/approve`,
  );
  return response.data.toolCall;
}

export async function rejectAgentToolCall(
  workspaceId: string,
  conversationId: string,
  runId: string,
  toolCallId: string,
): Promise<AgentToolCall> {
  const response = await apiClient.post<AgentToolCallResponse>(
    `/workspaces/${workspaceId}/conversations/${conversationId}/runs/${runId}/tool-calls/${toolCallId}/reject`,
  );
  return response.data.toolCall;
}
