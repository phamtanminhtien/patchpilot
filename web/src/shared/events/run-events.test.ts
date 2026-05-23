import { afterEach, describe, expect, it, vi } from "vitest";

import {
  closeRunEventConnectionsForTest,
  subscribeRunEvents,
} from "./run-events";

class MockEventSource {
  static instances: MockEventSource[] = [];

  listeners = new Map<string, ((message: MessageEvent<string>) => void)[]>();
  closed = false;
  url: string;
  withCredentials: boolean;

  constructor(url: string, init?: EventSourceInit) {
    this.url = url;
    this.withCredentials = init?.withCredentials ?? false;
    MockEventSource.instances.push(this);
  }

  addEventListener(
    type: string,
    listener: (message: MessageEvent<string>) => void,
  ) {
    this.listeners.set(type, [...(this.listeners.get(type) ?? []), listener]);
  }

  close() {
    this.closed = true;
  }

  emit(type: string, data: unknown) {
    for (const listener of this.listeners.get(type) ?? []) {
      listener({ data: JSON.stringify(data) } as MessageEvent<string>);
    }
  }
}

describe("run event gateway", () => {
  afterEach(() => {
    closeRunEventConnectionsForTest();
    MockEventSource.instances = [];
    vi.useRealTimers();
    vi.unstubAllGlobals();
  });

  it("subscribes to run output snapshots", () => {
    vi.stubGlobal("EventSource", MockEventSource);
    const handler = vi.fn();

    subscribeRunEvents("ws_1", "conv_1", "run_1", handler);

    expect(MockEventSource.instances[0]?.url).toBe(
      "/api/workspaces/ws_1/conversations/conv_1/runs/run_1/events",
    );
    expect(MockEventSource.instances[0]?.withCredentials).toBe(true);

    MockEventSource.instances[0]?.emit("agent.output.snapshot", {
      createdAt: "2026-05-22T00:00:00Z",
      id: "evt_inline",
      payload: { runId: "run_1", text: "Draft" },
      type: "agent.output.snapshot",
      workspaceId: "ws_1",
    });

    expect(handler).toHaveBeenCalledWith(
      expect.objectContaining({
        type: "agent.output.snapshot",
        payload: { runId: "run_1", text: "Draft" },
      }),
    );
  });
});
