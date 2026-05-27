import {
  ArrowUp,
  ChevronDown,
  Loader2,
  ShieldCheck,
  Square,
} from "lucide-react";
import { type FormEvent } from "react";

import type { AgentModel, AgentReasoningEffort } from "@/shared/api";
import { Button, Select } from "@/shared/ui";

import { agentModels, reasoningEfforts } from "../vibe-options";

export function Composer({
  activeRun,
  error,
  isPending,
  isStopping,
  model,
  onModelChange,
  onPromptChange,
  onReasoningEffortChange,
  onStop,
  onSubmit,
  prompt,
  reasoningEffort,
  workspaceReady,
}: {
  activeRun: boolean;
  error?: string;
  isPending: boolean;
  isStopping: boolean;
  model: AgentModel;
  onModelChange: (model: AgentModel) => void;
  onPromptChange: (prompt: string) => void;
  onReasoningEffortChange: (effort: AgentReasoningEffort) => void;
  onStop: () => void;
  onSubmit: (event: FormEvent<HTMLFormElement>) => void;
  prompt: string;
  reasoningEffort: AgentReasoningEffort;
  workspaceReady: boolean;
}) {
  return (
    <form className="grid gap-2" onSubmit={onSubmit}>
      <div className="vibe-composer bg-composer grid overflow-hidden rounded-xl transition">
        <label className="sr-only" htmlFor="agent-prompt">
          Ask AI
        </label>
        <textarea
          className="text-ink placeholder:text-muted min-h-24 resize-none border-0 bg-transparent px-4 pt-4 pb-3 text-sm leading-6 transition outline-none focus:border-0 focus:ring-0 focus:outline-none focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-60"
          disabled={!workspaceReady}
          id="agent-prompt"
          onChange={(event) => onPromptChange(event.target.value)}
          placeholder="Ask for follow-up changes"
          value={prompt}
        />
        <div className="bg-composer-bar flex min-w-0 flex-col gap-2 px-3 py-2 sm:flex-row sm:items-center sm:justify-between">
          <div className="flex min-w-0 flex-wrap items-center gap-2">
            <span className="bg-surface hover:bg-hover text-muted inline-flex min-h-7 min-w-0 cursor-pointer items-center gap-1.5 rounded-xl px-2 text-xs font-medium transition">
              <ShieldCheck aria-hidden="true" className="size-3.5" />
              <span className="truncate">Default permissions</span>
              <ChevronDown aria-hidden="true" className="size-3.5" />
            </span>
          </div>
          <div className="grid min-w-0 grid-cols-[minmax(0,1fr)_minmax(0,1fr)_auto] items-center gap-2">
            <Select
              label="Model"
              onValueChange={(value) => onModelChange(value as AgentModel)}
              options={agentModels.map((option) => ({ value: option }))}
              size="tiny"
              triggerClassName="w-full"
              value={model}
            />
            <Select
              label="Reasoning"
              onValueChange={(value) =>
                onReasoningEffortChange(value as AgentReasoningEffort)
              }
              options={reasoningEfforts.map((option) => ({ value: option }))}
              size="tiny"
              triggerClassName="w-full"
              value={reasoningEffort}
            />
            <Button
              aria-label={activeRun ? "Stop run" : "Start run"}
              disabled={
                activeRun
                  ? isStopping
                  : !workspaceReady || prompt.trim().length === 0 || isPending
              }
              icon={
                isPending || isStopping ? (
                  <Loader2 className="size-4! animate-spin" />
                ) : activeRun ? (
                  <Square className="size-4!" />
                ) : (
                  <ArrowUp className="size-4!" />
                )
              }
              className="size-8 shrink-0 rounded-xl"
              onClick={activeRun ? onStop : undefined}
              size="icon"
              type={activeRun ? "button" : "submit"}
            />
          </div>
        </div>
      </div>
      {error ? (
        <p className="text-warning text-xs font-medium">{error}</p>
      ) : null}
    </form>
  );
}
