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

export type AgentTaskStatus =
  | "queued"
  | "running"
  | "waiting_approval"
  | "applying"
  | "testing"
  | "done"
  | "rejected"
  | "failed";

export type AgentModel = "gpt-5.5" | "gpt-5.4" | "gpt-5.4-mini";

export type AgentReasoningEffort = "low" | "medium" | "high" | "xhigh";

export interface CreateAgentTaskRequest {
  model: AgentModel;
  prompt: string;
  reasoningEffort: AgentReasoningEffort;
}

export interface AgentTask {
  createdAt: string;
  error?: string | null;
  finishedAt?: string | null;
  generatedPatch: string;
  id: string;
  model: AgentModel;
  plan: string;
  prompt: string;
  reasoningEffort: AgentReasoningEffort;
  startedAt?: string | null;
  status: AgentTaskStatus;
  summary: string;
  updatedAt: string;
  workspaceId: string;
}

export interface AgentTaskListResponse {
  tasks: AgentTask[];
}

export interface AgentTaskEvent {
  createdAt: string;
  id: string;
  payload: unknown;
  taskId: string;
  type:
    | "agent.delta"
    | "agent.tool.started"
    | "agent.tool.finished"
    | "agent.approval_required"
    | "agent.task.status_changed"
    | "patch.created"
    | "patch.applied"
    | "patch.rejected"
    | "patch.reverted";
  workspaceId: string;
}

export interface AgentToolCall {
  createdAt: string;
  finishedAt?: string | null;
  id: string;
  input: string;
  name: string;
  output: string;
  startedAt?: string | null;
  status: "running" | "finished" | "failed";
  taskId: string;
  workspaceId: string;
}

export interface AgentPatch {
  appliedAt?: string | null;
  baseCommit?: string | null;
  createdAt: string;
  diff: string;
  id: string;
  status: string;
  summary: string;
  taskId: string;
  workspaceId: string;
}

export interface PatchResponse {
  patch: AgentPatch;
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

export interface AgentTaskDetail {
  events: AgentTaskEvent[];
  patches: AgentPatch[];
  task: AgentTask;
  toolCalls: AgentToolCall[];
}

export interface WorkspaceEvent {
  createdAt: string;
  id: string;
  payload: unknown;
  type:
    | CommandEvent["type"]
    | AgentTaskEvent["type"]
    | "workspace.indexing"
    | "workspace.ready"
    | "git.changed"
    | "port.opened"
    | "port.exposed"
    | "port.closed";
  workspaceId: string;
}
