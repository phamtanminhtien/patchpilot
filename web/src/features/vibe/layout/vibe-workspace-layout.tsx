import { Code2, Info, MoreHorizontal, PanelLeft } from "lucide-react";
import { type ReactNode } from "react";
import { Link } from "react-router";

import { Button } from "@/shared/ui";

export function VibeWorkspaceLayout({
  children,
  composer,
  sidebar,
  title,
  workspaceId,
}: {
  children: ReactNode;
  composer: ReactNode;
  sidebar: ReactNode;
  title: string;
  workspaceId: string;
}) {
  return (
    <main className="bg-canvas grid h-screen min-h-0 w-full overflow-hidden md:grid-cols-[20rem_minmax(0,1fr)]">
      {sidebar}

      <section className="grid min-h-0 min-w-0 grid-rows-[3.5rem_minmax(0,1fr)_auto]">
        <header className="border-line/30 bg-canvas flex min-w-0 items-center justify-between border-b px-4">
          <div className="flex min-w-0 items-center gap-2">
            <h1 className="text-ink truncate text-sm font-semibold">{title}</h1>
            <Button
              aria-label="Conversation actions"
              icon={<MoreHorizontal />}
              size="icon"
              type="button"
              variant="action"
            />
          </div>
          <div className="flex shrink-0 items-center gap-1.5">
            <Button
              aria-label="Open workspace"
              asChild
              icon={<Code2 />}
              size="compact"
              variant="secondary"
            >
              <Link
                to={`/workspace?workspaceId=${encodeURIComponent(workspaceId)}`}
              >
                Workspace
              </Link>
            </Button>
            <Button
              aria-label="Conversation info"
              icon={<Info />}
              size="icon"
              type="button"
              variant="action"
            />
            <Button
              aria-label="Toggle sidebar"
              icon={<PanelLeft />}
              size="icon"
              type="button"
              variant="action"
            />
          </div>
        </header>

        <div className="min-h-0 min-w-0 overflow-auto px-4 py-4">
          {children}
        </div>

        {composer}
      </section>
    </main>
  );
}
