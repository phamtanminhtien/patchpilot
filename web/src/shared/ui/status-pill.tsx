import { cn } from "./class-name";

interface StatusPillProps {
  className?: string;
  status: string;
}

export function StatusPill({ className, status }: StatusPillProps) {
  const tone =
    status === "ready" || status === "exited"
      ? "bg-success/12 text-success"
      : status === "error" || status === "failed" || status === "stopped"
        ? "bg-danger/12 text-danger"
        : "bg-surface text-muted";

  return (
    <span
      className={cn(
        "inline-flex min-h-5.5 max-w-full items-center rounded-xl px-1.5 text-[11px] font-medium",
        tone,
        className,
      )}
    >
      <span className="truncate">{status}</span>
    </span>
  );
}
