import * as Tabs from "@radix-ui/react-tabs";
import { useQuery } from "@tanstack/react-query";
import { Code2, Files, GitBranch, MonitorUp, Play } from "lucide-react";
import { useQueryState } from "nuqs";
import type { ReactNode } from "react";
import { Link } from "react-router";

import { getGitStatus, listFiles } from "../../shared/api";
import { Button, classNames, Section } from "../../shared/ui";
import { panelParser, workspaceIdParser } from "../../shared/url";

const panels = [
  { icon: Files, label: "Files", value: "files" },
  { icon: GitBranch, label: "Git", value: "git" },
  { icon: Play, label: "Commands", value: "commands" },
  { icon: MonitorUp, label: "Preview", value: "preview" },
] as const;

export function WorkspacePage() {
  const [workspaceId] = useQueryState("workspaceId", workspaceIdParser);
  const [panel, setPanel] = useQueryState("panel", panelParser);

  const filesQuery = useQuery({
    enabled: workspaceId.length > 0 && panel === "files",
    queryFn: () => listFiles(workspaceId),
    queryKey: ["workspace-files", workspaceId],
  });

  const gitQuery = useQuery({
    enabled: workspaceId.length > 0 && panel === "git",
    queryFn: () => getGitStatus(workspaceId),
    queryKey: ["workspace-git-status", workspaceId],
  });

  return (
    <main className="bg-canvas min-h-screen">
      <Section eyebrow="Workspace Mode" title="Lightweight repo support">
        <div className="flex flex-wrap gap-2">
          <Button asChild variant="secondary">
            <Link
              to={`/vibe${workspaceId ? `?workspaceId=${encodeURIComponent(workspaceId)}` : ""}`}
            >
              Vibe Mode
            </Link>
          </Button>
        </div>
      </Section>

      <section className="mx-auto grid w-full max-w-6xl gap-4 px-4 py-5 sm:px-6">
        <Tabs.Root
          className="grid gap-4"
          onValueChange={(value) => {
            void setPanel(value as typeof panel);
          }}
          value={panel}
        >
          <Tabs.List className="grid grid-cols-2 gap-2 sm:grid-cols-4">
            {panels.map((item) => (
              <Tabs.Trigger
                className="border-line bg-panel text-ink data-[state=active]:border-accent data-[state=active]:text-accent inline-flex min-h-11 items-center justify-center gap-2 rounded-md border px-3 py-2 text-sm font-medium"
                key={item.value}
                value={item.value}
              >
                <item.icon className="size-4" />
                {item.label}
              </Tabs.Trigger>
            ))}
          </Tabs.List>

          <Tabs.Content value="files">
            <Panel title="Files">
              {workspaceId ? (
                <ul className="grid gap-2">
                  {(filesQuery.data?.entries ?? []).map((entry) => (
                    <li
                      className="border-line flex min-h-10 items-center gap-2 rounded-md border px-3"
                      key={entry.path}
                    >
                      <Code2
                        className={classNames(
                          "size-4",
                          entry.isDir ? "text-accent" : "text-muted",
                        )}
                      />
                      <span className="text-sm break-all">{entry.path}</span>
                    </li>
                  ))}
                  {filesQuery.isPending ? (
                    <li className="text-muted text-sm">Loading files...</li>
                  ) : null}
                  {filesQuery.data?.entries.length === 0 ? (
                    <li className="text-muted text-sm">No files found.</li>
                  ) : null}
                </ul>
              ) : (
                <EmptyState message="Open a workspace from Vibe Mode to browse files." />
              )}
            </Panel>
          </Tabs.Content>

          <Tabs.Content value="git">
            <Panel title="Git status">
              {workspaceId ? (
                <pre className="border-line bg-canvas text-ink min-h-32 overflow-auto rounded-md border p-3 text-sm">
                  {gitQuery.data?.porcelain ||
                    "Working tree status will appear here."}
                </pre>
              ) : (
                <EmptyState message="Open a workspace to inspect Git status and diffs." />
              )}
            </Panel>
          </Tabs.Content>

          <Tabs.Content value="commands">
            <Panel title="Commands">
              <EmptyState message="Command queueing exists in the API. Streaming output awaits process endpoints." />
            </Panel>
          </Tabs.Content>

          <Tabs.Content value="preview">
            <Panel title="Preview">
              <EmptyState message="Same-host preview controls will appear after port APIs are implemented." />
            </Panel>
          </Tabs.Content>
        </Tabs.Root>
      </section>
    </main>
  );
}

function Panel({ children, title }: { children: ReactNode; title: string }) {
  return (
    <div className="border-line bg-panel rounded-lg border p-4">
      <h2 className="text-ink mb-3 text-lg font-semibold">{title}</h2>
      {children}
    </div>
  );
}

function EmptyState({ message }: { message: string }) {
  return <p className="text-muted text-sm leading-6">{message}</p>;
}
