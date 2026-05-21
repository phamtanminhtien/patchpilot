import { CheckCircle2, GitBranch, GitCommit, Loader2 } from "lucide-react";
import type { FormEvent } from "react";

import { Button, TextField } from "@/shared/ui";

import { ErrorState } from "../components/error-state";
import { LoadingState } from "../components/loading-state";
import { MainEmptyState } from "../components/main-empty-state";

export function GitPanel({
  commitError,
  commitMessage,
  diff,
  diffError,
  gitError,
  hasChanges,
  isCommitPending,
  isLoading,
  isStagePending,
  lastCommitHash,
  onCommitMessageChange,
  onCommitSubmit,
  selectedPath,
  onStageChanges,
  stagedGitPathCount,
  stageError,
  unstagedGitPathCount,
}: {
  commitError?: string;
  commitMessage: string;
  diff?: string;
  diffError?: string;
  gitError?: string;
  hasChanges: boolean;
  isCommitPending: boolean;
  isLoading: boolean;
  isStagePending: boolean;
  lastCommitHash?: string;
  onCommitMessageChange: (value: string) => void;
  onCommitSubmit: (event: FormEvent<HTMLFormElement>) => void;
  onStageChanges: () => void;
  selectedPath: string;
  stagedGitPathCount: number;
  stageError?: string;
  unstagedGitPathCount: number;
}) {
  if (gitError) {
    return <ErrorState className="p-3" message={gitError} />;
  }

  if (diffError) {
    return <ErrorState className="p-3" message={diffError} />;
  }

  if (isLoading) {
    return <LoadingState className="p-3" label="Loading diff" />;
  }

  if (!hasChanges) {
    return (
      <MainEmptyState
        icon={<CheckCircle2 aria-hidden="true" className="size-6" />}
        message="Git reports no modified, staged, or untracked paths."
        title="Working tree clean"
      />
    );
  }

  return (
    <div className="grid h-full min-h-0 grid-rows-[auto_minmax(0,1fr)]">
      <div className="bg-canvas border-line grid gap-2 border-b p-2">
        <div className="flex min-w-0 flex-wrap items-center gap-2">
          <Button
            disabled={unstagedGitPathCount === 0 || isStagePending}
            icon={
              isStagePending ? (
                <Loader2 className="animate-spin" />
              ) : (
                <GitBranch />
              )
            }
            onClick={onStageChanges}
            size="small"
            type="button"
            variant="secondary"
          >
            Stage changes
          </Button>
          <span className="text-muted text-xs">
            {unstagedGitPathCount} unstaged · {stagedGitPathCount} staged
          </span>
          {lastCommitHash ? (
            <span className="text-muted min-w-0 truncate text-xs">
              Committed {lastCommitHash.slice(0, 12)}
            </span>
          ) : null}
        </div>

        <form
          className="grid gap-2 sm:grid-cols-[minmax(0,1fr)_auto]"
          onSubmit={onCommitSubmit}
        >
          <TextField
            label="Commit message"
            name="commit-message"
            onChange={(event) => onCommitMessageChange(event.target.value)}
            placeholder="Describe the selected changes"
            size="compact"
            value={commitMessage}
          />
          <Button
            disabled={
              stagedGitPathCount === 0 ||
              commitMessage.trim().length === 0 ||
              isCommitPending
            }
            icon={
              isCommitPending ? (
                <Loader2 className="animate-spin" />
              ) : (
                <GitCommit />
              )
            }
            size="compact"
          >
            Commit
          </Button>
        </form>

        {stageError ? <ErrorState message={stageError} /> : null}
        {commitError ? <ErrorState message={commitError} /> : null}
      </div>

      {diff ? (
        <pre className="workspace-main-scroll text-ink h-full min-h-0 overflow-auto p-3 text-xs leading-5 break-words whitespace-pre-wrap">
          {diff}
        </pre>
      ) : (
        <MainEmptyState
          icon={<GitBranch aria-hidden="true" className="size-6" />}
          message={
            selectedPath
              ? "No diff is available for the selected path."
              : "Select a changed file to inspect its diff."
          }
          title="No diff output"
        />
      )}
    </div>
  );
}
