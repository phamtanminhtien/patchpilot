export type AppMode = "vibe" | "workspace";

const desktopQuery = "(min-width: 1024px)";

export function createDefaultMode(
  matchMediaImpl:
    | ((query: string) => Pick<MediaQueryList, "matches">)
    | undefined = globalThis.matchMedia,
): AppMode {
  if (!matchMediaImpl) {
    return "vibe";
  }

  return matchMediaImpl(desktopQuery).matches ? "workspace" : "vibe";
}
