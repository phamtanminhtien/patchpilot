import { afterEach, describe, expect, it, vi } from "vitest";

import {
  closeWorkspaceEventConnectionsForTest,
  subscribeWorkspaceEvents,
} from "./workspace-events";

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

describe("workspace event gateway", () => {
  afterEach(() => {
    closeWorkspaceEventConnectionsForTest();
    MockEventSource.instances = [];
    vi.useRealTimers();
    vi.unstubAllGlobals();
  });

  it("shares one EventSource per workspace", () => {
    vi.stubGlobal("EventSource", MockEventSource);
    const first = vi.fn();
    const second = vi.fn();

    const unsubscribeFirst = subscribeWorkspaceEvents("ws_1", first);
    const unsubscribeSecond = subscribeWorkspaceEvents("ws_1", second);

    expect(MockEventSource.instances).toHaveLength(1);
    expect(MockEventSource.instances[0]?.url).toBe(
      "/api/workspaces/ws_1/events",
    );
    expect(MockEventSource.instances[0]?.withCredentials).toBe(true);

    MockEventSource.instances[0]?.emit("git.changed", {
      createdAt: "2026-05-22T00:00:00Z",
      id: "evt_1",
      payload: {},
      type: "git.changed",
      workspaceId: "ws_1",
    });

    expect(first).toHaveBeenCalledOnce();
    expect(second).toHaveBeenCalledOnce();
    unsubscribeFirst();
    unsubscribeSecond();
  });

  it("keeps the connection through immediate resubscribe", () => {
    vi.useFakeTimers();
    vi.stubGlobal("EventSource", MockEventSource);
    const first = vi.fn();
    const second = vi.fn();

    subscribeWorkspaceEvents("ws_1", first)();
    subscribeWorkspaceEvents("ws_1", second);
    vi.advanceTimersByTime(250);

    expect(MockEventSource.instances).toHaveLength(1);
    expect(MockEventSource.instances[0]?.closed).toBe(false);
  });
});
