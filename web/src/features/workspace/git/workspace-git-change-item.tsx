import { GitBranch, Trash2, Undo2 } from "lucide-react";
import type { ButtonHTMLAttributes, ReactNode } from "react";

import { FileIcon } from "@/shared/file-icons";
import { Button, cn } from "@/shared/ui";

import type { GitChange } from "./workspace-git";
import { gitStatusBadgeCode, gitStatusBadgeTone } from "./workspace-git-status";

type GitChangeActionKind = "changes" | "staged";

export interface WorkspaceGitChangeItemProps {
  actionKind: GitChangeActionKind;
  change: GitChange;
  isDiscardingChanges: boolean;
  isSelected: boolean;
  isStagingChanges: boolean;
  isUnstagingChanges: boolean;
  onDiscard: () => void;
  onSelect: (path: string) => void;
  onStage: () => void;
  onUnstage: () => void;
}

export function WorkspaceGitChangeItem({
  actionKind,
  change,
  isDiscardingChanges,
  isSelected,
  isStagingChanges,
  isUnstagingChanges,
  onDiscard,
  onSelect,
  onStage,
  onUnstage,
}: WorkspaceGitChangeItemProps) {
  const { directory, filename } = splitGitPath(change.path);

  return (
    <div
      className={cn(
        "group hover:bg-hover focus-within:bg-hover relative grid min-h-5.5 min-w-0 grid-cols-[minmax(0,1fr)_auto] items-center gap-1 px-2 text-xs",
        isSelected ? "bg-hover text-ink" : undefined,
      )}
    >
      <button
        aria-current={isSelected ? "true" : undefined}
        aria-label={change.path}
        className="grid min-w-0 cursor-pointer grid-cols-[auto_minmax(0,1fr)] items-center gap-1 py-0.5 pr-1 text-left transition-[padding] group-focus-within:pr-12 group-hover:pr-12"
        onClick={() => onSelect(change.path)}
        title={change.displayPath}
        type="button"
      >
        <FileIcon className="size-3 shrink-0" path={change.path} />
        <span className="flex min-w-0 items-baseline gap-1">
          <span className="text-ink max-w-full min-w-0 shrink-0 truncate">
            {filename}
          </span>
          {directory ? (
            <span className="text-muted min-w-0 truncate opacity-70">
              {directory}
            </span>
          ) : null}
        </span>
      </button>

      <div className="bg-hover pointer-events-none absolute top-1/2 right-7 flex -translate-y-1/2 items-center gap-0.5 opacity-0 transition-opacity group-focus-within:pointer-events-auto group-focus-within:opacity-100 group-hover:pointer-events-auto group-hover:opacity-100">
        {actionKind === "changes" ? (
          <>
            <GitChangeActionButton
              disabled={isDiscardingChanges}
              icon={<Trash2 aria-hidden="true" className="size-3" />}
              label={`Discard change ${change.path}`}
              onClick={onDiscard}
            />
            <GitChangeActionButton
              disabled={isStagingChanges}
              icon={<GitBranch aria-hidden="true" className="size-3" />}
              label={`Stage change ${change.path}`}
              onClick={onStage}
            />
          </>
        ) : (
          <GitChangeActionButton
            disabled={isUnstagingChanges}
            icon={<Undo2 aria-hidden="true" className="size-3" />}
            label={`Unstage change ${change.path}`}
            onClick={onUnstage}
          />
        )}
      </div>

      <GitChangeStatusBadge change={change} />
    </div>
  );
}

interface GitChangeActionButtonProps extends Pick<
  ButtonHTMLAttributes<HTMLButtonElement>,
  "disabled" | "onClick"
> {
  icon: ReactNode;
  label: string;
}

export function GitChangeActionButton({
  disabled,
  icon,
  label,
  onClick,
}: GitChangeActionButtonProps) {
  return (
    <Button
      aria-label={label}
      disabled={disabled}
      icon={icon}
      onClick={onClick}
      size="icon"
      title={label}
      type="button"
      variant="action"
      className="size-4"
    >
      <span className="sr-only">{label}</span>
    </Button>
  );
}

function GitChangeStatusBadge({ change }: { change: GitChange }) {
  return (
    <span
      aria-hidden="true"
      className={cn(
        "mr-1 min-w-4 shrink-0 rounded-sm px-1 text-center text-[10px] leading-4 font-semibold",
        gitStatusBadgeTone(change.status),
      )}
      title={change.status}
    >
      {gitStatusBadgeCode({
        code: change.code,
        label: change.status,
      })}
    </span>
  );
}

function splitGitPath(path: string) {
  const normalized = path.endsWith("/") ? path.slice(0, -1) : path;
  const separatorIndex = normalized.lastIndexOf("/");

  if (separatorIndex === -1) {
    return {
      directory: "",
      filename: normalized || path,
    };
  }

  return {
    directory: normalized.slice(0, separatorIndex),
    filename: normalized.slice(separatorIndex + 1) || normalized,
  };
}
