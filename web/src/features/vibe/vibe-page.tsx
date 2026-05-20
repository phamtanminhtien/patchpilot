import { useMutation, useQuery } from "@tanstack/react-query";
import {
  ArrowRight,
  Bot,
  CheckCircle2,
  GitPullRequestArrow,
  Loader2,
  Play,
  Send,
} from "lucide-react";
import { useQueryState } from "nuqs";
import { type ReactNode, useState } from "react";
import { Link } from "react-router";

import { AppShell } from "@/app/app-shell";
import { useThemePreference } from "@/app/theme";
import {
  apiErrorMessage,
  createWorkspace,
  getWorkspace,
  listWorkspaces,
} from "@/shared/api";
import {
  Button,
  StarterScreen,
  StatusPill,
  Surface,
  ThemeSwitcher,
} from "@/shared/ui";
import { workspaceIdParser } from "@/shared/url";

export function VibePage() {
  const [workspaceId, setWorkspaceId] = useQueryState(
    "workspaceId",
    workspaceIdParser,
  );
  const [rootPath, setRootPath] = useState("");
  const { preference, setPreference } = useThemePreference();

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

  const workspace = workspaceQuery.data;
  const error = createWorkspaceMutation.error ?? workspaceQuery.error;

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
    <AppShell mode="vibe" workspace={workspace} workspaceId={workspaceId}>
      <section className="mx-auto grid w-full max-w-6xl gap-4 px-4 py-5 sm:px-6 lg:grid-cols-[minmax(0,1fr)_22rem]">
        <div className="grid gap-4">
          <Surface className="content-start gap-4" layout="grid">
            <div className="flex items-start gap-3">
              <span className="bg-accent-soft text-accent grid size-10 shrink-0 place-items-center rounded-md">
                <Bot aria-hidden="true" className="size-5" />
              </span>
              <div className="min-w-0">
                <p className="text-muted text-xs font-semibold uppercase">
                  Vibe Mode
                </p>
                <h2 className="text-ink text-xl font-semibold">
                  Agent task console
                </h2>
              </div>
            </div>

            <div className="border-line bg-canvas grid gap-3 rounded-md border p-3">
              <label
                className="text-ink grid gap-2 text-sm font-medium"
                htmlFor="agent-prompt"
              >
                Ask AI
                <textarea
                  className="border-line bg-panel text-ink placeholder:text-muted focus:border-accent min-h-28 resize-y rounded-md border px-3 py-2 text-base transition"
                  disabled
                  id="agent-prompt"
                  placeholder="Agent task creation will appear here when the endpoint is available."
                />
              </label>
              <div className="flex flex-wrap items-center gap-2">
                <Button disabled icon={<Send />}>
                  Start task
                </Button>
                <p className="text-muted text-sm">
                  Waiting for agent task APIs.
                </p>
              </div>
            </div>
          </Surface>

          <Surface className="content-start gap-4" layout="grid">
            <PanelTitle
              icon={<Play aria-hidden="true" className="size-5" />}
              kicker="Activity"
              title="Agent timeline"
            />
            <ol className="grid gap-3">
              {[
                "Read approved files",
                "Stream tool activity",
                "Create patch proposal",
              ].map((item) => (
                <li className="flex min-w-0 items-center gap-3" key={item}>
                  <span className="border-line bg-panel grid size-8 shrink-0 place-items-center rounded-md border">
                    <Loader2 aria-hidden="true" className="text-muted size-4" />
                  </span>
                  <span className="text-muted text-sm">{item}</span>
                </li>
              ))}
            </ol>
          </Surface>

          <Surface className="content-start gap-4" layout="grid">
            <PanelTitle
              icon={
                <GitPullRequestArrow aria-hidden="true" className="size-5" />
              }
              kicker="Review"
              title="Patch proposal"
            />
            <div className="border-line grid gap-3 rounded-md border p-3">
              <div className="flex min-w-0 items-start gap-3">
                <CheckCircle2
                  aria-hidden="true"
                  className="text-muted mt-0.5 size-5 shrink-0"
                />
                <p className="text-muted text-sm leading-6">
                  Patch summaries and mobile diff approval controls will appear
                  here once patch APIs exist.
                </p>
              </div>
              <div className="flex flex-wrap gap-2">
                <Button disabled>Apply patch</Button>
                <Button disabled variant="secondary">
                  Reject patch
                </Button>
              </div>
            </div>
          </Surface>
        </div>

        <Surface as="aside" className="content-start gap-4" layout="grid">
          <PanelTitle
            icon={<ArrowRight aria-hidden="true" className="size-5" />}
            kicker="Context"
            title="Workspace"
          />
          {workspace ? (
            <div className="grid gap-4">
              <div className="grid gap-1">
                <div className="flex min-w-0 items-center gap-2">
                  <h2 className="text-ink truncate text-lg font-semibold">
                    {workspace.name}
                  </h2>
                  <StatusPill status={workspace.status} />
                </div>
                <p className="text-muted text-sm leading-6 break-all">
                  {workspace.rootPath}
                </p>
              </div>
              <Button asChild variant="secondary">
                <Link
                  to={`/workspace?workspaceId=${encodeURIComponent(workspace.id)}`}
                >
                  <ArrowRight aria-hidden="true" className="size-4" />
                  Open workspace
                </Link>
              </Button>
            </div>
          ) : (
            <p className="text-warning text-sm font-medium">
              {error ? apiErrorMessage(error) : "Workspace is loading."}
            </p>
          )}
        </Surface>
      </section>
    </AppShell>
  );
}

function PanelTitle({
  icon,
  kicker,
  title,
}: {
  icon: ReactNode;
  kicker: string;
  title: string;
}) {
  return (
    <div className="flex min-w-0 items-center gap-3">
      <span className="bg-accent-soft text-accent grid size-10 shrink-0 place-items-center rounded-md">
        {icon}
      </span>
      <div className="min-w-0">
        <p className="text-muted text-xs font-semibold uppercase">{kicker}</p>
        <h2 className="text-ink truncate text-lg font-semibold">{title}</h2>
      </div>
    </div>
  );
}
