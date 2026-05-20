export interface RestErrorBody {
  code: string;
  details: Record<string, unknown>;
  message: string;
}

export interface RestErrorResponse {
  error: RestErrorBody;
}

export interface Workspace {
  createdAt: string;
  id: string;
  name: string;
  rootPath: string;
  status: "indexing" | "ready" | "error";
  updatedAt: string;
}

export interface FileEntry {
  isDir: boolean;
  name: string;
  path: string;
  size: number;
}

export interface FileListResponse {
  entries: FileEntry[];
}

export interface FileContent {
  content: string;
  path: string;
}

export interface GitStatus {
  porcelain: string;
}

export interface GitDiff {
  diff: string;
  path?: string;
}

export interface Command {
  command: string;
  createdAt: string;
  id: string;
  status: "queued" | "running" | "exited" | "stopped" | "failed";
}
