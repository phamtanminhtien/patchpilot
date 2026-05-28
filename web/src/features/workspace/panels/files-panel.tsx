import { Circle, Files, Loader2, Save, SaveAll, X } from "lucide-react";
import { useRef, useState } from "react";

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
  cn,
} from "@/shared/ui";

import { ErrorState } from "../components/error-state";
import { LoadingState } from "../components/loading-state";
import { MainEmptyState } from "../components/main-empty-state";
import { CodeEditor } from "./code-editor";

export function FilesPanel({
  activeDraft,
  dirtyPaths,
  file,
  fileError,
  isLoading,
  isSaving,
  onCloseTab,
  onDraftChange,
  onSave,
  onSaveAll,
  onSelectTab,
  openTabs,
  saveError,
  selectedPath,
}: {
  activeDraft: string;
  dirtyPaths: string[];
  file?: string;
  fileError?: string;
  isLoading: boolean;
  isSaving: boolean;
  onCloseTab: (path: string) => void;
  onDraftChange: (path: string, content: string) => void;
  onSave: (path: string, content: string) => void;
  onSaveAll: () => void;
  onSelectTab: (path: string) => void;
  openTabs: string[];
  saveError?: string;
  selectedPath: string;
}) {
  const closeCandidateRef = useRef("");
  const [closeCandidate, setCloseCandidate] = useState("");
  const dirtyPathSet = new Set(dirtyPaths);
  const isActiveDirty =
    selectedPath.length > 0 && dirtyPathSet.has(selectedPath);
  const hasDirtyTabs = dirtyPaths.length > 0;
  const lineSummary = file ? `${lineCount(file)} lines` : "Text file";

  if (selectedPath.length === 0 && openTabs.length === 0) {
    return (
      <MainEmptyState
        icon={<Files aria-hidden="true" className="size-6" />}
        message="Select a file from the workspace list to inspect its text content."
        title="No file selected"
      />
    );
  }

  function requestTabClose(path: string) {
    if (dirtyPathSet.has(path)) {
      closeCandidateRef.current = path;
      setCloseCandidate(path);
      return;
    }
    onCloseTab(path);
  }

  return (
    <div className="grid h-full min-h-0 grid-rows-[auto_auto_minmax(0,1fr)] overflow-hidden">
      <div className="border-line/35 bg-surface flex min-h-9 min-w-0 items-center justify-between gap-2 border-b px-3">
        <span className="text-ink min-w-0 truncate text-xs font-semibold">
          {selectedPath || "Workspace editor"}
        </span>
        <div className="flex shrink-0 items-center gap-1">
          <span className="text-muted text-xs">{lineSummary}</span>
          <Button
            aria-label="Save file"
            disabled={!isActiveDirty || isSaving || selectedPath.length === 0}
            icon={isSaving ? <Loader2 className="animate-spin" /> : <Save />}
            onClick={() => onSave(selectedPath, activeDraft)}
            size="icon"
            type="button"
            variant="action"
          />
          <Button
            aria-label="Save all files"
            disabled={!hasDirtyTabs || isSaving}
            icon={isSaving ? <Loader2 className="animate-spin" /> : <SaveAll />}
            onClick={onSaveAll}
            size="icon"
            type="button"
            variant="action"
          />
        </div>
      </div>

      <div className="border-line/35 bg-panel flex min-h-7 min-w-0 items-end gap-1 overflow-x-auto border-b px-1 pt-1">
        {openTabs.map((path) => {
          const isActive = path === selectedPath;
          const isDirty = dirtyPathSet.has(path);

          return (
            <div
              className={cn(
                "border-line/45 flex h-7 max-w-64 min-w-0 items-center gap-1 rounded-t-md border border-b-0 px-2",
                isActive ? "bg-raised text-ink" : "bg-surface/70 text-muted",
              )}
              key={path}
            >
              <button
                className="flex min-w-0 flex-1 cursor-pointer items-center gap-1.5 text-left text-xs"
                onClick={() => onSelectTab(path)}
                type="button"
              >
                {isDirty ? (
                  <Circle
                    aria-hidden="true"
                    className="size-2 shrink-0 fill-current"
                  />
                ) : null}
                <span className="min-w-0 truncate">{fileName(path)}</span>
              </button>
              <Button
                aria-label={`Close ${path}`}
                icon={<X />}
                onClick={() => requestTabClose(path)}
                size="icon"
                type="button"
                variant="action"
              />
            </div>
          );
        })}
      </div>

      {fileError ? (
        <ErrorState className="p-3" message={fileError} />
      ) : isLoading ? (
        <LoadingState className="p-3" label="Loading file" />
      ) : selectedPath.length === 0 ? (
        <MainEmptyState
          icon={<Files aria-hidden="true" className="size-6" />}
          message="Open a file tab to begin editing."
          title="No active tab"
        />
      ) : (
        <div className="grid min-h-0 grid-rows-[minmax(0,1fr)_auto]">
          <CodeEditor
            ariaLabel="File content"
            className="h-full"
            onChange={(content) => onDraftChange(selectedPath, content)}
            onSave={() => onSave(selectedPath, activeDraft)}
            path={selectedPath}
            value={activeDraft}
          />
          {saveError ? (
            <ErrorState className="p-3" message={saveError} />
          ) : null}
        </div>
      )}

      <AlertDialogRoot
        onOpenChange={(open) => {
          if (!open) {
            setCloseCandidate("");
          }
        }}
        open={closeCandidate.length > 0}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Close unsaved file?</AlertDialogTitle>
            <AlertDialogDescription>
              {closeCandidate} has unsaved edits. Closing the tab will discard
              the draft.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              onClick={() => {
                onCloseTab(closeCandidateRef.current || closeCandidate);
                closeCandidateRef.current = "";
                setCloseCandidate("");
              }}
            >
              Close tab
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialogRoot>
    </div>
  );
}

function fileName(path: string) {
  return path.split("/").pop() ?? path;
}

function lineCount(content: string) {
  return content.split("\n").length;
}
