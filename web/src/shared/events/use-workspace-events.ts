import { useEffect } from "react";

import {
  subscribeWorkspaceEvents,
  type WorkspaceEventHandler,
} from "./workspace-events";

export function useWorkspaceEvents(
  workspaceId: string,
  handler: WorkspaceEventHandler,
) {
  useEffect(
    () => subscribeWorkspaceEvents(workspaceId, handler),
    [handler, workspaceId],
  );
}
