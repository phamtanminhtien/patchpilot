import { CheckCircle2, GitBranch } from "lucide-react";

import { DiffViewer } from "@/shared/diff/diff-viewer";
import type {
  UnifiedDiffFile,
  UnifiedDiffHunk,
} from "@/shared/diff/unified-diff";

import { ErrorState } from "../components/error-state";
import { LoadingState } from "../components/loading-state";
import { MainEmptyState } from "../components/main-empty-state";

export function GitPanel({
  diff,
  diffError,
  gitError,
  hasChanges,
  isLoading,
  isPatchStaging,
  onFilePatchStage,
  onHunkPatchStage,
  selectedPath,
}: {
  diff?: string;
  diffError?: string;
  gitError?: string;
  hasChanges: boolean;
  isLoading: boolean;
  isPatchStaging: boolean;
  onFilePatchStage: (patch: string) => void;
  onHunkPatchStage: (patch: string) => void;
  selectedPath: string;
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

  return diff ? (
    <div className="grid min-h-0 grid-rows-[auto_minmax(0,1fr)]">
      <div className="border-line/35 bg-surface flex min-h-9 min-w-0 items-center justify-between gap-2 border-b px-3">
        <span className="text-ink min-w-0 truncate text-xs font-semibold">
          {selectedPath || "Workspace diff"}
        </span>
        <span className="text-muted shrink-0 text-xs">
          {diffLineCount(diff)} lines
        </span>
      </div>
      <DiffViewer
        actionLabel="Stage"
        diff={diff}
        isActionPending={isPatchStaging}
        onFileAction={(file: UnifiedDiffFile) => onFilePatchStage(file.patch)}
        onHunkAction={(hunk: UnifiedDiffHunk) => onHunkPatchStage(hunk.patch)}
      />
    </div>
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
  );
}

function diffLineCount(diff: string) {
  return diff.split("\n").length;
}
