import {
  ChevronDown,
  GitBranch,
  MoreHorizontal,
  Trash2,
  Undo2,
} from "lucide-react";
import type { ReactNode } from "react";
import { useState } from "react";

import {
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogRoot,
  AlertDialogTitle,
  Button,
  PopoverClose,
  PopoverContent,
  PopoverRoot,
  PopoverTrigger,
} from "@/shared/ui";

import { EmptyState } from "../components/empty-state";
import { ErrorState } from "../components/error-state";
import { LoadingState } from "../components/loading-state";
import {
  type GitChange,
  isGitChangeStageable,
  isStagedGitChange,
  isUnstagedGitChange,
  visibleGitChanges,
} from "./workspace-git";
import { WorkspaceGitChangeItem } from "./workspace-git-change-item";

export function GitChangeList({
  changes,
  error,
  isDiscardingChanges,
  isLoading,
  isStagingChanges,
  isUnstagingChanges,
  onChangesDiscard,
  onChangesStage,
  onSelect,
  onStagedChangesUnstage,
  selectedPath,
}: {
  changes: GitChange[];
  error?: string;
  isDiscardingChanges: boolean;
  isLoading: boolean;
  isStagingChanges: boolean;
  isUnstagingChanges: boolean;
  onChangesDiscard: (paths: string[]) => void;
  onChangesStage: (paths: string[]) => void;
  onSelect: (path: string) => void;
  onStagedChangesUnstage: (paths: string[]) => void;
  selectedPath: string;
}) {
  const [pendingDiscard, setPendingDiscard] = useState<{
    label: string;
    paths: string[];
  } | null>(null);
  const visibleChanges = visibleGitChanges(changes);
  const stagedChanges = visibleChanges.filter((change) =>
    isStagedGitChange(change),
  );
  const unstagedChanges = visibleChanges.filter((change) =>
    isUnstagedGitChange(change),
  );

  if (error) {
    return <ErrorState message={error} />;
  }

  if (isLoading) {
    return <LoadingState label="Loading Git status" />;
  }

  if (visibleChanges.length === 0) {
    return <EmptyState message="Working tree is clean." />;
  }

  return (
    <div className="grid gap-1">
      <GitChangeSection
        actionKind="staged"
        allSectionPathCount={stagedChanges.length}
        changes={stagedChanges}
        isDiscardingChanges={isDiscardingChanges}
        isStagingChanges={isStagingChanges}
        isUnstagingChanges={isUnstagingChanges}
        label="Staged Changes"
        onChangesDiscard={(paths, label) => setPendingDiscard({ label, paths })}
        onChangesStage={onChangesStage}
        onSelect={onSelect}
        onStagedChangesUnstage={onStagedChangesUnstage}
        selectedPath={selectedPath}
      />
      <GitChangeSection
        actionKind="changes"
        allSectionPathCount={unstagedChanges.length}
        changes={unstagedChanges}
        isDiscardingChanges={isDiscardingChanges}
        isStagingChanges={isStagingChanges}
        isUnstagingChanges={isUnstagingChanges}
        label="Changes"
        onChangesDiscard={(paths, label) => setPendingDiscard({ label, paths })}
        onChangesStage={onChangesStage}
        onSelect={onSelect}
        onStagedChangesUnstage={onStagedChangesUnstage}
        selectedPath={selectedPath}
      />
      <AlertDialogRoot
        onOpenChange={(open) => {
          if (!open) {
            setPendingDiscard(null);
          }
        }}
        open={pendingDiscard !== null}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Discard changes?</AlertDialogTitle>
            <AlertDialogDescription>
              {pendingDiscard
                ? `Discard ${pendingDiscard.paths.length} ${
                    pendingDiscard.paths.length === 1 ? "path" : "paths"
                  } from ${pendingDiscard.label}. This cannot be undone from PatchPilot.`
                : "Discard changes."}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              onClick={() => {
                if (pendingDiscard) {
                  onChangesDiscard(pendingDiscard.paths);
                }
              }}
            >
              Discard
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialogRoot>
    </div>
  );
}

function GitChangeSection({
  actionKind,
  allSectionPathCount,
  changes,
  isDiscardingChanges,
  isStagingChanges,
  isUnstagingChanges,
  label,
  onChangesDiscard,
  onChangesStage,
  onSelect,
  onStagedChangesUnstage,
  selectedPath,
}: {
  actionKind: "changes" | "staged";
  allSectionPathCount: number;
  changes: GitChange[];
  isDiscardingChanges: boolean;
  isStagingChanges: boolean;
  isUnstagingChanges: boolean;
  label: string;
  onChangesDiscard: (paths: string[], label: string) => void;
  onChangesStage: (paths: string[]) => void;
  onSelect: (path: string) => void;
  onStagedChangesUnstage: (paths: string[]) => void;
  selectedPath: string;
}) {
  const actionablePaths = changes
    .filter((change) => isGitChangeStageable(change))
    .map((change) => change.path);

  return (
    <section className="grid">
      <div className="text-ink grid min-h-8 grid-cols-[auto_minmax(0,1fr)_auto_auto] items-center gap-1 px-1 text-xs font-semibold">
        <ChevronDown aria-hidden="true" className="text-muted size-4" />
        <span className="truncate">{label}</span>
        <span
          aria-label={`${allSectionPathCount} ${label.toLowerCase()} paths`}
          className="bg-hover text-muted grid min-w-6 place-items-center rounded-full px-1.5 py-0.5 text-xs font-semibold"
        >
          {allSectionPathCount}
        </span>
        <GitChangeSectionActions
          actionKind={actionKind}
          isDiscardingChanges={isDiscardingChanges}
          isStagingChanges={isStagingChanges}
          isUnstagingChanges={isUnstagingChanges}
          onDiscard={(paths) => onChangesDiscard(paths, label.toLowerCase())}
          onStage={() => onChangesStage(actionablePaths)}
          onUnstage={() => onStagedChangesUnstage(actionablePaths)}
          pathCount={actionablePaths.length}
          paths={actionablePaths}
        />
      </div>

      {changes.length === 0 ? (
        <p className="text-muted px-6 py-1 text-xs">No paths.</p>
      ) : (
        changes.map((change) => (
          <WorkspaceGitChangeItem
            actionKind={actionKind}
            change={change}
            isDiscardingChanges={isDiscardingChanges}
            isSelected={selectedPath === change.path}
            isStagingChanges={isStagingChanges}
            isUnstagingChanges={isUnstagingChanges}
            key={`${label}-${change.id}`}
            onDiscard={() => onChangesDiscard([change.path], change.path)}
            onSelect={onSelect}
            onStage={() => onChangesStage([change.path])}
            onUnstage={() => onStagedChangesUnstage([change.path])}
          />
        ))
      )}
    </section>
  );
}

function GitChangeSectionActions({
  actionKind,
  isDiscardingChanges,
  isStagingChanges,
  isUnstagingChanges,
  onDiscard,
  onStage,
  onUnstage,
  pathCount,
  paths,
}: {
  actionKind: "changes" | "staged";
  isDiscardingChanges: boolean;
  isStagingChanges: boolean;
  isUnstagingChanges: boolean;
  onDiscard: (paths: string[]) => void;
  onStage: () => void;
  onUnstage: () => void;
  pathCount: number;
  paths: string[];
}) {
  if (pathCount === 0) {
    return null;
  }

  return (
    <PopoverRoot>
      <PopoverTrigger asChild>
        <Button
          aria-label={`${actionKind === "changes" ? "Changes" : "Staged changes"} actions`}
          className="size-6"
          icon={<MoreHorizontal />}
          size="icon"
          type="button"
          variant="action"
        />
      </PopoverTrigger>
      <PopoverContent>
        {actionKind === "changes" ? (
          <>
            <GitChangePopoverButton
              disabled={isStagingChanges}
              icon={<GitBranch aria-hidden="true" className="size-3.5" />}
              label="Stage all changes"
              onClick={onStage}
            />
            <GitChangePopoverButton
              disabled={isDiscardingChanges}
              icon={<Trash2 aria-hidden="true" className="size-3.5" />}
              label="Discard all changes"
              onClick={() => onDiscard(paths)}
            />
          </>
        ) : (
          <GitChangePopoverButton
            disabled={isUnstagingChanges}
            icon={<Undo2 aria-hidden="true" className="size-3.5" />}
            label="Unstage all staged changes"
            onClick={onUnstage}
          />
        )}
      </PopoverContent>
    </PopoverRoot>
  );
}

function GitChangePopoverButton({
  disabled,
  icon,
  label,
  onClick,
}: {
  disabled: boolean;
  icon: ReactNode;
  label: string;
  onClick: () => void;
}) {
  return (
    <PopoverClose asChild>
      <Button
        className="min-h-8 justify-start px-2 shadow-none"
        disabled={disabled}
        icon={icon}
        onClick={onClick}
        size="small"
        type="button"
        variant="ghost"
        width="full"
      >
        {label}
      </Button>
    </PopoverClose>
  );
}
