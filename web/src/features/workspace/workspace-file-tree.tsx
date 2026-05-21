import { Copy, FileCode2, Folder, FolderOpen, RefreshCw } from "lucide-react";
import { useMemo, useState } from "react";

import type { FileIndexEntry } from "@/shared/api";
import { cn, HoverCard } from "@/shared/ui";

import {
  WorkspaceFileTreeItem,
  type WorkspaceFileTreeStatus,
} from "./workspace-file-tree-item";
import type { GitChange } from "./workspace-git";
import { gitStatusBadgeCode, gitStatusTextTone } from "./workspace-git-status";

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
}

export function WorkspaceFileTree({
  entries,
  error,
  gitChanges,
  isLoading,
  onSelect,
  selectedPath,
}: WorkspaceFileTreeProps) {
  const tree = useMemo(() => buildFileTree(entries), [entries]);
  const [expandedFolders, setExpandedFolders] = useState<Set<string>>(
    () => new Set(),
  );
  const selectedAncestorFolders = useMemo(
    () => new Set(collectAncestorFolders(selectedPath)),
    [selectedPath],
  );
  const visibleExpandedFolders = useMemo(
    () => new Set([...expandedFolders, ...selectedAncestorFolders]),
    [expandedFolders, selectedAncestorFolders],
  );

  function toggleFolder(path: string) {
    setExpandedFolders((current) => {
      const next = new Set(current);
      if (next.has(path)) {
        next.delete(path);
      } else {
        next.add(path);
      }
      return next;
    });
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
}: {
  depth?: number;
  expandedFolders: Set<string>;
  gitChanges: GitChange[];
  node: FileTreeNode;
  onSelect: (path: string) => void;
  onToggle: (path: string) => void;
  selectedPath: string;
}) {
  const isDirectory = node.type === "directory";
  const isExpanded = isDirectory && expandedFolders.has(node.path);
  const isSelected = !isDirectory && selectedPath === node.path;
  const gitStatus = gitStatusForNode(node, gitChanges);
  const isIgnored = gitStatus?.label === "Ignored";

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
            isDirectory ? (
              isExpanded ? (
                <FolderOpen
                  aria-hidden="true"
                  className={cn(
                    "size-3 shrink-0",
                    nodeColorClass(gitStatus, true),
                  )}
                />
              ) : (
                <Folder
                  aria-hidden="true"
                  className={cn(
                    "size-3 shrink-0",
                    nodeColorClass(gitStatus, true),
                  )}
                />
              )
            ) : (
              <FileCode2
                aria-hidden="true"
                className={cn(
                  "size-3 shrink-0",
                  nodeColorClass(gitStatus, false),
                )}
              />
            )
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
          {node.children.map((child) => (
            <FileTreeItem
              depth={depth + 1}
              expandedFolders={expandedFolders}
              gitChanges={gitChanges}
              key={child.path}
              node={child}
              onSelect={onSelect}
              onToggle={onToggle}
              selectedPath={selectedPath}
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
      className="bg-hover text-ink hover:bg-accent-soft hover:text-accent inline-flex min-h-7 min-w-0 items-center gap-1 rounded-sm px-2 text-xs transition"
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

function buildFileTree(entries: FileIndexEntry[]) {
  const root = new Map<string, FileTreeNode>();

  const directories = new Map<
    string,
    FileTreeNode & { childrenMap: Map<string, FileTreeNode> }
  >();
  const rootDirectory = {
    children: [],
    childrenMap: root,
    name: "",
    path: "",
    type: "directory" as const,
  };
  directories.set("", rootDirectory);

  for (const entry of entries) {
    const parts = entry.path.split("/").filter(Boolean);
    let parentPath = "";
    for (const [index, part] of parts.entries()) {
      const path = parts.slice(0, index + 1).join("/");
      const isFile = index === parts.length - 1;
      const parent = directories.get(parentPath) ?? rootDirectory;
      if (isFile) {
        parent.childrenMap.set(path, {
          children: [],
          entry,
          name: part,
          path,
          type: "file",
        });
      } else if (!directories.has(path)) {
        const directory = {
          children: [],
          childrenMap: new Map<string, FileTreeNode>(),
          name: part,
          path,
          type: "directory" as const,
        };
        directories.set(path, directory);
        parent.childrenMap.set(path, directory);
      }
      parentPath = path;
    }
  }

  const materialize = (
    node: FileTreeNode & { childrenMap?: Map<string, FileTreeNode> },
  ): FileTreeNode => {
    const childrenMap =
      "childrenMap" in node && node.childrenMap ? node.childrenMap : undefined;
    const children = childrenMap
      ? [...childrenMap.values()]
          .sort((left, right) => {
            if (left.type !== right.type) {
              return left.type === "directory" ? -1 : 1;
            }
            return left.name.localeCompare(right.name);
          })
          .map((child) => materialize(child))
      : [];

    return {
      children,
      entry: node.entry,
      name: node.name,
      path: node.path,
      type: node.type,
    };
  };

  return materialize(rootDirectory).children;
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

function LoadingState({ label }: { label: string }) {
  return (
    <div
      aria-label={label}
      className="text-muted flex min-h-9 items-center gap-1.5 text-xs"
    >
      <RefreshCw aria-hidden="true" className="size-4 shrink-0 animate-spin" />
      <span>{label}...</span>
    </div>
  );
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
