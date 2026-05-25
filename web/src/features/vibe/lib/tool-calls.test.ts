import { describe, expect, it } from "vitest";

import type { AgentToolCall } from "@/shared/api";

import { toolCallDisplay } from "./tool-calls";

const baseToolCall: AgentToolCall = {
  batchId: "batch_1",
  createdAt: "2026-05-20T00:00:00Z",
  decision: null,
  finishedAt: "2026-05-20T00:00:01Z",
  id: "evt_1",
  input: "{}",
  name: "read_file",
  output: "",
  providerCallId: "call_1",
  requiresApproval: false,
  runId: "run_1",
  sequence: 0,
  source: "builtin",
  sourceRef: null,
  startedAt: "2026-05-20T00:00:00Z",
  status: "finished",
  workspaceId: "ws_1",
};

describe("toolCallDisplay", () => {
  it("formats use_skill calls with a human-readable skill name", () => {
    const display = toolCallDisplay({
      ...baseToolCall,
      input: '{"name":"incremental-implementation"}',
      name: "use_skill",
      output: '{"instruction":"Implement in small verified steps."}',
      source: "skill",
      sourceRef: "incremental-implementation",
    });

    expect(display.label).toBe("Load skill");
    expect(display.statusLabel).toBe("Loaded");
    expect(display.text).toBe("Incremental Implementation");
    expect(display.detail).toBe("Implement in small verified steps.");
  });
});
