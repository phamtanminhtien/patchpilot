export function ThinkingIndicator({ status }: { status: string }) {
  return (
    <div className="max-w-[78%] text-sm font-medium">
      <span
        className="pp-shimmer-text"
        style={
          {
            "--pp-shimmer-base":
              "color-mix(in srgb, var(--pp-color-muted) 45%, transparent)",
          } as React.CSSProperties
        }
      >
        {status === "waiting_tool_approval"
          ? "Waiting for approval"
          : "Thinking"}
      </span>
    </div>
  );
}
