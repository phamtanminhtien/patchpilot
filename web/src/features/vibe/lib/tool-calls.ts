import {
  FileEdit,
  FileText,
  FolderTree,
  GitBranch,
  GitCompare,
  type LucideIcon,
  Search,
  Terminal,
  Wrench,
} from "lucide-react";

import type { AgentToolCall } from "@/shared/api";

interface ToolCallMetadata {
  Icon: LucideIcon;
  label: string;
}

interface ToolCallDisplay {
  Icon: LucideIcon;
  detail: string;
  expandable: boolean;
  label: string;
  statusLabel: string;
  text: string;
}

const TOOL_METADATA: Record<string, ToolCallMetadata> = {
  apply_patch: {
    Icon: FileEdit,
    label: "Apply patch",
  },
  git_diff: {
    Icon: GitCompare,
    label: "Inspect diff",
  },
  git_status: {
    Icon: GitBranch,
    label: "Check Git status",
  },
  list_files: {
    Icon: FolderTree,
    label: "List files",
  },
  read_file: {
    Icon: FileText,
    label: "Read file",
  },
  run_command: {
    Icon: Terminal,
    label: "Run command",
  },
  search_files: {
    Icon: Search,
    label: "Search files",
  },
};

export function parseToolInput(input: string): Record<string, unknown> {
  try {
    const parsed = JSON.parse(input) as unknown;
    return parsed && typeof parsed === "object"
      ? (parsed as Record<string, unknown>)
      : {};
  } catch {
    return {};
  }
}

export function toolCallMetadata(name: string): ToolCallMetadata {
  return (
    TOOL_METADATA[name] ?? {
      Icon: Wrench,
      label: titleCaseToolName(name),
    }
  );
}

export function toolCallDisplay(toolCall: AgentToolCall): ToolCallDisplay {
  const metadata = toolCallMetadata(toolCall.name);
  const input = parseToolInput(toolCall.input);
  const statusLabel = toolCallStatusLabel(toolCall);
  const detail = toolCallDetail(toolCall, input);

  return {
    ...metadata,
    detail,
    expandable: toolCall.name !== "read_file",
    statusLabel,
    text: toolCallText(toolCall.name, input, metadata.label),
  };
}

export function toolCallNeedsAttention(toolCall: AgentToolCall) {
  return (
    toolCall.status === "waiting_approval" ||
    toolCall.status === "running" ||
    toolCall.status === "failed"
  );
}

export function nextApprovalToolCall(toolCalls: AgentToolCall[]) {
  return [...toolCalls]
    .filter(
      (toolCall) =>
        toolCall.requiresApproval && toolCall.status === "waiting_approval",
    )
    .sort((left, right) =>
      left.batchId === right.batchId
        ? left.sequence - right.sequence
        : left.createdAt.localeCompare(right.createdAt),
    )[0];
}

function titleCaseToolName(name: string) {
  return name
    .split(/[_\s-]+/)
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(" ");
}

function toolCallStatusLabel(toolCall: AgentToolCall) {
  if (toolCall.status === "waiting_approval") {
    return "Waiting approval";
  }
  if (toolCall.status === "approved") {
    return "Approved";
  }
  if (toolCall.status === "rejected") {
    return "Rejected";
  }
  if (toolCall.status === "failed") {
    return "Failed";
  }
  if (toolCall.status === "pending") {
    return "Pending";
  }

  const isFinished = toolCall.status === "finished";

  switch (toolCall.name) {
    case "apply_patch":
      return isFinished ? "Edited" : "Editing";
    case "git_diff":
      return isFinished ? "Inspected" : "Inspecting";
    case "git_status":
      return isFinished ? "Checked" : "Checking";
    case "list_files":
      return isFinished ? "Listed" : "Listing";
    case "read_file":
      return isFinished ? "Read" : "Reading";
    case "run_command":
      return isFinished ? "Ran" : "Running";
    case "search_files":
      return isFinished ? "Searched" : "Searching";
    default:
      return isFinished ? "Finished" : "Running";
  }
}

function toolCallText(
  name: string,
  input: Record<string, unknown>,
  fallback: string,
) {
  switch (name) {
    case "apply_patch":
      return (
        firstPatchPath(stringValue(input.diff)) ||
        stringValue(input.summary) ||
        "patch"
      );
    case "git_diff":
      return stringValue(input.path) || "workspace diff";
    case "git_status":
      return "workspace";
    case "list_files":
      return stringValue(input.path) || "workspace";
    case "read_file":
      return stringValue(input.path) || "file";
    case "run_command":
      return stringValue(input.command) || "command";
    case "search_files":
      return stringValue(input.query) || "workspace";
    default:
      return fallback;
  }
}

function toolCallDetail(
  toolCall: AgentToolCall,
  input: Record<string, unknown>,
) {
  if (toolCall.name === "read_file") {
    return "";
  }
  if (toolCall.name === "apply_patch") {
    return stringValue(input.diff) || toolCall.output || toolCall.input;
  }
  if (toolCall.name === "run_command") {
    return toolCall.output || stringValue(input.command) || toolCall.input;
  }
  if (toolCall.name === "git_diff") {
    return toolCall.output || toolCall.input;
  }
  return toolCall.output || toolCall.input;
}

function firstPatchPath(diff: string) {
  const match = /^diff --git a\/(.+?) b\//m.exec(diff);
  return match?.[1] ?? "";
}

function stringValue(value: unknown) {
  return typeof value === "string" ? value.trim() : "";
}
