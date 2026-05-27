import type { ReactNode } from "react";

export function MainEmptyState({
  action,
  icon,
  message,
  title,
}: {
  action?: ReactNode;
  icon: ReactNode;
  message: string;
  title: string;
}) {
  return (
    <div className="grid min-h-56 place-items-center p-3">
      <div className="grid max-w-md justify-items-center gap-2 text-center">
        <span className="bg-surface text-accent grid size-9 place-items-center rounded-xl">
          {icon}
        </span>
        <div className="grid gap-1">
          <h3 className="text-ink text-sm font-semibold">{title}</h3>
          <p className="text-muted text-xs leading-5">{message}</p>
        </div>
        {action}
      </div>
    </div>
  );
}
