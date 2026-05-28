import { useQuery } from "@tanstack/react-query";
import { Copy } from "lucide-react";
import { useMemo, useState } from "react";

import {
  type FileEntry,
  type FileIndexEntry,
  listFileIndexDirectory,
  listFiles,
} from "@/shared/api";
import { FileIcon } from "@/shared/file-icons";
import { cn, HoverCard } from "@/shared/ui";

import { LoadingState } from "./components/loading-state";
import type { GitChange } from "./git/workspace-git";
import {
  gitStatusBadgeCode,
  gitStatusTextTone,
} from "./git/workspace-git-status";
import {
  WorkspaceFileTreeItem,
  type WorkspaceFileTreeStatus,
} from "./workspace-file-tree-item";

type FileTreeNode = {
  children: FileTreeNode[];
  entry?: FileIndexEntry;
  name: string;
  path: string;
  type: "directory" | "file";
};

type NodeGitStatus = WorkspaceFileTreeStatus;

interface WorkspaceFileTreeProps {
  entries: FileIndexEntry[];
  error?: string;
  gitChanges: GitChange[];
  isLoading: boolean;
  onSelect: (path: string) => void;
  selectedPath: string;
  workspaceId: string;
}

export function WorkspaceFileTree({
  entries,
  error,
  gitChanges,
  isLoading,
  onSelect,
  selectedPath,
  workspaceId,
}: WorkspaceFileTreeProps) {
  const tree = useMemo(
    () => entries.map(entryToNode).sort(compareFileTreeNodes),
    [entries],
  );
  const [expandedFolders, setExpandedFolders] = useState<Set<string>>(
    () => new Set(),
  );
  const [collapsedFolders, setCollapsedFolders] = useState<{
    paths: Set<string>;
    selectedPath: string;
  }>(() => ({ paths: new Set(), selectedPath: "" }));
  const selectedAncestorFolders = useMemo(
    () => new Set(collectAncestorFolders(selectedPath)),
    [selectedPath],
  );
  const manuallyCollapsedFolders = useMemo(
    () =>
      collapsedFolders.selectedPath === selectedPath
        ? collapsedFolders.paths
        : new Set<string>(),
    [collapsedFolders, selectedPath],
  );
  const visibleExpandedFolders = useMemo(() => {
    const next = new Set([...expandedFolders, ...selectedAncestorFolders]);
    for (const path of manuallyCollapsedFolders) {
      next.delete(path);
    }
    return next;
  }, [expandedFolders, manuallyCollapsedFolders, selectedAncestorFolders]);

  function toggleFolder(path: string) {
    if (visibleExpandedFolders.has(path)) {
      setExpandedFolders((current) => {
        const next = new Set(current);
        next.delete(path);
        return next;
      });
      setCollapsedFolders((current) => {
        const next =
          current.selectedPath === selectedPath
            ? new Set(current.paths)
            : new Set<string>();
        next.add(path);
        return { paths: next, selectedPath };
      });
    } else {
      setExpandedFolders((current) => {
        const next = new Set(current);
        next.add(path);
        return next;
      });
      setCollapsedFolders((current) => {
        const next =
          current.selectedPath === selectedPath
            ? new Set(current.paths)
            : new Set<string>();
        next.delete(path);
        return { paths: next, selectedPath };
      });
    }
  }

  if (error) {
    return <ErrorState message={error} />;
  }

  if (isLoading) {
    return <LoadingState label="Loading file index" />;
  }

  if (entries.length === 0) {
    return (
      <div className="grid gap-2 p-1.5">
        <EmptyState message="No indexed files found." />
      </div>
    );
  }

  return (
    <div className="grid gap-1">
      <div aria-label="File tree" className="grid" role="tree">
        {tree.map((node) => (
          <FileTreeItem
            expandedFolders={visibleExpandedFolders}
            gitChanges={gitChanges}
            key={node.path}
            node={node}
            onSelect={onSelect}
            onToggle={toggleFolder}
            selectedPath={selectedPath}
            workspaceId={workspaceId}
          />
        ))}
      </div>
    </div>
  );
}

function FileTreeItem({
  depth = 0,
  expandedFolders,
  gitChanges,
  node,
  onSelect,
  onToggle,
  selectedPath,
  workspaceId,
}: {
  depth?: number;
  expandedFolders: Set<string>;
  gitChanges: GitChange[];
  node: FileTreeNode;
  onSelect: (path: string) => void;
  onToggle: (path: string) => void;
  selectedPath: string;
  workspaceId: string;
}) {
  const isDirectory = node.type === "directory";
  const isExpanded = isDirectory && expandedFolders.has(node.path);
  const isSelected = !isDirectory && selectedPath === node.path;
  const gitStatus = gitStatusForNode(node, gitChanges);
  const isIgnored = gitStatus?.label === "Ignored";
  const childrenQuery = useQuery({
    enabled: isExpanded && isDirectory && workspaceId.length > 0,
    queryFn: async () => {
      if (node.entry?.indexStatus === "skipped") {
        const response = await listFiles(workspaceId, node.path);
        return { entries: response.entries.map(fileEntryToIndexEntry) };
      }
      return listFileIndexDirectory(workspaceId, node.path, {
        includeSkipped: true,
      });
    },
    queryKey: ["workspace-file-index-directory", workspaceId, node.path],
  });
  const children = childrenQuery.data?.entries
    ? childrenQuery.data.entries.map(entryToNode).sort(compareFileTreeNodes)
    : node.children;

  return (
    <div role="none">
      <HoverCard
        content={<FileNodeDetails gitStatus={gitStatus} node={node} />}
        openDelay={450}
      >
        <WorkspaceFileTreeItem
          aria-expanded={isDirectory ? isExpanded : undefined}
          depth={depth}
          disclosure={
            isDirectory ? (isExpanded ? "expanded" : "collapsed") : "none"
          }
          icon={
            <FileIcon
              isDirectory={isDirectory}
              isExpanded={isExpanded}
              name={node.name}
              path={node.path}
            />
          }
          isDimmed={isIgnored}
          isSelected={isSelected}
          label={
            <span className={cn(nodeColorClass(gitStatus, isDirectory))}>
              {node.name}
            </span>
          }
          onClick={() => {
            if (isDirectory) {
              onToggle(node.path);
            } else {
              onSelect(node.path);
            }
          }}
          role="treeitem"
          status={gitStatus}
        />
      </HoverCard>

      {isExpanded ? (
        <div role="group">
          {childrenQuery.isPending && node.children.length === 0 ? (
            <div className="py-0.5" role="none">
              <WorkspaceFileTreeItem
                depth={depth + 1}
                disclosure="none"
                icon={<span className="size-3" />}
                isDimmed
                isSelected={false}
                label={<span className="text-muted">Loading...</span>}
                onClick={() => {}}
                role="treeitem"
              />
            </div>
          ) : null}
          {children.map((child) => (
            <FileTreeItem
              depth={depth + 1}
              expandedFolders={expandedFolders}
              gitChanges={gitChanges}
              key={child.path}
              node={child}
              onSelect={onSelect}
              onToggle={onToggle}
              selectedPath={selectedPath}
              workspaceId={workspaceId}
            />
          ))}
        </div>
      ) : null}
    </div>
  );
}

function FileNodeDetails({
  gitStatus,
  node,
}: {
  gitStatus: NodeGitStatus | null;
  node: FileTreeNode;
}) {
  return (
    <div className="grid max-w-80 gap-2">
      <div className="grid gap-1">
        <p className="text-ink truncate font-semibold">{node.name}</p>
        <p className="text-muted break-all">{node.path || "."}</p>
        {node.entry ? (
          <div className="text-muted grid grid-cols-[auto_minmax(0,1fr)] gap-x-2 gap-y-1">
            <span>Size</span>
            <span className="text-ink">{formatFileSize(node.entry.size)}</span>
            <span>Modified</span>
            <span className="text-ink">
              {formatModifiedTime(node.entry.modifiedAt)}
            </span>
          </div>
        ) : (
          <p className="text-muted">{node.children.length} items</p>
        )}
        {gitStatus ? (
          <div className="text-muted grid grid-cols-[auto_minmax(0,1fr)] gap-x-2 gap-y-1">
            <span>Git</span>
            <span className={cn("text-ink", nodeColorClass(gitStatus))}>
              {gitStatus.label}
            </span>
          </div>
        ) : null}
      </div>
      <div className="flex min-w-0 gap-1">
        <CopyButton label="Copy path" value={node.path || "."} />
        <CopyButton label="Copy name" value={node.name || "."} />
      </div>
    </div>
  );
}

function CopyButton({ label, value }: { label: string; value: string }) {
  const [copied, setCopied] = useState(false);

  async function handleCopy() {
    await navigator.clipboard?.writeText(value);
    setCopied(true);
    window.setTimeout(() => setCopied(false), 1200);
  }

  return (
    <button
      className="bg-hover text-ink hover:bg-accent-soft hover:text-accent inline-flex min-h-7 min-w-0 items-center gap-1 rounded-xl px-2 text-xs transition"
      onClick={() => void handleCopy()}
      type="button"
    >
      <Copy aria-hidden="true" className="size-3 shrink-0" />
      <span className="truncate">{copied ? "Copied" : label}</span>
    </button>
  );
}

function nodeColorClass(status: NodeGitStatus | null, isDirectory = false) {
  if (!status) {
    return isDirectory ? "text-ink" : "text-ink";
  }
  return gitStatusTextTone(status.label);
}

function gitStatusForNode(
  node: FileTreeNode,
  changes: GitChange[],
): NodeGitStatus | null {
  if (node.type === "file") {
    const change = changes.find((candidate) =>
      pathMatchesNode(candidate.path, node.path),
    );
    if (!change) {
      return null;
    }
    return {
      code: gitStatusBadgeCode({
        code: change.code,
        label: change.status,
      }),
      label: change.status,
    };
  }

  const exactIgnoredFolder = changes.find((change) => {
    const changePath = normalizeGitPath(change.path);
    return change.status === "Ignored" && changePath === node.path;
  });
  if (exactIgnoredFolder) {
    return {
      code: gitStatusBadgeCode({
        code: exactIgnoredFolder.code,
        label: exactIgnoredFolder.status,
      }),
      label: exactIgnoredFolder.status,
    };
  }

  const changedChildren = changes.filter((change) => {
    const changePath = normalizeGitPath(change.path);
    return (
      change.status !== "Ignored" &&
      (changePath === node.path || changePath.startsWith(`${node.path}/`))
    );
  });
  if (changedChildren.length === 0) {
    return null;
  }
  return {
    code: String(changedChildren.length),
    label: "Changed",
  };
}

function pathMatchesNode(gitPath: string, nodePath: string) {
  const normalized = normalizeGitPath(gitPath);
  return normalized === nodePath || nodePath.startsWith(`${normalized}/`);
}

function normalizeGitPath(path: string) {
  return path.endsWith("/") ? path.slice(0, -1) : path;
}

function entryToNode(entry: FileIndexEntry): FileTreeNode {
  return {
    children: [],
    entry,
    name:
      entry.name || entry.path.split("/").filter(Boolean).at(-1) || entry.path,
    path: entry.path,
    type: entry.kind === "folder" ? "directory" : "file",
  };
}

function fileEntryToIndexEntry(entry: FileEntry): FileIndexEntry {
  return {
    dir: pathDir(entry.path),
    extension: entry.isDir ? "" : fileExtension(entry.name),
    indexStatus: "indexed",
    kind: entry.isDir ? "folder" : "file",
    modifiedAt: entry.modifiedAt,
    name: entry.name,
    path: entry.path,
    size: entry.size,
  };
}

function pathDir(path: string) {
  const parts = path.split("/").filter(Boolean);
  return parts.slice(0, -1).join("/");
}

function fileExtension(name: string) {
  const index = name.lastIndexOf(".");
  if (index <= 0 || index === name.length - 1) {
    return "";
  }
  return name.slice(index + 1).toLowerCase();
}

function compareFileTreeNodes(left: FileTreeNode, right: FileTreeNode) {
  if (left.type !== right.type) {
    return left.type === "directory" ? -1 : 1;
  }
  return left.name.localeCompare(right.name);
}

function collectAncestorFolders(path: string) {
  const parts = path.split("/").filter(Boolean);
  return parts
    .slice(0, -1)
    .map((_, index) => parts.slice(0, index + 1).join("/"));
}

function EmptyState({ message }: { message: string }) {
  return <p className="text-muted text-xs leading-5">{message}</p>;
}

function ErrorState({ message }: { message: string }) {
  return <p className="text-warning text-xs font-medium">{message}</p>;
}

function formatFileSize(size: number) {
  if (size < 1024) {
    return `${size} B`;
  }
  if (size < 1024 * 1024) {
    return `${(size / 1024).toFixed(1)} KB`;
  }
  return `${(size / (1024 * 1024)).toFixed(1)} MB`;
}

function formatModifiedTime(value: string) {
  return new Intl.DateTimeFormat(undefined, {
    dateStyle: "medium",
    timeStyle: "short",
  }).format(new Date(value));
}
