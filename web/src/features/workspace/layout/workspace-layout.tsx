import type { ReactNode } from "react";

export function WorkspaceLayout({
  activityRail,
  bottomPanel,
  mainPanels,
  sidebar,
}: {
  activityRail: ReactNode;
  bottomPanel: ReactNode;
  mainPanels: ReactNode;
  sidebar: ReactNode;
}) {
  return (
    <section className="grid h-[calc(100vh-2.5rem)] min-h-0 grid-rows-[auto_minmax(0,1fr)] overflow-hidden lg:grid-cols-[3.5rem_15.5rem_minmax(0,1fr)] lg:grid-rows-1">
      {activityRail}

      <div className="grid min-h-0 grid-rows-[auto_minmax(0,1fr)] overflow-hidden lg:contents">
        {sidebar}

        <main className="grid min-h-0 grid-rows-[minmax(0,1fr)_14rem] overflow-hidden lg:grid-rows-[minmax(0,1fr)_16rem]">
          {mainPanels}
          {bottomPanel}
        </main>
      </div>
    </section>
  );
}
