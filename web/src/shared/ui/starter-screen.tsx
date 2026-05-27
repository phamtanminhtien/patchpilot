import { ArrowRight, Braces, FolderOpen, History, Loader2 } from "lucide-react";
import type { FormEvent, ReactNode } from "react";

import { Button } from "./button";
import { cn } from "./class-name";
import { StatusPill } from "./status-pill";
import { Surface } from "./surface";
import { TextField } from "./text-field";

export interface StarterWorkspace {
  id: string;
  name: string;
  rootPath: string;
  status: "error" | "indexing" | "ready";
}

interface StarterScreenProps {
  createError?: string;
  isCreating?: boolean;
  isLoadingRecent?: boolean;
  onRootPathChange: (value: string) => void;
  onSelectWorkspace: (workspaceId: string) => void;
  onSubmit: () => void;
  recentError?: string;
  recentWorkspaces: StarterWorkspace[];
  rootPath: string;
  themeControl?: ReactNode;
}

export function StarterScreen({
  createError,
  isCreating = false,
  isLoadingRecent = false,
  onRootPathChange,
  onSelectWorkspace,
  onSubmit,
  recentError,
  recentWorkspaces,
  rootPath,
  themeControl,
}: StarterScreenProps) {
  function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    onSubmit();
  }

  return (
    <main className="pp-pattern-bg min-h-screen px-4 py-6 sm:px-6">
      <section className="mx-auto flex min-h-[calc(100vh-3rem)] w-full max-w-3xl flex-col justify-center">
        {themeControl ? (
          <div className="mb-4 flex justify-end">{themeControl}</div>
        ) : null}

        <Surface className="content-start gap-4" layout="grid">
          <div className="flex min-w-0 items-center justify-between gap-3 pb-2">
            <div className="flex min-w-0 items-center gap-3">
              <span className="bg-surface text-accent grid size-9 shrink-0 place-items-center rounded-xl">
                <Braces aria-hidden="true" className="size-5" />
              </span>
              <div className="min-w-0">
                <p className="text-muted text-xs font-semibold tracking-wide uppercase">
                  PatchPilot
                </p>
                <h1 className="text-ink truncate text-xl font-semibold">
                  Open a workspace
                </h1>
              </div>
            </div>
            <span className="text-muted hidden text-xs font-medium sm:block">
              Local IDE cockpit
            </span>
          </div>

          <form
            className="grid gap-3 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-end"
            onSubmit={handleSubmit}
          >
            <TextField
              label="Workspace root"
              name="workspace-root"
              onChange={(event) => onRootPathChange(event.target.value)}
              placeholder="/absolute/path/to/repo"
              value={rootPath}
            />
            <Button
              disabled={isCreating || rootPath.trim().length === 0}
              icon={<FolderOpen />}
            >
              Open repo
            </Button>
          </form>

          {createError ? (
            <p className="text-warning text-sm font-medium">{createError}</p>
          ) : null}

          <div className="bg-surface grid gap-3 rounded-xl p-3">
            <div className="flex min-w-0 items-center justify-between gap-3">
              <div className="flex min-w-0 items-center gap-2">
                <History
                  aria-hidden="true"
                  className="text-muted size-4 shrink-0"
                />
                <h2 className="text-ink truncate text-sm font-semibold">
                  Recent workspaces
                </h2>
              </div>
              {isLoadingRecent ? (
                <Loader2
                  aria-label="Loading recent workspaces"
                  className="text-muted size-4 shrink-0 animate-spin"
                />
              ) : null}
            </div>

            {recentError ? (
              <p className="text-warning text-sm font-medium">{recentError}</p>
            ) : null}

            <div className="grid gap-2">
              {recentWorkspaces.map((workspace) => (
                <button
                  className={cn(
                    "bg-panel hover:bg-hover grid min-h-12 min-w-0 gap-1 rounded-xl px-3 py-2 text-left transition",
                  )}
                  key={workspace.id}
                  onClick={() => onSelectWorkspace(workspace.id)}
                  type="button"
                >
                  <span className="flex min-w-0 items-center justify-between gap-2">
                    <span className="text-ink truncate text-sm font-medium">
                      {workspace.name}
                    </span>
                    <StatusPill status={workspace.status} />
                  </span>
                  <span className="text-muted grid min-w-0 grid-cols-[minmax(0,1fr)_auto] items-center gap-2 text-xs">
                    <span className="truncate">{workspace.rootPath}</span>
                    <ArrowRight
                      aria-hidden="true"
                      className="size-4 shrink-0"
                    />
                  </span>
                </button>
              ))}
            </div>

            {!isLoadingRecent &&
            recentWorkspaces.length === 0 &&
            !recentError ? (
              <p className="text-muted text-sm">No recent workspaces yet.</p>
            ) : null}
          </div>
        </Surface>
      </section>
    </main>
  );
}
