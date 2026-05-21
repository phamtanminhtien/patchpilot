import type { ReactNode } from "react";

export function SidebarHint({
  icon,
  message,
  title,
}: {
  icon: ReactNode;
  message: string;
  title: string;
}) {
  return (
    <div className="grid gap-1.5 rounded-md p-1.5">
      <div className="text-ink flex min-w-0 items-center gap-1.5 text-xs font-semibold">
        <span className="text-accent grid size-6 shrink-0 place-items-center">
          {icon}
        </span>
        <span className="truncate">{title}</span>
      </div>
      <p className="text-muted text-xs leading-5">{message}</p>
    </div>
  );
}
