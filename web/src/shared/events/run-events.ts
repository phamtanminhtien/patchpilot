import type { WorkspaceEvent } from "@/shared/api";

export type RunEventHandler = (event: WorkspaceEvent) => void;

const runEventTypes: WorkspaceEvent["type"][] = [
  "conversation.message.created",
  "agent.delta",
  "agent.output.snapshot",
  "agent.tool.started",
  "agent.tool.finished",
  "agent.approval_required",
  "agent.run.status_changed",
];

interface RunEventConnection {
  closeTimer: ReturnType<typeof setTimeout> | undefined;
  handlers: Set<RunEventHandler>;
  source: EventSource;
}

const connections = new Map<string, RunEventConnection>();

export function subscribeRunEvents(
  workspaceId: string,
  conversationId: string,
  runId: string,
  handler: RunEventHandler,
) {
  if (
    workspaceId.length === 0 ||
    conversationId.length === 0 ||
    runId.length === 0 ||
    typeof EventSource === "undefined"
  ) {
    return () => {};
  }
  const connection = connectionForRun(workspaceId, conversationId, runId);
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
      connections.delete(connectionKey(workspaceId, conversationId, runId));
    }, 250);
  };
}

export function closeRunEventConnectionsForTest() {
  for (const [key, connection] of connections) {
    if (connection.closeTimer !== undefined) {
      clearTimeout(connection.closeTimer);
    }
    connection.source.close();
    connections.delete(key);
  }
}

function connectionForRun(
  workspaceId: string,
  conversationId: string,
  runId: string,
) {
  const key = connectionKey(workspaceId, conversationId, runId);
  const current = connections.get(key);
  if (current) {
    return current;
  }

  const source = new EventSource(
    `/api/workspaces/${workspaceId}/conversations/${conversationId}/runs/${runId}/events`,
    { withCredentials: true },
  );
  const connection: RunEventConnection = {
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
  for (const eventType of runEventTypes) {
    source.addEventListener(eventType, handleMessage);
  }
  connections.set(key, connection);
  return connection;
}

function connectionKey(
  workspaceId: string,
  conversationId: string,
  runId: string,
) {
  return `${workspaceId}:${conversationId}:${runId}`;
}
