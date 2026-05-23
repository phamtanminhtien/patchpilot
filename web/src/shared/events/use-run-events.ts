import { useEffect } from "react";

import { type RunEventHandler, subscribeRunEvents } from "./run-events";

export function useRunEvents(
  workspaceId: string,
  conversationId: string,
  runId: string,
  handler: RunEventHandler,
) {
  useEffect(
    () => subscribeRunEvents(workspaceId, conversationId, runId, handler),
    [conversationId, handler, runId, workspaceId],
  );
}
