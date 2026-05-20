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
      <header className="bg-panel shadow-sm">
        <div className="flex min-h-10 w-full items-center gap-2 px-2 py-1 sm:px-3">
          <div className="flex min-w-0 flex-1 items-center gap-1.5">
            <span className="bg-accent-soft text-accent grid size-7 shrink-0 place-items-center rounded-sm shadow-sm">
              <Bot aria-hidden="true" className="size-3.5" />
            </span>
            <div className="flex min-w-0 flex-1 items-center gap-1.5">
              <span className="sr-only">PatchPilot</span>
              <h1 className="text-ink truncate text-sm font-semibold">
                {workspace?.name ?? "No workspace open"}
              </h1>
              {workspace ? <StatusPill status={workspace.status} /> : null}
              {workspace ? (
                <span className="text-muted hidden min-w-0 truncate text-xs md:inline">
                  {workspace.rootPath}
                </span>
              ) : null}
            </div>
          </div>

          <div className="flex shrink-0 items-center gap-1.5">
            <nav
              aria-label="Mode"
              className="bg-canvas grid grid-cols-2 rounded-md p-0.5 shadow-sm"
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
        "text-muted hover:bg-hover hover:text-ink inline-flex min-h-7 min-w-0 items-center justify-center gap-1 rounded-sm px-1.5 text-xs font-medium transition",
        active ? "bg-panel text-accent shadow-sm" : undefined,
      )}
      to={to}
    >
      <span className="grid size-4 shrink-0 place-items-center">{icon}</span>
      <span className="truncate">{children}</span>
    </Link>
  );
}
