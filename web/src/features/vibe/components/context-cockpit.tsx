import type { ReactNode } from "react";

import type {
  AgentContextSnapshot,
  AgentContextWarning,
  AgentSkill,
} from "@/shared/api";
import { Switch } from "@/shared/ui";

import { skillDisplayName } from "../lib/skills";

export function ContextCockpit({
  context,
  error,
  isLoading,
  isUpdatingSkill,
  onSkillEnabledChange,
}: {
  context?: AgentContextSnapshot;
  error?: string;
  isLoading: boolean;
  isUpdatingSkill: boolean;
  onSkillEnabledChange: (skill: AgentSkill, enabled: boolean) => void;
}) {
  const instructionSources = context?.instructionSources ?? [];
  const skills = context?.skills ?? [];
  const mcpServers = context?.mcpServers ?? [];
  const warnings = [
    ...(context?.skippedSources ?? []),
    ...(context?.contextWarnings ?? []),
  ];

  return (
    <div className="grid min-w-0 gap-4">
      {error ? (
        <p className="text-warning text-sm font-medium">{error}</p>
      ) : null}
      {isLoading ? <p className="text-muted text-sm">Loading context</p> : null}

      <CockpitSection title="Instructions">
        {instructionSources.length ? (
          instructionSources.map((source) => (
            <div className="grid min-w-0 gap-2 pb-3" key={source.path}>
              <div className="text-ink flex min-w-0 items-center gap-2 text-xs font-medium">
                <span className="bg-accent-soft text-accent rounded-xl px-1.5 py-0.5">
                  {source.precedence + 1}
                </span>
                <span className="truncate">{source.path}</span>
              </div>
              <pre className="bg-surface text-muted max-h-28 overflow-auto rounded-xl p-2 text-xs whitespace-pre-wrap">
                {source.content}
              </pre>
            </div>
          ))
        ) : (
          <EmptyLine text="No AGENTS.md sources found." />
        )}
      </CockpitSection>

      <CockpitSection title="Skills">
        {skills.length ? (
          skills.map((skill) => {
            const displayName = skillDisplayName(skill);

            return (
              <div
                className="flex min-w-0 items-start justify-between gap-3 rounded-xl px-1 py-2"
                key={skill.key}
              >
                <div className="grid min-w-0 gap-1">
                  <div className="flex min-w-0 items-center gap-2">
                    <span className="text-ink truncate text-sm font-medium">
                      {displayName}
                    </span>
                    {!skill.valid ? (
                      <span className="text-warning shrink-0 text-xs">
                        Invalid
                      </span>
                    ) : null}
                  </div>
                  <p className="text-muted truncate text-xs">
                    {skill.description || skill.source}
                  </p>
                  {skill.warning ? (
                    <p className="text-warning text-xs">{skill.warning}</p>
                  ) : null}
                </div>
                <Switch
                  aria-label={`Toggle ${displayName}`}
                  checked={skill.enabled}
                  disabled={isUpdatingSkill}
                  onCheckedChange={(checked) =>
                    onSkillEnabledChange(skill, checked)
                  }
                />
              </div>
            );
          })
        ) : (
          <EmptyLine text="No local skills discovered." />
        )}
      </CockpitSection>

      <CockpitSection title="MCP">
        {mcpServers.length ? (
          mcpServers.map((server) => (
            <div
              className="grid min-w-0 gap-1 rounded-xl px-1 py-2"
              key={server.id}
            >
              <div className="flex min-w-0 items-center gap-2">
                <span className="text-ink truncate text-sm font-medium">
                  {server.name}
                </span>
                <span className="text-muted shrink-0 text-xs">
                  {server.transport}
                </span>
                <span className="text-muted shrink-0 text-xs">
                  {server.status}
                </span>
              </div>
              <p className="text-muted truncate text-xs">
                Approval: {server.approvalPolicy}
              </p>
              {server.lastError ? (
                <p className="text-warning text-xs">{server.lastError}</p>
              ) : null}
              <Warnings warnings={server.warnings ?? []} />
            </div>
          ))
        ) : (
          <EmptyLine text="No MCP servers configured." />
        )}
      </CockpitSection>

      {warnings.length ? (
        <CockpitSection title="Warnings">
          <Warnings warnings={warnings} />
        </CockpitSection>
      ) : null}
    </div>
  );
}

function CockpitSection({
  children,
  title,
}: {
  children: ReactNode;
  title: string;
}) {
  return (
    <section className="grid min-w-0 gap-2">
      <h3 className="text-muted text-xs font-semibold tracking-wide uppercase">
        {title}
      </h3>
      <div className="grid min-w-0 gap-3">{children}</div>
    </section>
  );
}

function EmptyLine({ text }: { text: string }) {
  return <p className="text-muted text-sm">{text}</p>;
}

function Warnings({ warnings }: { warnings: AgentContextWarning[] }) {
  return (
    <div className="grid min-w-0 gap-1">
      {warnings.map((warning, index) => (
        <p
          className="text-warning min-w-0 text-xs"
          key={`${warning.path ?? "warning"}-${index}`}
        >
          {warning.path ? `${warning.path}: ` : ""}
          {warning.message}
        </p>
      ))}
    </div>
  );
}
