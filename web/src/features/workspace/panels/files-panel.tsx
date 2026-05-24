import { Edit3, Files, Loader2, Save, X } from "lucide-react";
import { useState } from "react";

import { Button } from "@/shared/ui";

import { ErrorState } from "../components/error-state";
import { LoadingState } from "../components/loading-state";
import { MainEmptyState } from "../components/main-empty-state";

export function FilesPanel({
  file,
  fileError,
  isLoading,
  isSaving,
  onSave,
  saveError,
  selectedPath,
}: {
  file?: string;
  fileError?: string;
  isLoading: boolean;
  isSaving: boolean;
  onSave: (content: string) => void;
  saveError?: string;
  selectedPath: string;
}) {
  const fileSource = file ?? "";
  const [editState, setEditState] = useState({
    draft: "",
    isEditing: false,
    path: "",
    source: "",
  });
  const isEditStateCurrent =
    editState.path === selectedPath && editState.source === fileSource;
  const draft = isEditStateCurrent ? editState.draft : fileSource;
  const isEditing = isEditStateCurrent ? editState.isEditing : false;

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
        <div className="flex shrink-0 items-center gap-1">
          <span className="text-muted text-xs">
            {file ? `${lineCount(file)} lines` : "Text file"}
          </span>
          {fileError || isLoading || file === undefined ? null : isEditing ? (
            <>
              <Button
                aria-label="Cancel file edit"
                icon={<X />}
                onClick={() => {
                  setEditState({
                    draft: fileSource,
                    isEditing: false,
                    path: selectedPath,
                    source: fileSource,
                  });
                }}
                size="icon"
                type="button"
                variant="action"
              />
              <Button
                aria-label="Save file"
                disabled={
                  draft === file || isSaving || selectedPath.length === 0
                }
                icon={
                  isSaving ? <Loader2 className="animate-spin" /> : <Save />
                }
                onClick={() => onSave(draft)}
                size="icon"
                type="button"
                variant="action"
              />
            </>
          ) : (
            <Button
              aria-label="Edit file"
              icon={<Edit3 />}
              onClick={() =>
                setEditState({
                  draft: fileSource,
                  isEditing: true,
                  path: selectedPath,
                  source: fileSource,
                })
              }
              size="icon"
              type="button"
              variant="action"
            />
          )}
        </div>
      </div>

      {fileError ? (
        <ErrorState className="p-3" message={fileError} />
      ) : isLoading ? (
        <LoadingState className="p-3" label="Loading file" />
      ) : isEditing ? (
        <div className="grid min-h-0 grid-rows-[minmax(0,1fr)_auto]">
          <label className="sr-only" htmlFor="workspace-file-editor">
            File content
          </label>
          <textarea
            className="workspace-main-scroll bg-panel text-ink h-full min-h-0 resize-none overflow-auto p-3 font-mono text-xs leading-5 whitespace-pre focus-visible:!outline-none"
            id="workspace-file-editor"
            onChange={(event) =>
              setEditState({
                draft: event.target.value,
                isEditing: true,
                path: selectedPath,
                source: fileSource,
              })
            }
            spellCheck={false}
            value={draft}
          />
          {saveError ? (
            <ErrorState className="p-3" message={saveError} />
          ) : null}
        </div>
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
