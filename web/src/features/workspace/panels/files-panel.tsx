import { Files } from "lucide-react";

import { ErrorState } from "../components/error-state";
import { LoadingState } from "../components/loading-state";
import { MainEmptyState } from "../components/main-empty-state";

export function FilesPanel({
  file,
  fileError,
  isLoading,
  selectedPath,
}: {
  file?: string;
  fileError?: string;
  isLoading: boolean;
  selectedPath: string;
}) {
  if (selectedPath.length === 0) {
    return (
      <MainEmptyState
        icon={<Files aria-hidden="true" className="size-6" />}
        message="Select a file from the workspace list to inspect its text content."
        title="No file selected"
      />
    );
  }

  return (
    <div className="grid min-h-0 grid-rows-[auto_minmax(0,1fr)]">
      <div className="bg-hover flex min-h-9 min-w-0 items-center justify-between gap-2 px-3">
        <span className="text-ink min-w-0 truncate text-xs font-semibold">
          {selectedPath}
        </span>
        <span className="text-muted shrink-0 text-xs">
          {file ? `${lineCount(file)} lines` : "Text file"}
        </span>
      </div>

      {fileError ? (
        <ErrorState className="p-3" message={fileError} />
      ) : isLoading ? (
        <LoadingState className="p-3" label="Loading file" />
      ) : (
        <pre className="workspace-main-scroll text-ink h-full min-h-0 overflow-auto p-3 text-xs leading-5 break-words whitespace-pre-wrap">
          {file ?? "File content will appear here."}
        </pre>
      )}
    </div>
  );
}

function lineCount(content: string) {
  return content.split("\n").length;
}
