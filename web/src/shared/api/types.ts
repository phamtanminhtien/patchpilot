export interface RestErrorBody {
  code: string;
  details: Record<string, unknown>;
  message: string;
}

export interface RestErrorResponse {
  error: RestErrorBody;
}

export interface HealthResponse {
  status: "ok";
}

export interface AuthSession {
  expiresAt: string;
  id: string;
  lastSeenAt: string;
}

export interface AuthSessionResponse {
  session: AuthSession;
}

export interface Workspace {
  createdAt: string;
  id: string;
  name: string;
  rootPath: string;
  status: "indexing" | "ready" | "error";
  updatedAt: string;
}

export interface WorkspaceListResponse {
  workspaces: Workspace[];
}

export interface FileEntry {
  isDir: boolean;
  name: string;
  path: string;
  size: number;
  modifiedAt: string;
}

export interface FileListResponse {
  entries: FileEntry[];
}

export interface FileIndexEntry {
  modifiedAt: string;
  path: string;
  size: number;
}

export interface FileIndexResponse {
  entries: FileIndexEntry[];
}

export interface FileContent {
  content: string;
  path: string;
}

export interface FileSearchResult {
  kind: "filename" | "content";
  line?: number;
  path: string;
  preview?: string;
}

export interface FileSearchResponse {
  results: FileSearchResult[];
}

export interface GitStatus {
  porcelain: string;
}

export interface GitDiff {
  diff: string;
  path?: string;
}

export interface GitStageRequest {
  paths: string[];
}

export interface GitDiscardRequest {
  paths: string[];
}

export interface GitUnstageRequest {
  paths: string[];
}

export interface GitCommitRequest {
  message: string;
  paths: string[];
}

export interface GitCommitResponse {
  hash: string;
}

export interface Command {
  command: string;
  cwd: string;
  createdAt: string;
  durationMs?: number | null;
  exitCode?: number | null;
  finishedAt?: string | null;
  id: string;
  startedAt?: string | null;
  status: "queued" | "running" | "exited" | "stopped" | "failed";
  workspaceId: string;
}

export interface CommandOutput {
  chunk: string;
  commandId: string;
  createdAt: string;
  id: string;
  stream: "stdout" | "stderr";
}

export interface CommandDetail {
  command: Command;
  output: CommandOutput[];
}

export interface CommandListResponse {
  processes: Command[];
}

export interface CommandEvent {
  createdAt: string;
  id: string;
  payload: Command | CommandOutput;
  type: "process.started" | "command.output" | "process.exited";
  workspaceId: string;
}

export type AgentRunStatus =
  | "queued"
  | "running"
  | "waiting_tool_approval"
  | "done"
  | "failed";

export type AgentModel = "gpt-5.5" | "gpt-5.4" | "gpt-5.4-mini";

export type AgentReasoningEffort = "low" | "medium" | "high" | "xhigh";

export interface Conversation {
  createdAt: string;
  id: string;
  lastMessageAt: string;
  title: string;
  updatedAt: string;
  workspaceId: string;
}

export interface ConversationListResponse {
  conversations: Conversation[];
}

export interface CreateConversationRequest {
  title: string;
}

export interface CreateMessageRequest {
  content: string;
  model: AgentModel;
  reasoningEffort: AgentReasoningEffort;
}

export interface Message {
  content: string;
  conversationId: string;
  createdAt: string;
  id: string;
  role: "user" | "assistant";
  runId?: string | null;
  workspaceId: string;
}

export interface MessageRunResponse {
  message: Message;
  run: AgentRun;
}

export interface AgentRun {
  conversationId: string;
  createdAt: string;
  error?: string | null;
  finishedAt?: string | null;
  id: string;
  model: AgentModel;
  reasoningEffort: AgentReasoningEffort;
  startedAt?: string | null;
  status: AgentRunStatus;
  summary: string;
  triggerMessageId: string;
  updatedAt: string;
  workspaceId: string;
}

export interface AgentRunEvent {
  createdAt: string;
  id: string;
  payload: unknown;
  runId: string;
  type:
    | "agent.delta"
    | "agent.tool.started"
    | "agent.tool.finished"
    | "agent.approval_required"
    | "agent.run.status_changed";
  workspaceId: string;
}

export interface AgentToolCall {
  batchId: string;
  createdAt: string;
  decision?: "approved" | "rejected" | null;
  finishedAt?: string | null;
  id: string;
  input: string;
  name: string;
  output: string;
  providerCallId: string;
  requiresApproval: boolean;
  sequence: number;
  startedAt?: string | null;
  status:
    | "pending"
    | "waiting_approval"
    | "approved"
    | "rejected"
    | "running"
    | "finished"
    | "failed";
  runId: string;
  workspaceId: string;
}

export interface AgentToolCallResponse {
  toolCall: AgentToolCall;
}

export interface Port {
  closedAt?: string | null;
  createdAt: string;
  exposedPath?: string | null;
  exposedUrl?: string | null;
  id: string;
  port: number;
  processId?: string | null;
  status: "detected" | "exposed" | "closed";
  updatedAt: string;
  workspaceId: string;
}

export interface PortListResponse {
  ports: Port[];
}

export interface PortResponse {
  port: Port;
}

export interface ConversationDetail {
  conversation: Conversation;
  events: AgentRunEvent[];
  messages: Message[];
  runs: AgentRun[];
  toolCalls: AgentToolCall[];
}

export interface WorkspaceEvent {
  createdAt: string;
  id: string;
  payload: unknown;
  type:
    | CommandEvent["type"]
    | AgentRunEvent["type"]
    | "workspace.indexing"
    | "workspace.ready"
    | "git.changed"
    | "port.opened"
    | "port.exposed"
    | "port.closed";
  workspaceId: string;
}
