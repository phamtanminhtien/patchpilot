import {
  BookOpen,
  FileEdit,
  FileText,
  FolderTree,
  type LucideIcon,
  Search,
  Terminal,
  Wrench,
} from "lucide-react";

import type { AgentToolCall } from "@/shared/api";

import { humanizeSkillName } from "./skills";

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
  use_skill: {
    Icon: BookOpen,
    label: "Load skill",
  },
};

const READ_FILE_METADATA = TOOL_METADATA.read_file as ToolCallMetadata;

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
  const input = parseToolInput(toolCall.input);
  const readCommandPath =
    toolCall.name === "run_command"
      ? readCommandFilePath(stringValue(input.command))
      : "";
  const metadata = readCommandPath
    ? READ_FILE_METADATA
    : toolCallMetadata(toolCall.name);
  const statusLabel = toolCallStatusLabel(toolCall);
  const detail = toolCallDetail(toolCall, input);

  return {
    ...metadata,
    detail,
    expandable:
      toolCall.name !== "read_file" &&
      (!readCommandPath || toolCall.requiresApproval),
    statusLabel,
    text: readCommandPath || toolCallText(toolCall.name, input, metadata.label),
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
    case "list_files":
      return isFinished ? "Listed" : "Listing";
    case "read_file":
      return isFinished ? "Read" : "Reading";
    case "run_command":
      if (
        readCommandFilePath(stringValue(parseToolInput(toolCall.input).command))
      ) {
        return isFinished ? "Read" : "Reading";
      }
      return isFinished ? "Ran" : "Running";
    case "search_files":
      return isFinished ? "Searched" : "Searching";
    case "use_skill":
      return isFinished ? "Loaded" : "Loading";
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
    case "list_files":
      return stringValue(input.path) || "workspace";
    case "read_file":
      return stringValue(input.path) || "file";
    case "run_command":
      return stringValue(input.command) || "command";
    case "search_files":
      return stringValue(input.query) || "workspace";
    case "use_skill":
      return humanizeSkillName(stringValue(input.name) || fallback);
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
    if (readCommandFilePath(stringValue(input.command))) {
      return "";
    }
    return toolCall.output || stringValue(input.command) || toolCall.input;
  }
  if (toolCall.name === "use_skill") {
    const output = parseToolInput(toolCall.output);
    return (
      stringValue(output.instruction) ||
      stringValue(output.description) ||
      humanizeSkillName(stringValue(input.name)) ||
      toolCall.output ||
      toolCall.input
    );
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

function readCommandFilePath(command: string) {
  const parts = splitCommand(command);
  const catPath = parts[1];
  if (parts.length === 2 && parts[0] === "cat" && catPath) {
    return validReadPath(catPath) ? catPath : "";
  }
  const sedRange = parts[2];
  const sedPath = parts[3];
  if (
    parts.length === 4 &&
    parts[0] === "sed" &&
    parts[1] === "-n" &&
    sedRange &&
    sedPath &&
    validSedPrintRange(sedRange) &&
    validReadPath(sedPath)
  ) {
    return sedPath;
  }
  return "";
}

function splitCommand(command: string) {
  const parts: string[] = [];
  let current = "";
  let inQuote = false;
  for (const character of command.trim()) {
    if (character === "'") {
      inQuote = !inQuote;
      continue;
    }
    if (!inQuote && /\s/.test(character)) {
      if (current) {
        parts.push(current);
        current = "";
      }
      continue;
    }
    current += character;
  }
  if (current) {
    parts.push(current);
  }
  return inQuote ? [] : parts;
}

function validReadPath(path: string) {
  return path.length > 0 && path !== "." && !path.endsWith("/");
}

function validSedPrintRange(range: string) {
  const match = /^(\d+),(\d+)p$/.exec(range);
  if (!match) {
    return false;
  }
  const start = Number(match[1]);
  const end = Number(match[2]);
  return (
    Number.isInteger(start) &&
    Number.isInteger(end) &&
    start >= 1 &&
    end >= start
  );
}
