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
        "text-muted flex h-full min-h-9 w-full items-center justify-center gap-1.5 text-center text-xs",
        className,
      )}
    >
      <Loader2 aria-hidden="true" className="size-4 shrink-0 animate-spin" />
      <span>{label}...</span>
    </div>
  );
}
