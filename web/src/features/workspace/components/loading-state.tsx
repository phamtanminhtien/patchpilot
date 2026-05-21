import { Loader2 } from "lucide-react";

import { cn } from "@/shared/ui";

export function LoadingState({
  className,
  label,
}: {
  className?: string;
  label: string;
}) {
  return (
    <div
      aria-label={label}
      className={cn(
        "text-muted flex min-h-9 items-center gap-1.5 text-xs",
        className,
      )}
    >
      <Loader2 aria-hidden="true" className="size-4 shrink-0 animate-spin" />
      <span>{label}...</span>
    </div>
  );
}
