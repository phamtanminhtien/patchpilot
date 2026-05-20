import { classNames } from "./class-name";

interface StatusPillProps {
  className?: string;
  status: string;
}

export function StatusPill({ className, status }: StatusPillProps) {
  const tone =
    status === "ready"
      ? "bg-accent-soft text-accent"
      : status === "error"
        ? "bg-panel text-warning"
        : "bg-hover text-muted";

  return (
    <span
      className={classNames(
        "inline-flex min-h-7 max-w-full items-center rounded-md px-2 text-xs font-medium",
        tone,
        className,
      )}
    >
      <span className="truncate">{status}</span>
    </span>
  );
}
