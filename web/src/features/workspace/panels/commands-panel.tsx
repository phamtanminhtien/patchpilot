import {
  AlertTriangle,
  CheckCircle2,
  Clock,
  Loader2,
  Play,
  Send,
  Square,
  XCircle,
} from "lucide-react";
import type { FormEvent } from "react";

import type { Command, CommandOutput } from "@/shared/api";
import { Button, cn, StatusPill, TextField } from "@/shared/ui";

import { ErrorState } from "../components/error-state";
import { LoadingState } from "../components/loading-state";
import { MainEmptyState } from "../components/main-empty-state";

const commonCommands = [
  "pnpm --dir web test",
  "pnpm --dir web build",
  "go test ./...",
  "git status",
];

export function CommandsPanel({
  activeCommand,
  activeCommandId,
  commandText,
  confirmationCommand,
  error,
  isLoadingProcesses,
  isPending,
  isStopping,
  onCancelConfirmation,
  onCommandChange,
  onCommandConfirm,
  onCommandSelect,
  onCommandShortcut,
  onCommandStop,
  onSubmit,
  output,
  processes,
  stopError,
}: {
  activeCommand: Command | null;
  activeCommandId: string;
  commandText: string;
  confirmationCommand: string;
  error?: string;
  isLoadingProcesses: boolean;
  isPending: boolean;
  isStopping: boolean;
  onCancelConfirmation: () => void;
  onCommandChange: (value: string) => void;
  onCommandConfirm: () => void;
  onCommandSelect: (commandId: string) => void;
  onCommandShortcut: (command: string) => void;
  onCommandStop: () => void;
  onSubmit: (event: FormEvent<HTMLFormElement>) => void;
  output: CommandOutput[];
  processes: Command[];
  queuedCommand: Command | null;
  stopError?: string;
}) {
  const canStop = activeCommand?.status === "running";

  return (
    <div className="grid min-h-0 grid-rows-[auto_minmax(0,1fr)] overflow-hidden">
      <div className="border-line bg-panel grid gap-2 border-b p-3">
        <form className="grid gap-1.5" onSubmit={onSubmit}>
          <div className="grid gap-2 sm:grid-cols-[minmax(0,1fr)_auto]">
            <TextField
              className="bg-hover"
              id="workspace-command"
              label="Command"
              labelClassName="text-muted font-semibold"
              name="workspace-command"
              onChange={(event) => onCommandChange(event.target.value)}
              placeholder="pnpm --dir web test"
              size="compact"
              value={commandText}
            />
            <Button
              className="min-h-10"
              disabled={isPending || commandText.trim().length === 0}
              icon={isPending ? <Loader2 className="animate-spin" /> : <Send />}
              size="compact"
            >
              Run
            </Button>
          </div>
        </form>

        <div className="flex min-w-0 gap-1 overflow-x-auto">
          {commonCommands.map((command) => (
            <Button
              className="min-h-8 shrink-0 px-2"
              key={command}
              onClick={() => onCommandShortcut(command)}
              size="small"
              type="button"
              variant="action"
            >
              {command}
            </Button>
          ))}
        </div>

        {confirmationCommand ? (
          <div className="bg-hover grid gap-2 rounded-sm p-2.5">
            <div className="flex min-w-0 items-center gap-2">
              <AlertTriangle className="text-warning size-4 shrink-0" />
              <p className="text-ink min-w-0 text-xs font-semibold">
                Confirm risky command
              </p>
            </div>
            <p className="text-muted text-xs break-all">
              {confirmationCommand}
            </p>
            <div className="flex justify-end gap-2">
              <Button
                onClick={onCancelConfirmation}
                size="small"
                type="button"
                variant="secondary"
              >
                Cancel
              </Button>
              <Button
                disabled={isPending}
                icon={
                  isPending ? <Loader2 className="animate-spin" /> : <Play />
                }
                onClick={onCommandConfirm}
                size="small"
                type="button"
              >
                Run anyway
              </Button>
            </div>
          </div>
        ) : null}

        {error ? <ErrorState message={error} /> : null}
        {stopError ? <ErrorState message={stopError} /> : null}
      </div>

      <div className="grid min-h-0 overflow-hidden lg:grid-cols-[16rem_minmax(0,1fr)]">
        <CommandList
          activeCommandId={activeCommandId}
          isLoading={isLoadingProcesses}
          onSelect={onCommandSelect}
          processes={processes}
        />
        <div className="border-line grid min-h-0 grid-rows-[auto_minmax(0,1fr)] overflow-hidden border-l">
          {activeCommand ? (
            <>
              <CommandSummary
                command={activeCommand}
                isStopping={isStopping}
                onStop={onCommandStop}
                showStop={canStop}
              />
              <CommandOutputViewer command={activeCommand} output={output} />
            </>
          ) : (
            <MainEmptyState
              icon={<Play aria-hidden="true" className="size-6" />}
              message="Run a test or build command to stream workspace output here."
              title="No command selected"
            />
          )}
        </div>
      </div>
    </div>
  );
}

function CommandList({
  activeCommandId,
  isLoading,
  onSelect,
  processes,
}: {
  activeCommandId: string;
  isLoading: boolean;
  onSelect: (commandId: string) => void;
  processes: Command[];
}) {
  if (isLoading && processes.length === 0) {
    return <LoadingState label="Loading commands" />;
  }

  if (processes.length === 0) {
    return (
      <div className="p-3">
        <p className="text-muted text-xs">No command history.</p>
      </div>
    );
  }

  return (
    <div className="min-h-0 overflow-auto">
      <div className="border-line border-t">
        {processes.map((command) => (
          <button
            key={command.id}
            className={cn(
              "border-line hover:bg-hover grid min-h-12 w-full min-w-0 gap-1 border-b px-3 py-2 text-left transition",
              activeCommandId === command.id
                ? "bg-hover text-ink shadow-[inset_3px_0_0_var(--pp-color-focus)]"
                : "",
            )}
            onClick={() => onSelect(command.id)}
            type="button"
          >
            <span className="text-ink truncate text-xs font-semibold">
              {command.command}
            </span>
            <span className="flex min-w-0 items-center justify-between gap-1">
              <span className="text-muted truncate text-xs">
                {shortId(command.id)}
              </span>
              <span className="text-muted text-xs">{command.status}</span>
            </span>
          </button>
        ))}
      </div>
    </div>
  );
}

function CommandSummary({
  command,
  isStopping,
  onStop,
  showStop,
}: {
  command: Command;
  isStopping: boolean;
  onStop: () => void;
  showStop: boolean;
}) {
  const icon =
    command.status === "exited" && command.exitCode === 0 ? (
      <CheckCircle2 className="text-accent size-4" />
    ) : command.status === "failed" || command.exitCode ? (
      <XCircle className="text-warning size-4" />
    ) : (
      <Clock className="text-muted size-4" />
    );

  return (
    <div className="bg-panel border-line border-b px-3 py-2">
      <div className="grid min-w-0 gap-2 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-center">
        <div className="flex min-w-0 items-center gap-2">
          {icon}
          <div className="min-w-0">
            <p className="text-ink truncate text-xs font-semibold">
              {command.command}
            </p>
            <p className="text-muted text-xs">{summaryText(command)}</p>
          </div>
        </div>
        <div className="flex items-center gap-2">
          <StatusPill status={command.status} />
          {showStop ? (
            <Button
              disabled={isStopping}
              icon={
                isStopping ? <Loader2 className="animate-spin" /> : <Square />
              }
              onClick={onStop}
              size="small"
              type="button"
              variant="secondary"
            >
              Stop
            </Button>
          ) : null}
        </div>
      </div>
    </div>
  );
}

function CommandOutputViewer({
  command,
  output,
}: {
  command: Command;
  output: CommandOutput[];
}) {
  if (output.length === 0) {
    return (
      <pre className="bg-canvas text-muted min-h-0 overflow-auto p-4 font-mono text-xs leading-5">
        Waiting for output...
      </pre>
    );
  }

  return (
    <pre
      aria-label={`Output for ${command.command}`}
      className="bg-canvas text-ink min-h-0 overflow-auto p-4 font-mono text-xs leading-5 whitespace-pre-wrap"
    >
      {output.map((chunk) => (
        <span
          className={chunk.stream === "stderr" ? "text-warning" : undefined}
          key={chunk.id}
        >
          {chunk.chunk}
        </span>
      ))}
    </pre>
  );
}

function shortId(id: string) {
  return id.length > 14 ? id.slice(0, 14) : id;
}

function summaryText(command: Command) {
  if (command.status === "running") {
    return "Running";
  }
  if (command.status === "queued") {
    return "Queued";
  }
  const exit =
    command.exitCode === null || command.exitCode === undefined
      ? "no exit code"
      : `exit ${command.exitCode}`;
  const duration =
    command.durationMs === null || command.durationMs === undefined
      ? ""
      : ` · ${formatDuration(command.durationMs)}`;
  return `${exit}${duration}`;
}

function formatDuration(durationMs: number) {
  if (durationMs < 1000) {
    return `${durationMs}ms`;
  }
  return `${(durationMs / 1000).toFixed(1)}s`;
}
