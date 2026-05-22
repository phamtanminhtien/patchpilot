import { ExternalLink, Loader2, MonitorUp } from "lucide-react";

import type { Port } from "@/shared/api";
import { Button, StatusPill } from "@/shared/ui";

import { ErrorState } from "../components/error-state";
import { LoadingState } from "../components/loading-state";
import { MainEmptyState } from "../components/main-empty-state";

export function PreviewPanel({
  error,
  exposeError,
  exposingPort,
  isExposing,
  isLoading,
  onExpose,
  ports,
}: {
  error?: string;
  exposeError?: string;
  exposingPort?: number;
  isExposing: boolean;
  isLoading: boolean;
  onExpose: (port: number) => void;
  ports: Port[];
}) {
  if (isLoading && ports.length === 0) {
    return <LoadingState label="Loading preview ports" />;
  }

  if (ports.length === 0) {
    return (
      <MainEmptyState
        icon={<MonitorUp aria-hidden="true" className="size-6" />}
        message={
          error ?? "Run a dev command to detect a local preview port here."
        }
        title="No preview port detected"
      />
    );
  }

  return (
    <div className="grid h-full min-h-0 grid-rows-[auto_minmax(0,1fr)] overflow-hidden">
      <div className="border-line bg-panel grid gap-2 border-b p-3">
        <div className="flex min-w-0 items-center justify-between gap-3">
          <div className="min-w-0">
            <h3 className="text-ink text-sm font-semibold">
              Same-host previews
            </h3>
            <p className="text-muted text-xs">
              {ports.length} detected {ports.length === 1 ? "port" : "ports"}
            </p>
          </div>
          <MonitorUp aria-hidden="true" className="text-accent size-5" />
        </div>
        {exposeError ? <ErrorState message={exposeError} /> : null}
      </div>

      <div
        aria-label="Preview ports"
        className="grid min-h-0 content-start gap-2 overflow-auto p-3"
        role="region"
      >
        {ports.map((port) => (
          <PreviewPortCard
            isPending={isExposing && exposingPort === port.port}
            key={port.id}
            onExpose={onExpose}
            port={port}
          />
        ))}
      </div>
    </div>
  );
}

function PreviewPortCard({
  isPending,
  onExpose,
  port,
}: {
  isPending: boolean;
  onExpose: (port: number) => void;
  port: Port;
}) {
  const isOpenable = port.status === "exposed" && port.exposedUrl;

  return (
    <section className="bg-canvas grid min-w-0 gap-3 rounded-md p-3 shadow-sm">
      <div className="flex min-w-0 items-start justify-between gap-3">
        <div className="min-w-0">
          <p className="text-ink truncate text-sm font-semibold">
            localhost:{port.port}
          </p>
          <p className="text-muted truncate text-xs">
            {port.exposedUrl ?? "Expose the port to open the backend proxy."}
          </p>
        </div>
        <StatusPill status={port.status} />
      </div>

      <div className="flex flex-wrap justify-end gap-2">
        {isOpenable ? (
          <Button asChild icon={<ExternalLink />} size="small">
            <a href={port.exposedUrl ?? ""} rel="noreferrer" target="_blank">
              Open preview
            </a>
          </Button>
        ) : (
          <Button
            disabled={isPending || port.status === "closed"}
            icon={
              isPending ? (
                <Loader2 className="animate-spin" />
              ) : (
                <ExternalLink />
              )
            }
            onClick={() => onExpose(port.port)}
            size="small"
            type="button"
          >
            Expose port
          </Button>
        )}
      </div>
    </section>
  );
}
