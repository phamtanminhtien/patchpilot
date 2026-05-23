import { ArrowDown, Code2 } from "lucide-react";
import { type ReactNode, type RefObject } from "react";
import { Link } from "react-router";

import { Button } from "@/shared/ui";

export function VibeWorkspaceLayout({
  children,
  composer,
  onJumpToLatest,
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
  onScroll: () => void;
  scrollContainerRef: RefObject<HTMLDivElement | null>;
  sidebar: ReactNode;
  showJumpToLatest: boolean;
  title: string;
  workspaceId: string;
}) {
  return (
    <main className="bg-canvas grid h-screen min-h-0 w-full overflow-hidden md:grid-cols-[20rem_minmax(0,1fr)]">
      {sidebar}

      <section className="grid min-h-0 min-w-0 grid-rows-[3.5rem_minmax(0,1fr)]">
        <header className="border-line/30 bg-canvas flex min-w-0 items-center justify-between border-b px-4">
          <div className="flex min-w-0 items-center gap-2">
            <h1 className="text-ink truncate text-sm font-semibold">{title}</h1>
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
          </div>
        </header>

        <div className="relative min-h-0 min-w-0">
          <div
            aria-label="Conversation timeline"
            className="absolute inset-0 min-w-0 overflow-auto px-4 pt-4 pb-48 sm:pb-52"
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

          <div className="pointer-events-none absolute inset-x-0 bottom-0 z-10 px-4 pb-4">
            <div className="pointer-events-auto mx-auto w-full max-w-3xl">
              {composer}
            </div>
          </div>
        </div>
      </section>
    </main>
  );
}
