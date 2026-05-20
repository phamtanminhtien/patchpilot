import { Bot, PanelLeft } from "lucide-react";
import type { ReactNode } from "react";
import { Link } from "react-router";

import { useThemePreference } from "@/app/theme";
import type { Workspace } from "@/shared/api";
import { classNames, StatusPill, ThemeSwitcher } from "@/shared/ui";

interface AppShellProps {
  children: ReactNode;
  mode: "vibe" | "workspace";
  workspace?: Workspace;
  workspaceId: string;
}

export function AppShell({
  children,
  mode,
  workspace,
  workspaceId,
}: AppShellProps) {
  const { preference, setPreference } = useThemePreference();
  const query = workspaceId
    ? `?workspaceId=${encodeURIComponent(workspaceId)}`
    : "";

  return (
    <main className="bg-canvas min-h-screen">
      <header className="border-line bg-panel border-b">
        <div className="mx-auto flex w-full max-w-6xl flex-col gap-3 px-4 py-3 sm:px-6 lg:flex-row lg:items-center lg:justify-between">
          <div className="flex min-w-0 items-center gap-3">
            <span className="bg-accent-soft text-accent grid size-10 shrink-0 place-items-center rounded-md">
              <Bot aria-hidden="true" className="size-5" />
            </span>
            <div className="min-w-0">
              <p className="text-muted text-xs font-semibold uppercase">
                PatchPilot
              </p>
              <div className="flex min-w-0 flex-wrap items-center gap-2">
                <h1 className="text-ink truncate text-lg font-semibold">
                  {workspace?.name ?? "No workspace open"}
                </h1>
                {workspace ? <StatusPill status={workspace.status} /> : null}
              </div>
              {workspace ? (
                <p className="text-muted max-w-full truncate text-xs">
                  {workspace.rootPath}
                </p>
              ) : null}
            </div>
          </div>

          <div className="flex flex-wrap items-center gap-2">
            <nav
              aria-label="Mode"
              className="border-line bg-canvas grid grid-cols-2 rounded-md border p-1"
            >
              <ModeLink
                active={mode === "vibe"}
                icon={<Bot aria-hidden="true" className="size-4" />}
                to={`/vibe${query}`}
              >
                Vibe
              </ModeLink>
              <ModeLink
                active={mode === "workspace"}
                icon={<PanelLeft aria-hidden="true" className="size-4" />}
                to={`/workspace${query}`}
              >
                Workspace
              </ModeLink>
            </nav>
            <ThemeSwitcher onChange={setPreference} value={preference} />
          </div>
        </div>
      </header>
      {children}
    </main>
  );
}

function ModeLink({
  active,
  children,
  icon,
  to,
}: {
  active: boolean;
  children: ReactNode;
  icon: ReactNode;
  to: string;
}) {
  return (
    <Link
      aria-current={active ? "page" : undefined}
      className={classNames(
        "text-muted hover:bg-hover hover:text-ink inline-flex min-h-10 min-w-0 items-center justify-center gap-2 rounded-md px-3 text-sm font-medium transition",
        active ? "bg-panel text-accent shadow-sm" : undefined,
      )}
      to={to}
    >
      <span className="grid size-4 shrink-0 place-items-center">{icon}</span>
      <span className="truncate">{children}</span>
    </Link>
  );
}
