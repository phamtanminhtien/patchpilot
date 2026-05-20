import { useMutation, useQuery } from "@tanstack/react-query";
import {
  ChevronDown,
  FolderOpen,
  MessageSquare,
  Plus,
  Search,
  Send,
  ShieldCheck,
} from "lucide-react";
import { useQueryState } from "nuqs";
import { useState } from "react";
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
      <section className="grid min-h-[calc(100vh-2.5rem)] w-full overflow-hidden lg:grid-cols-[18rem_minmax(0,1fr)]">
        <aside className="border-line bg-panel hidden min-h-0 border-r px-3 py-3 lg:grid lg:grid-rows-[auto_minmax(0,1fr)_auto]">
          <div className="grid gap-1.5">
            <button
              className="text-ink flex min-h-9 min-w-0 items-center gap-2 rounded-md px-2 text-left text-sm font-medium transition disabled:cursor-not-allowed disabled:opacity-55"
              disabled
              type="button"
            >
              <Plus aria-hidden="true" className="size-4 shrink-0" />
              <span className="truncate">New chat</span>
            </button>
            <button
              className="text-muted flex min-h-9 min-w-0 items-center gap-2 rounded-md px-2 text-left text-sm font-medium transition disabled:cursor-not-allowed disabled:opacity-55"
              disabled
              type="button"
            >
              <Search aria-hidden="true" className="size-4 shrink-0" />
              <span className="truncate">Search</span>
            </button>
          </div>

          <div className="min-h-0 overflow-auto py-5">
            <div className="grid gap-5">
              <ConversationGroup
                items={[
                  "Ask PatchPilot anything",
                  "Review next patch proposal",
                  "Run checks after approval",
                ]}
                title="Pinned"
              />
              <ConversationGroup
                items={[
                  workspace?.name ?? "Workspace loading",
                  "Agent task console",
                  "Open workspace tools",
                ]}
                title="PatchPilot"
              />
            </div>
          </div>

          <div className="grid gap-2">
            {workspace ? (
              <Button asChild size="compact" variant="ghost" width="full">
                <Link
                  to={`/workspace?workspaceId=${encodeURIComponent(workspace.id)}`}
                >
                  <FolderOpen aria-hidden="true" className="size-4" />
                  Open workspace
                </Link>
              </Button>
            ) : null}
          </div>
        </aside>

        <div className="grid min-w-0 place-items-center px-3 py-6 sm:px-4">
          <div className="grid w-full max-w-4xl gap-5">
            <div className="grid justify-items-center gap-3 text-center">
              <h1 className="text-ink text-2xl font-semibold text-balance sm:text-3xl">
                What should we build in PatchPilot?
              </h1>
            </div>

            <Surface
              className="bg-panel/95 gap-0 overflow-hidden shadow-md"
              layout="grid"
              padding="none"
            >
              <label className="sr-only" htmlFor="agent-prompt">
                Ask AI
              </label>
              <textarea
                className="text-ink placeholder:text-muted min-h-24 resize-none bg-transparent px-4 py-4 text-sm leading-6 transition"
                disabled
                id="agent-prompt"
                placeholder="Ask PatchPilot anything. Agent task creation will appear here when the endpoint is available."
              />
              <div className="border-line flex min-w-0 flex-col gap-2 border-t px-3 py-3 sm:flex-row sm:items-center sm:justify-between">
                <div className="flex min-w-0 flex-wrap items-center gap-2">
                  <span className="border-line text-muted inline-flex min-h-10 min-w-0 items-center gap-2 rounded-md border px-3 text-xs font-medium">
                    <ShieldCheck aria-hidden="true" className="size-4" />
                    Default permissions
                    <ChevronDown aria-hidden="true" className="size-4" />
                  </span>
                  {workspace ? (
                    <Button asChild size="compact" variant="secondary">
                      <Link
                        to={`/workspace?workspaceId=${encodeURIComponent(workspace.id)}`}
                      >
                        <FolderOpen aria-hidden="true" className="size-4" />
                        {workspace.name}
                      </Link>
                    </Button>
                  ) : (
                    <span className="text-warning text-sm font-medium">
                      {error ? apiErrorMessage(error) : "Workspace is loading."}
                    </span>
                  )}
                </div>

                <div className="flex min-w-0 items-center justify-between gap-2 sm:justify-end">
                  {workspace ? <StatusPill status={workspace.status} /> : null}
                  <Button disabled icon={<Send />} size="compact">
                    Start task
                  </Button>
                </div>
              </div>
            </Surface>

            {workspace ? (
              <p className="text-muted mx-auto max-w-full truncate text-center text-xs">
                {workspace.rootPath}
              </p>
            ) : null}
          </div>
        </div>
      </section>
    </AppShell>
  );
}

function ConversationGroup({
  items,
  title,
}: {
  items: string[];
  title: string;
}) {
  return (
    <section className="grid gap-1">
      <h2 className="text-muted px-2 text-xs font-semibold">{title}</h2>
      <div className="grid gap-0.5">
        {items.map((item, index) => (
          <div
            aria-current={index === 0 ? "page" : undefined}
            className="text-muted aria-[current=page]:bg-hover aria-[current=page]:text-ink flex min-h-9 min-w-0 items-center gap-2 rounded-md px-2 text-sm"
            key={item}
          >
            <MessageSquare aria-hidden="true" className="size-4 shrink-0" />
            <span className="truncate">{item}</span>
          </div>
        ))}
      </div>
    </section>
  );
}
