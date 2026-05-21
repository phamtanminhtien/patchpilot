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

  if (fileError) {
    return <ErrorState className="p-3" message={fileError} />;
  }

  if (isLoading) {
    return <LoadingState className="p-3" label="Loading file" />;
  }

  return (
    <pre className="workspace-main-scroll text-ink h-full min-h-0 overflow-auto p-3 text-xs leading-5 break-words whitespace-pre-wrap">
      {file ?? "File content will appear here."}
    </pre>
  );
}
