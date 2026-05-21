import type { ReactNode } from "react";

import type { Command, CommandOutput } from "@/shared/api";
import { cn } from "@/shared/ui";

import { LoadingState } from "../components/loading-state";
import type { WorkspacePanel } from "../workspace-panels";

export function WorkspaceBottomPanel({
  activePanel,
  gitRawStatus,
  activeCommand,
  commandOutput,
  isGitLoading,
  selectedPath,
}: {
  activeCommand: Command | null;
  activePanel: WorkspacePanel;
  commandOutput: CommandOutput[];
  gitRawStatus?: string;
  isGitLoading: boolean;
  selectedPath: string;
}) {
  return (
    <section className="bg-panel border-line grid min-h-0 grid-rows-[auto_minmax(0,1fr)] overflow-hidden border-t shadow-sm">
      <div className="bg-hover flex min-h-8 min-w-0 items-center gap-1 overflow-x-auto px-1.5">
        <BottomTab active={activePanel === "files"}>File</BottomTab>
        <BottomTab active={activePanel === "git"}>Git status</BottomTab>
        <BottomTab active={activePanel === "commands"}>Command</BottomTab>
        <BottomTab active={activePanel === "preview"}>Preview</BottomTab>
      </div>

      <div className="min-h-0 overflow-auto p-2">
        {activePanel === "files" ? (
          <p className="text-muted text-xs">
            {selectedPath
              ? `Selected path: ${selectedPath}`
              : "Select a file to populate the workspace output area."}
          </p>
        ) : null}

        {activePanel === "git" ? (
          isGitLoading ? (
            <LoadingState label="Loading raw Git status" />
          ) : (
            <pre className="text-ink text-xs leading-5 break-words whitespace-pre-wrap">
              {gitStatusPreview(gitRawStatus)}
            </pre>
          )
        ) : null}

        {activePanel === "commands" ? (
          activeCommand ? (
            <pre className="text-ink text-xs leading-5 break-words whitespace-pre-wrap">
              {`${activeCommand.status} ${activeCommand.id}\n${activeCommand.command}\n${latestOutput(commandOutput)}`}
            </pre>
          ) : (
            <p className="text-muted text-xs">
              Run a command to see its latest output here.
            </p>
          )
        ) : null}

        {activePanel === "preview" ? (
          <p className="text-muted text-xs">
            No detected port API is connected in this MVP slice.
          </p>
        ) : null}
      </div>
    </section>
  );
}

function latestOutput(output: CommandOutput[]) {
  const text = output
    .slice(-2)
    .map((chunk) => chunk.chunk)
    .join("");
  const trimmed = text.trim();
  if (!trimmed) {
    return "No output yet.";
  }
  return trimmed.length > 220 ? `${trimmed.slice(-220)}` : trimmed;
}

function gitStatusPreview(status?: string) {
  if (!status) {
    return "Working tree clean.";
  }
  const lines = status.trim().split("\n");
  const preview = lines.slice(0, 5).join("\n");
  const remainder = lines.length - 5;
  return remainder > 0 ? `${preview}\n... ${remainder} more paths` : preview;
}

function BottomTab({
  active,
  children,
}: {
  active: boolean;
  children: ReactNode;
}) {
  return (
    <span
      className={cn(
        "text-muted inline-flex min-h-6 shrink-0 items-center rounded-sm px-1.5 text-xs font-medium",
        active ? "bg-panel text-ink shadow-sm" : undefined,
      )}
    >
      {children}
    </span>
  );
}
