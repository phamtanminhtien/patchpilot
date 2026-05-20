import { useMutation, useQuery } from "@tanstack/react-query";
import { ArrowRight, Bot, FolderOpen, GitPullRequestArrow } from "lucide-react";
import { useQueryState } from "nuqs";
import { useState } from "react";
import { Link } from "react-router";

import {
  apiErrorMessage,
  createWorkspace,
  getWorkspace,
} from "../../shared/api";
import { Button, Section, TextField } from "../../shared/ui";
import { workspaceIdParser } from "../../shared/url";

export function VibePage() {
  const [workspaceId, setWorkspaceId] = useQueryState(
    "workspaceId",
    workspaceIdParser,
  );
  const [rootPath, setRootPath] = useState("");

  const workspaceQuery = useQuery({
    enabled: workspaceId.length > 0,
    queryFn: () => getWorkspace(workspaceId),
    queryKey: ["workspace", workspaceId],
  });

  const createWorkspaceMutation = useMutation({
    mutationFn: createWorkspace,
    onSuccess: (workspace) => {
      void setWorkspaceId(workspace.id);
    },
  });

  const workspace = workspaceQuery.data;
  const error = createWorkspaceMutation.error ?? workspaceQuery.error;

  return (
    <main className="bg-canvas min-h-screen">
      <Section eyebrow="Vibe Mode" title="Patch-first AI coding loop">
        <form
          className="grid gap-3 sm:grid-cols-[1fr_auto] sm:items-end"
          onSubmit={(event) => {
            event.preventDefault();
            createWorkspaceMutation.mutate(rootPath);
          }}
        >
          <TextField
            label="Workspace root"
            name="workspace-root"
            onChange={(event) => setRootPath(event.target.value)}
            placeholder="/absolute/path/to/repo"
            value={rootPath}
          />
          <Button
            disabled={
              createWorkspaceMutation.isPending || rootPath.trim().length === 0
            }
            icon={<FolderOpen />}
          >
            Open repo
          </Button>
        </form>
        {error ? (
          <p className="text-warning text-sm font-medium">
            {apiErrorMessage(error)}
          </p>
        ) : null}
      </Section>

      <section className="mx-auto grid w-full max-w-6xl gap-4 px-4 py-5 sm:px-6 lg:grid-cols-[minmax(0,1fr)_22rem]">
        <div className="grid gap-4">
          <div className="border-line bg-panel rounded-lg border p-4">
            <div className="flex items-start gap-3">
              <span className="bg-accent/10 text-accent grid size-10 shrink-0 place-items-center rounded-md">
                <Bot className="size-5" />
              </span>
              <div className="grid gap-1">
                <h2 className="text-ink text-lg font-semibold">Ask AI</h2>
                <p className="text-muted text-sm leading-6">
                  Agent task creation is waiting on backend endpoints. This area
                  is reserved for prompt entry, progress, and patch review.
                </p>
              </div>
            </div>
          </div>

          <div className="border-line bg-panel rounded-lg border p-4">
            <div className="flex items-start gap-3">
              <span className="bg-accent/10 text-accent grid size-10 shrink-0 place-items-center rounded-md">
                <GitPullRequestArrow className="size-5" />
              </span>
              <div className="grid gap-1">
                <h2 className="text-ink text-lg font-semibold">Patch review</h2>
                <p className="text-muted text-sm leading-6">
                  Proposed patch summaries and mobile diff approval controls
                  will appear here once patch APIs exist.
                </p>
              </div>
            </div>
          </div>
        </div>

        <aside className="border-line bg-panel grid content-start gap-3 rounded-lg border p-4">
          <p className="text-muted text-xs font-semibold uppercase">
            Current workspace
          </p>
          {workspace ? (
            <div className="grid gap-3">
              <div>
                <h2 className="text-ink text-xl font-semibold break-words">
                  {workspace.name}
                </h2>
                <p className="text-muted text-sm leading-6 break-all">
                  {workspace.rootPath}
                </p>
              </div>
              <p className="text-muted text-sm">Status: {workspace.status}</p>
              <Button asChild icon={<ArrowRight />} variant="secondary">
                <Link
                  to={`/workspace?workspaceId=${encodeURIComponent(workspace.id)}`}
                >
                  Open workspace
                </Link>
              </Button>
            </div>
          ) : (
            <p className="text-muted text-sm leading-6">
              Open a local Git repository to start the MVP flow.
            </p>
          )}
        </aside>
      </section>
    </main>
  );
}
