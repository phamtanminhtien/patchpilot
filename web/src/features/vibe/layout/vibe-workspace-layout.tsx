import { ArrowDown, Code2, Cpu, Search } from "lucide-react";
import { type ReactNode, type RefObject } from "react";
import { Link } from "react-router";

import { Button } from "@/shared/ui";

export function VibeWorkspaceLayout({
  children,
  composer,
  onJumpToLatest,
  onOpenContext,
  onSearchConversations,
  onScroll,
  scrollContainerRef,
  sidebar,
  showJumpToLatest,
  title,
  workspaceId,
}: {
  children: ReactNode;
  composer: ReactNode;
  onJumpToLatest: () => void;
  onOpenContext: () => void;
  onSearchConversations: () => void;
  onScroll: () => void;
  scrollContainerRef: RefObject<HTMLDivElement | null>;
  sidebar: ReactNode;
  showJumpToLatest: boolean;
  title: string;
  workspaceId: string;
}) {
  return (
    <main className="bg-canvas grid h-screen min-h-0 w-full overflow-hidden md:grid-cols-[19rem_minmax(0,1fr)]">
      {sidebar}

      <section className="grid min-h-0 min-w-0 grid-rows-[3.5rem_minmax(0,1fr)]">
        <header className="bg-panel flex min-w-0 items-center justify-between px-4">
          <div className="flex min-w-0 items-center gap-2">
            <h1 className="text-ink truncate text-sm font-semibold">{title}</h1>
          </div>
          <div className="flex shrink-0 items-center gap-1.5">
            <Button
              aria-label="Search conversations"
              className="md:hidden"
              icon={<Search />}
              onClick={onSearchConversations}
              size="icon"
              type="button"
              variant="secondary"
            />
            <Button
              aria-label="Open agent context"
              icon={<Cpu />}
              onClick={onOpenContext}
              size="compact"
              variant="surface"
            >
              Cockpit
            </Button>
            <Button
              aria-label="Open workspace"
              asChild
              size="compact"
              variant="surface"
            >
              <Link
                to={`/workspace?workspaceId=${encodeURIComponent(workspaceId)}`}
              >
                <Code2 aria-hidden="true" className="size-4 shrink-0" />
                Workspace
              </Link>
            </Button>
          </div>
        </header>

        <div className="relative min-h-0 min-w-0">
          <div
            aria-label="Conversation timeline"
            className="absolute inset-0 min-w-0 overflow-auto px-4 pt-5 pb-48 sm:pb-52"
            onScroll={onScroll}
            ref={scrollContainerRef}
            role="region"
          >
            {children}
          </div>

          {showJumpToLatest ? (
            <div className="pointer-events-none absolute inset-x-0 bottom-28 z-10 px-4 sm:bottom-32">
              <div className="mx-auto flex w-full max-w-3xl justify-center">
                <Button
                  aria-label="Jump to latest"
                  className="pointer-events-auto"
                  icon={<ArrowDown />}
                  onClick={onJumpToLatest}
                  size="small"
                  variant="secondary"
                >
                  Jump to latest
                </Button>
              </div>
            </div>
          ) : null}

          <div className="from-canvas via-canvas/95 pointer-events-none absolute inset-x-0 bottom-0 z-10 bg-linear-to-t to-transparent px-4 pt-12 pb-4">
            <div className="pointer-events-auto mx-auto w-full max-w-3xl">
              {composer}
            </div>
          </div>
        </div>
      </section>
    </main>
  );
}
