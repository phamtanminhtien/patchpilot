import { Loader2, Play, Send } from "lucide-react";
import type { FormEvent } from "react";

import type { Command } from "@/shared/api";
import { Button, StatusPill, TextField } from "@/shared/ui";

import { ErrorState } from "../components/error-state";
import { MainEmptyState } from "../components/main-empty-state";

export function CommandsPanel({
  commandText,
  error,
  isPending,
  onCommandChange,
  onSubmit,
  queuedCommand,
}: {
  commandText: string;
  error?: string;
  isPending: boolean;
  onCommandChange: (value: string) => void;
  onSubmit: (event: FormEvent<HTMLFormElement>) => void;
  queuedCommand: Command | null;
}) {
  return (
    <div className="grid gap-3 p-3">
      <form
        className="grid gap-2 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-end"
        onSubmit={onSubmit}
      >
        <TextField
          label="Command"
          name="workspace-command"
          onChange={(event) => onCommandChange(event.target.value)}
          placeholder="pnpm --dir web test"
          size="compact"
          value={commandText}
        />
        <Button
          disabled={isPending || commandText.trim().length === 0}
          icon={isPending ? <Loader2 className="animate-spin" /> : <Send />}
          size="compact"
        >
          Queue command
        </Button>
      </form>

      {error ? <ErrorState message={error} /> : null}

      {queuedCommand ? (
        <div className="bg-hover grid gap-1.5 rounded-md p-2.5">
          <div className="flex min-w-0 items-center justify-between gap-2">
            <p className="text-ink truncate text-xs font-semibold">
              Command accepted
            </p>
            <StatusPill status={queuedCommand.status} />
          </div>
          <p className="text-muted text-xs break-all">
            {queuedCommand.command}
          </p>
          <p className="text-muted text-xs">ID: {queuedCommand.id}</p>
        </div>
      ) : (
        <MainEmptyState
          icon={<Play aria-hidden="true" className="size-6" />}
          message="Submit a command to queue it. Streaming output will appear after process endpoints are available."
          title="No command queued"
        />
      )}
    </div>
  );
}
