import type { ReactNode } from "react";

export function MainEmptyState({
  icon,
  message,
  title,
}: {
  icon: ReactNode;
  message: string;
  title: string;
}) {
  return (
    <div className="grid min-h-56 place-items-center p-3">
      <div className="grid max-w-md justify-items-center gap-2 text-center">
        <span className="bg-accent-soft text-accent grid size-10 place-items-center rounded-md shadow-sm">
          {icon}
        </span>
        <div className="grid gap-1">
          <h3 className="text-ink text-sm font-semibold">{title}</h3>
          <p className="text-muted text-xs leading-5">{message}</p>
        </div>
      </div>
    </div>
  );
}
