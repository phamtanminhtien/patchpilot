import { ChevronDown, GitBranch, Trash2, Undo2 } from "lucide-react";

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
import {
  GitChangeActionButton,
  WorkspaceGitChangeItem,
} from "./workspace-git-change-item";

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
        changes={stagedChanges}
        isDiscardingChanges={isDiscardingChanges}
        isStagingChanges={isStagingChanges}
        isUnstagingChanges={isUnstagingChanges}
        label="Staged Changes"
        onChangesDiscard={onChangesDiscard}
        onChangesStage={onChangesStage}
        onSelect={onSelect}
        onStagedChangesUnstage={onStagedChangesUnstage}
        selectedPath={selectedPath}
      />
      <GitChangeSection
        actionKind="changes"
        changes={unstagedChanges}
        isDiscardingChanges={isDiscardingChanges}
        isStagingChanges={isStagingChanges}
        isUnstagingChanges={isUnstagingChanges}
        label="Changes"
        onChangesDiscard={onChangesDiscard}
        onChangesStage={onChangesStage}
        onSelect={onSelect}
        onStagedChangesUnstage={onStagedChangesUnstage}
        selectedPath={selectedPath}
      />
    </div>
  );
}

function GitChangeSection({
  actionKind,
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
  changes: GitChange[];
  isDiscardingChanges: boolean;
  isStagingChanges: boolean;
  isUnstagingChanges: boolean;
  label: string;
  onChangesDiscard: (paths: string[]) => void;
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
          aria-label={`${changes.length} ${label.toLowerCase()} paths`}
          className="bg-hover text-muted grid min-w-6 place-items-center rounded-full px-1.5 py-0.5 text-xs font-semibold"
        >
          {changes.length}
        </span>
        <GitChangeSectionActions
          actionKind={actionKind}
          isDiscardingChanges={isDiscardingChanges}
          isStagingChanges={isStagingChanges}
          isUnstagingChanges={isUnstagingChanges}
          onDiscard={() => onChangesDiscard(actionablePaths)}
          onStage={() => onChangesStage(actionablePaths)}
          onUnstage={() => onStagedChangesUnstage(actionablePaths)}
          pathCount={actionablePaths.length}
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
            onDiscard={() => onChangesDiscard([change.path])}
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
}: {
  actionKind: "changes" | "staged";
  isDiscardingChanges: boolean;
  isStagingChanges: boolean;
  isUnstagingChanges: boolean;
  onDiscard: () => void;
  onStage: () => void;
  onUnstage: () => void;
  pathCount: number;
}) {
  if (pathCount === 0) {
    return null;
  }

  return (
    <div className="flex shrink-0 items-center gap-0.5">
      {actionKind === "changes" ? (
        <>
          <GitChangeActionButton
            disabled={isDiscardingChanges}
            icon={<Trash2 aria-hidden="true" className="size-3.5" />}
            label="Discard all changes"
            onClick={onDiscard}
          />
          <GitChangeActionButton
            disabled={isStagingChanges}
            icon={<GitBranch aria-hidden="true" className="size-3.5" />}
            label="Stage all changes"
            onClick={onStage}
          />
        </>
      ) : (
        <GitChangeActionButton
          disabled={isUnstagingChanges}
          icon={<Undo2 aria-hidden="true" className="size-3.5" />}
          label="Unstage all staged changes"
          onClick={onUnstage}
        />
      )}
    </div>
  );
}
