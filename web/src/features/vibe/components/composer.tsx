import { ArrowUp, ChevronDown, Loader2, ShieldCheck } from "lucide-react";
import { type FormEvent } from "react";

import type { AgentModel, AgentReasoningEffort } from "@/shared/api";
import { Button } from "@/shared/ui";

import { agentModels, reasoningEfforts } from "../vibe-options";
import { SelectControl } from "./select-control";

export function Composer({
  error,
  isPending,
  model,
  onModelChange,
  onPromptChange,
  onReasoningEffortChange,
  onSubmit,
  prompt,
  reasoningEffort,
  workspaceReady,
}: {
  error?: string;
  isPending: boolean;
  model: AgentModel;
  onModelChange: (model: AgentModel) => void;
  onPromptChange: (prompt: string) => void;
  onReasoningEffortChange: (effort: AgentReasoningEffort) => void;
  onSubmit: (event: FormEvent<HTMLFormElement>) => void;
  prompt: string;
  reasoningEffort: AgentReasoningEffort;
  workspaceReady: boolean;
}) {
  return (
    <form className="grid gap-2" onSubmit={onSubmit}>
      <div className="vibe-composer border-line/40 bg-composer focus-within:border-line/40 grid overflow-hidden rounded-xl border shadow-md focus-within:outline-none">
        <label className="sr-only" htmlFor="agent-prompt">
          Ask AI
        </label>
        <textarea
          className="text-ink placeholder:text-muted min-h-20 resize-none border-0 bg-transparent px-4 py-3 text-sm leading-6 transition outline-none focus:border-0 focus:ring-0 focus:outline-none focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-60"
          disabled={!workspaceReady}
          id="agent-prompt"
          onChange={(event) => onPromptChange(event.target.value)}
          placeholder="Ask for follow-up changes"
          value={prompt}
        />
        <div className="flex min-w-0 flex-col gap-2 px-3 py-3 sm:flex-row sm:items-center sm:justify-between">
          <div className="flex min-w-0 flex-wrap items-center gap-2">
            <span className="hover:bg-hover text-muted inline-flex min-h-9 min-w-0 cursor-pointer items-center gap-2 rounded-md px-2 text-xs font-medium">
              <ShieldCheck aria-hidden="true" className="size-4" />
              Default permissions
              <ChevronDown aria-hidden="true" className="size-4" />
            </span>
          </div>
          <div className="flex min-w-0 items-center justify-between gap-2 sm:justify-end">
            <SelectControl
              label="Model"
              onChange={(value) => onModelChange(value as AgentModel)}
              options={agentModels}
              value={model}
            />
            <SelectControl
              label="Reasoning"
              onChange={(value) =>
                onReasoningEffortChange(value as AgentReasoningEffort)
              }
              options={reasoningEfforts}
              value={reasoningEffort}
            />
            <Button
              aria-label="Start run"
              disabled={
                !workspaceReady || prompt.trim().length === 0 || isPending
              }
              icon={
                isPending ? (
                  <Loader2 className="size-4! animate-spin" />
                ) : (
                  <ArrowUp className="size-4!" />
                )
              }
              size="icon"
              type="submit"
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
