import { ChevronDown, ChevronRight, FileCode2 } from "lucide-react";
import type { ReactNode } from "react";

import { cn } from "@/shared/ui";

import {
  gitStatusBadgeCode,
  gitStatusBadgeTone,
} from "./git/workspace-git-status";

export interface WorkspaceFileTreeStatus {
  code: string;
  label: string;
}

export interface WorkspaceFileTreeItemProps {
  "aria-expanded"?: boolean;
  depth?: number;
  disclosure?: "expanded" | "collapsed" | "none";
  icon?: ReactNode;
  isDimmed?: boolean;
  isSelected?: boolean;
  label: ReactNode;
  onClick: () => void;
  role?: "treeitem";
  status?: WorkspaceFileTreeStatus | null;
}

export function WorkspaceFileTreeItem({
  "aria-expanded": ariaExpanded,
  depth = 0,
  disclosure = "none",
  icon,
  isDimmed = false,
  isSelected = false,
  label,
  onClick,
  role,
  status,
}: WorkspaceFileTreeItemProps) {
  return (
    <button
      aria-current={isSelected ? "true" : undefined}
      aria-expanded={ariaExpanded}
      className={cn(
        "hover:bg-hover grid min-h-7 w-full min-w-0 cursor-pointer grid-cols-[minmax(0,1fr)_auto] items-center gap-1 py-0.5 pr-1.5 text-left text-xs",
        isSelected ? "bg-hover text-ink" : undefined,
        isDimmed ? "opacity-55 hover:opacity-75" : undefined,
      )}
      onClick={onClick}
      role={role}
      style={{ paddingLeft: `${depth * 0.875 + 0.375}rem` }}
      type="button"
    >
      <span className="flex min-w-0 items-center gap-1">
        <span
          className={cn(
            "grid size-3 shrink-0 place-items-center",
            isDimmed ? "text-muted" : "text-ink",
          )}
        >
          {disclosure === "expanded" ? (
            <ChevronDown aria-hidden="true" className="size-3" />
          ) : null}
          {disclosure === "collapsed" ? (
            <ChevronRight aria-hidden="true" className="size-3" />
          ) : null}
        </span>
        {icon ?? <FileCode2 aria-hidden="true" className="size-3 shrink-0" />}
        <span className="min-w-0 truncate">{label}</span>
      </span>
      <WorkspaceFileTreeStatusBadge status={status} />
    </button>
  );
}

function WorkspaceFileTreeStatusBadge({
  status,
}: {
  status: WorkspaceFileTreeStatus | null | undefined;
}) {
  if (!status) {
    return null;
  }

  if (status.label === "Ignored") {
    return null;
  }

  return (
    <span
      aria-hidden="true"
      className={cn(
        "min-w-4 shrink-0 rounded-sm px-1 text-center text-[10px] leading-4 font-semibold",
        gitStatusBadgeTone(status.label),
      )}
      title={status.label}
    >
      {gitStatusBadgeCode(status)}
    </span>
  );
}
