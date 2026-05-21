import { cn } from "@/shared/ui";

export function ErrorState({
  className,
  message,
}: {
  className?: string;
  message: string;
}) {
  return (
    <p
      className={cn("text-warning text-center text-xs font-medium", className)}
    >
      {message}
    </p>
  );
}
