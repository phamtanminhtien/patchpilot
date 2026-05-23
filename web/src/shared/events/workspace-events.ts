import type { WorkspaceEvent } from "@/shared/api";

export type WorkspaceEventHandler = (event: WorkspaceEvent) => void;

const workspaceEventTypes: WorkspaceEvent["type"][] = [
  "workspace.indexing",
  "workspace.ready",
  "conversation.message.created",
  "git.changed",
  "port.opened",
  "port.exposed",
  "port.closed",
  "process.started",
  "process.exited",
  "command.output",
  "agent.delta",
  "agent.tool.started",
  "agent.tool.finished",
  "agent.approval_required",
  "agent.run.status_changed",
];

interface WorkspaceEventConnection {
  closeTimer: ReturnType<typeof setTimeout> | undefined;
  handlers: Set<WorkspaceEventHandler>;
  source: EventSource;
}

const connections = new Map<string, WorkspaceEventConnection>();

export function subscribeWorkspaceEvents(
  workspaceId: string,
  handler: WorkspaceEventHandler,
) {
  if (workspaceId.length === 0 || typeof EventSource === "undefined") {
    return () => {};
  }
  const connection = connectionForWorkspace(workspaceId);
  connection.handlers.add(handler);
  if (connection.closeTimer !== undefined) {
    clearTimeout(connection.closeTimer);
    connection.closeTimer = undefined;
  }

  return () => {
    connection.handlers.delete(handler);
    if (connection.handlers.size > 0) {
      return;
    }
    connection.closeTimer = setTimeout(() => {
      if (connection.handlers.size > 0) {
        return;
      }
      connection.source.close();
      connections.delete(workspaceId);
    }, 250);
  };
}

export function closeWorkspaceEventConnectionsForTest() {
  for (const [workspaceId, connection] of connections) {
    if (connection.closeTimer !== undefined) {
      clearTimeout(connection.closeTimer);
    }
    connection.source.close();
    connections.delete(workspaceId);
  }
}

function connectionForWorkspace(workspaceId: string) {
  const current = connections.get(workspaceId);
  if (current) {
    return current;
  }

  const source = new EventSource(`/api/workspaces/${workspaceId}/events`, {
    withCredentials: true,
  });
  const connection: WorkspaceEventConnection = {
    closeTimer: undefined,
    handlers: new Set(),
    source,
  };
  const handleMessage = (message: MessageEvent<string>) => {
    const event = JSON.parse(message.data) as WorkspaceEvent;
    for (const handler of connection.handlers) {
      handler(event);
    }
  };
  for (const eventType of workspaceEventTypes) {
    source.addEventListener(eventType, handleMessage);
  }
  connections.set(workspaceId, connection);
  return connection;
}
