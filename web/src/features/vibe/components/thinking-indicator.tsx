export function ThinkingIndicator({ status }: { status: string }) {
  return (
    <div className="text-muted flex max-w-[78%] items-center gap-2 text-sm">
      <span className="flex gap-1">
        <span className="bg-muted size-1.5 animate-pulse rounded-full" />
        <span className="bg-muted size-1.5 animate-pulse rounded-full [animation-delay:120ms]" />
        <span className="bg-muted size-1.5 animate-pulse rounded-full [animation-delay:240ms]" />
      </span>
      <span>
        {status === "waiting_tool_approval"
          ? "Waiting for approval"
          : "Thinking"}
      </span>
    </div>
  );
}
