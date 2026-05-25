import { BookOpen, Puzzle, RefreshCw } from "lucide-react";
import { useState } from "react";

import type { AgentContextSnapshot, AgentSkill } from "@/shared/api";
import {
  Button,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogRoot,
  DialogTitle,
  Markdown,
  Switch,
} from "@/shared/ui";

export function SkillsDialog({
  context,
  error,
  isLoading,
  isOpen,
  isRefreshing,
  isUpdatingSkill,
  onOpenChange,
  onRefresh,
  onSkillEnabledChange,
}: {
  context?: AgentContextSnapshot;
  error?: string;
  isLoading: boolean;
  isOpen: boolean;
  isRefreshing: boolean;
  isUpdatingSkill: boolean;
  onOpenChange: (open: boolean) => void;
  onRefresh: () => void;
  onSkillEnabledChange: (skill: AgentSkill, enabled: boolean) => void;
}) {
  const skills = context?.skills ?? [];
  const [selectedSkillKey, setSelectedSkillKey] = useState("");
  const selectedSkill =
    skills.find((skill) => skill.key === selectedSkillKey) ?? skills[0];

  return (
    <DialogRoot onOpenChange={onOpenChange} open={isOpen}>
      <DialogContent className="grid-rows-[auto_minmax(0,1fr)_auto] gap-0 overflow-hidden p-0 sm:max-h-[min(52rem,calc(100vh-2rem))] sm:max-w-6xl">
        <DialogHeader className="border-line/30 border-b px-5 py-4">
          <DialogTitle>Skills</DialogTitle>
          <DialogDescription>
            {context
              ? `Refreshed ${new Date(context.refreshedAt).toLocaleTimeString()}`
              : "Not loaded"}
          </DialogDescription>
        </DialogHeader>

        <div className="grid min-h-0 overflow-hidden px-5 py-4">
          <div className="grid min-h-0 min-w-0 grid-rows-[auto_auto_minmax(0,1fr)] gap-4">
            {error ? (
              <p className="text-warning text-sm font-medium">{error}</p>
            ) : null}
            {isLoading ? (
              <p className="text-muted text-sm">Loading skills</p>
            ) : null}

            <section className="grid min-h-0 min-w-0 overflow-hidden">
              {skills.length ? (
                <div className="grid h-full min-h-0 min-w-0 gap-4 overflow-hidden lg:grid-cols-[minmax(18rem,0.78fr)_minmax(0,1.22fr)]">
                  <div className="grid max-h-72 min-h-0 min-w-0 content-start gap-2 overflow-auto pr-1 lg:max-h-none">
                    {skills.map((skill) => (
                      <SkillRow
                        isSelected={skill.key === selectedSkill?.key}
                        isUpdating={isUpdatingSkill}
                        key={skill.key}
                        onSelect={() => setSelectedSkillKey(skill.key)}
                        onSkillEnabledChange={onSkillEnabledChange}
                        skill={skill}
                      />
                    ))}
                  </div>
                  {selectedSkill ? <SkillDetail skill={selectedSkill} /> : null}
                </div>
              ) : (
                <p className="text-muted text-sm">
                  No local skills discovered.
                </p>
              )}
            </section>
          </div>
        </div>

        <DialogFooter className="border-line/30 border-t px-5 py-3">
          <Button
            disabled={isRefreshing}
            icon={<RefreshCw className={isRefreshing ? "animate-spin" : ""} />}
            onClick={onRefresh}
            size="small"
            type="button"
            variant="surface"
          >
            Refresh
          </Button>
        </DialogFooter>
      </DialogContent>
    </DialogRoot>
  );
}

function SkillRow({
  isSelected,
  isUpdating,
  onSelect,
  onSkillEnabledChange,
  skill,
}: {
  isSelected: boolean;
  isUpdating: boolean;
  onSelect: () => void;
  onSkillEnabledChange: (skill: AgentSkill, enabled: boolean) => void;
  skill: AgentSkill;
}) {
  return (
    <div
      className="hover:bg-hover data-[selected=true]:bg-hover flex min-h-16 min-w-0 items-center gap-3 rounded-md px-2 py-2 transition"
      data-selected={isSelected}
    >
      <button
        aria-current={isSelected ? "true" : undefined}
        className="flex min-w-0 flex-1 cursor-pointer items-center gap-3 text-left"
        onClick={onSelect}
        type="button"
      >
        <span className="bg-accent-soft text-accent grid size-9 shrink-0 place-items-center rounded-md">
          <Puzzle aria-hidden="true" className="size-4" />
        </span>
        <div className="grid min-w-0 flex-1 gap-1">
          <div className="flex min-w-0 items-center gap-2">
            <span className="text-ink truncate text-sm font-medium">
              {skill.name}
            </span>
            {!skill.valid ? (
              <span className="text-warning shrink-0 text-xs">Invalid</span>
            ) : null}
          </div>
          <p className="text-muted line-clamp-2 text-xs">
            {skill.description || skill.source}
          </p>
          {skill.warning ? (
            <p className="text-warning line-clamp-2 text-xs">{skill.warning}</p>
          ) : null}
        </div>
      </button>
      <Switch
        aria-label={`Toggle ${skill.name}`}
        checked={skill.enabled}
        disabled={isUpdating}
        onCheckedChange={(checked) => onSkillEnabledChange(skill, checked)}
      />
    </div>
  );
}

function SkillDetail({ skill }: { skill: AgentSkill }) {
  return (
    <section className="bg-hover grid min-h-0 min-w-0 grid-rows-[auto_minmax(0,1fr)] gap-3 overflow-hidden rounded-md p-3">
      <div className="flex min-w-0 items-start gap-3">
        <span className="bg-accent-soft text-accent grid size-9 shrink-0 place-items-center rounded-md">
          <BookOpen aria-hidden="true" className="size-4" />
        </span>
        <div className="grid min-w-0 gap-1">
          <h3 className="text-ink truncate text-sm font-semibold">
            {skill.name}
          </h3>
          <p className="text-muted text-xs">{skill.description}</p>
          <p className="text-muted text-xs">Source: {skill.source}</p>
          {!skill.enabled ? (
            <p className="text-muted text-xs">Disabled</p>
          ) : null}
          {!skill.valid ? (
            <p className="text-warning text-xs">
              {skill.warning ?? "Invalid skill"}
            </p>
          ) : null}
        </div>
      </div>
      <div className="bg-panel min-h-0 overflow-auto rounded-sm p-3">
        <Markdown className="text-xs">
          {skill.instruction?.trim() || "No skill body available."}
        </Markdown>
      </div>
    </section>
  );
}
