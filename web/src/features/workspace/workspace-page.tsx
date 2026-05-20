import * as Tabs from "@radix-ui/react-tabs";
import { useMutation, useQuery } from "@tanstack/react-query";
import {
  ArrowRight,
  Code2,
  Files,
  GitBranch,
  MonitorUp,
  Play,
} from "lucide-react";
import { useQueryState } from "nuqs";
import { type ReactNode, useState } from "react";
import { Link } from "react-router";

import { AppShell } from "@/app/app-shell";
import { useThemePreference } from "@/app/theme";
import {
  apiErrorMessage,
  createWorkspace,
  getGitStatus,
  getWorkspace,
  listFiles,
  listWorkspaces,
} from "@/shared/api";
import {
  Button,
  classNames,
  StarterScreen,
  StatusPill,
  Surface,
  ThemeSwitcher,
} from "@/shared/ui";
import { panelParser, workspaceIdParser } from "@/shared/url";

const panels = [
  {
    description: "Browse workspace files and directories.",
    icon: Files,
    label: "Files",
    value: "files",
  },
  {
    description: "Inspect working tree status and diffs.",
    icon: GitBranch,
    label: "Git",
    value: "git",
  },
  {
    description: "Queue approved commands and inspect output.",
    icon: Play,
    label: "Commands",
    value: "commands",
  },
  {
    description: "Expose local dev ports through the same-host proxy.",
    icon: MonitorUp,
    label: "Preview",
    value: "preview",
  },
] as const;

export function WorkspacePage() {
  const [workspaceId, setWorkspaceId] = useQueryState(
    "workspaceId",
    workspaceIdParser,
  );
  const [panel, setPanel] = useQueryState("panel", panelParser);
  const [rootPath, setRootPath] = useState("");
  const { preference, setPreference } = useThemePreference();

  const activePanel = panels.find((item) => item.value === panel) ?? panels[0];

  const workspaceQuery = useQuery({
    enabled: workspaceId.length > 0,
    queryFn: () => getWorkspace(workspaceId),
    queryKey: ["workspace", workspaceId],
  });

  const workspacesQuery = useQuery({
    enabled: workspaceId.length === 0,
    queryFn: listWorkspaces,
    queryKey: ["workspaces"],
  });

  const createWorkspaceMutation = useMutation({
    mutationFn: createWorkspace,
    onSuccess: (workspace) => {
      void setWorkspaceId(workspace.id);
    },
  });

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

  const workspace = workspaceQuery.data;

  if (workspaceId.length === 0) {
    return (
      <StarterScreen
        createError={
          createWorkspaceMutation.error
            ? apiErrorMessage(createWorkspaceMutation.error)
            : undefined
        }
        isCreating={createWorkspaceMutation.isPending}
        isLoadingRecent={workspacesQuery.isPending}
        onRootPathChange={setRootPath}
        onSelectWorkspace={(selectedWorkspaceId) => {
          void setWorkspaceId(selectedWorkspaceId);
        }}
        onSubmit={() => createWorkspaceMutation.mutate(rootPath)}
        recentError={
          workspacesQuery.error
            ? apiErrorMessage(workspacesQuery.error)
            : undefined
        }
        recentWorkspaces={workspacesQuery.data?.workspaces ?? []}
        rootPath={rootPath}
        themeControl={
          <ThemeSwitcher onChange={setPreference} value={preference} />
        }
      />
    );
  }

  return (
    <AppShell mode="workspace" workspace={workspace} workspaceId={workspaceId}>
      <Tabs.Root
        className="grid w-full gap-3 px-3 py-3 sm:px-4 lg:grid-cols-[4.25rem_15rem_minmax(0,1fr)]"
        onValueChange={(value) => {
          void setPanel(value as typeof panel);
        }}
        value={panel}
      >
        <Tabs.List className="border-line bg-panel grid grid-cols-4 gap-1 rounded-lg border p-1 lg:grid-cols-1 lg:content-start">
          {panels.map((item) => (
            <Tabs.Trigger
              className="text-muted hover:bg-hover hover:text-ink data-[state=active]:bg-accent-soft data-[state=active]:text-accent grid min-h-12 min-w-0 place-items-center gap-1 rounded-md px-2 py-1.5 text-xs font-medium transition"
              key={item.value}
              value={item.value}
            >
              <item.icon aria-hidden="true" className="size-5" />
              <span className="max-w-full truncate">{item.label}</span>
            </Tabs.Trigger>
          ))}
        </Tabs.List>

        <Surface as="aside" className="content-start gap-3" layout="grid">
          <div className="grid gap-2">
            <p className="text-muted text-xs font-semibold uppercase">
              Explorer
            </p>
            <div className="flex min-w-0 items-center gap-2">
              <h2 className="text-ink truncate text-base font-semibold">
                {activePanel.label}
              </h2>
              {workspace ? <StatusPill status={workspace.status} /> : null}
            </div>
            <p className="text-muted text-sm leading-5">
              {activePanel.description}
            </p>
          </div>

          {workspace ? (
            <div className="border-line grid gap-1 border-t pt-2">
              <p className="text-muted text-xs font-semibold uppercase">
                Workspace
              </p>
              <p className="text-ink truncate text-sm font-medium">
                {workspace.name}
              </p>
              <p className="text-muted text-xs leading-5 break-all">
                {workspace.rootPath}
              </p>
            </div>
          ) : (
            <p className="text-warning text-sm font-medium">
              {workspaceQuery.error
                ? apiErrorMessage(workspaceQuery.error)
                : "Workspace is loading."}
            </p>
          )}

          <Button asChild variant="secondary">
            <Link to={`/vibe?workspaceId=${encodeURIComponent(workspaceId)}`}>
              <ArrowRight aria-hidden="true" className="size-4" />
              Vibe Mode
            </Link>
          </Button>
        </Surface>

        <div className="min-w-0">
          <Tabs.Content value="files">
            <Panel title="Files">
              {filesQuery.error ? (
                <ErrorState message={apiErrorMessage(filesQuery.error)} />
              ) : (
                <ul className="grid gap-2">
                  {(filesQuery.data?.entries ?? []).map((entry) => (
                    <li
                      className="border-line flex min-h-9 min-w-0 items-center gap-2 rounded-md border px-2"
                      key={entry.path}
                    >
                      <Code2
                        aria-hidden="true"
                        className={classNames(
                          "size-4 shrink-0",
                          entry.isDir ? "text-accent" : "text-muted",
                        )}
                      />
                      <span className="text-ink min-w-0 truncate text-sm">
                        {entry.path}
                      </span>
                    </li>
                  ))}
                  {filesQuery.isPending ? (
                    <li className="text-muted text-sm">Loading files...</li>
                  ) : null}
                  {filesQuery.data?.entries.length === 0 ? (
                    <li className="text-muted text-sm">No files found.</li>
                  ) : null}
                </ul>
              )}
            </Panel>
          </Tabs.Content>

          <Tabs.Content value="git">
            <Panel title="Git status">
              {gitQuery.error ? (
                <ErrorState message={apiErrorMessage(gitQuery.error)} />
              ) : (
                <pre className="border-line bg-canvas text-ink min-h-48 overflow-auto rounded-md border p-3 text-sm">
                  {gitQuery.data?.porcelain ||
                    "Working tree status will appear here."}
                </pre>
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
        </div>
      </Tabs.Root>
    </AppShell>
  );
}

function Panel({ children, title }: { children: ReactNode; title: string }) {
  return (
    <Surface>
      <h2 className="text-ink mb-2 text-base font-semibold">{title}</h2>
      {children}
    </Surface>
  );
}

function EmptyState({ message }: { message: string }) {
  return <p className="text-muted text-sm leading-5">{message}</p>;
}

function ErrorState({ message }: { message: string }) {
  return <p className="text-warning text-sm font-medium">{message}</p>;
}
