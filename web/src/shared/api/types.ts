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
  createdAt: string;
  id: string;
  status: "queued" | "running" | "exited" | "stopped" | "failed";
}
