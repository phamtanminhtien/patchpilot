import { MonitorUp } from "lucide-react";

export function PreviewPanel() {
  return (
    <div className="bg-panel grid h-full min-h-0 overflow-auto p-3">
      <section className="flex min-h-48 items-center justify-center gap-3 p-1">
        <span className="bg-hover text-muted grid size-10 place-items-center rounded-full shadow-sm">
          <MonitorUp aria-hidden="true" className="size-5" />
        </span>
        <div className="grid gap-1">
          <h3 className="text-ink text-sm font-semibold">Same-host previews</h3>
          <p className="text-muted max-w-xl text-sm leading-6">
            Exposed ports stay local to this workspace session. Opening a port
            launches the proxied preview in a separate browser tab.
          </p>
        </div>
      </section>
    </div>
  );
}
