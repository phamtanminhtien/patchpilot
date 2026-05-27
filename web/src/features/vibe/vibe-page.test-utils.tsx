import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import type userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { MemoryRouter } from "react-router";
import { expect } from "vitest";

import { ThemeProvider } from "@/app/theme";

const queryState = globalThis.__patchPilotNuqsQueryState;

export function queryStateValue(key: string) {
  return queryState.get(key);
}

export class MockEventSource {
  static instances: MockEventSource[] = [];

  listeners = new Map<string, ((message: MessageEvent<string>) => void)[]>();
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

  close() {}

  emit(type: string, data: unknown) {
    for (const listener of this.listeners.get(type) ?? []) {
      listener({ data: JSON.stringify(data) } as MessageEvent<string>);
    }
  }
}

export const workspace = {
  createdAt: "2026-05-20T00:00:00Z",
  id: "ws_1",
  name: "patchpilot",
  rootPath: "/workspace/patchpilot",
  status: "ready" as const,
  updatedAt: "2026-05-20T00:00:00Z",
};

export const run = {
  conversationId: "conv_1",
  createdAt: "2026-05-20T00:00:00Z",
  error: null,
  finishedAt: null,
  id: "run_1",
  model: "gpt-5.4-mini" as const,
  reasoningEffort: "high" as const,
  startedAt: null,
  status: "queued" as const,
  summary: "",
  triggerMessageId: "msg_1",
  updatedAt: "2026-05-20T00:00:00Z",
  workspaceId: "ws_1",
};

export const doneRun = {
  ...run,
  finishedAt: "2026-05-20T00:00:01Z",
  status: "done" as const,
  summary: "Done",
};

export const conversation = {
  createdAt: "2026-05-20T00:00:00Z",
  hasRunningRun: false,
  id: "conv_1",
  lastMessageAt: "2026-05-20T00:00:00Z",
  title: "Fix the failing test",
  updatedAt: "2026-05-20T00:00:00Z",
  workspaceId: "ws_1",
};

export const searchConversation = {
  ...conversation,
  createdAt: "2026-05-20T00:00:01Z",
  id: "conv_2",
  lastMessageAt: "2026-05-20T00:00:01Z",
  title: "Search conversation modal",
  updatedAt: "2026-05-20T00:00:01Z",
};

export const message = {
  content: "Fix the failing test",
  conversationId: "conv_1",
  createdAt: "2026-05-20T00:00:00Z",
  id: "msg_1",
  role: "user" as const,
  runId: "run_1",
  workspaceId: "ws_1",
};

export const agentContext = {
  contextWarnings: [],
  instructionSources: [],
  mcpServers: [],
  mcpTools: [],
  refreshedAt: "2026-05-20T00:00:00Z",
  skills: [
    {
      description: "Browser automation for local targets.",
      enabled: true,
      instruction: "Use the in-app browser to inspect local UI.",
      key: "browser",
      name: "browser",
      path: "patchpilot/browser",
      source: "patchpilot",
      valid: true,
    },
    {
      description: "",
      enabled: true,
      instruction: "",
      key: "broken-skill",
      name: "Broken Skill",
      path: "patchpilot/broken-skill",
      source: "patchpilot",
      valid: false,
      warning: "SKILL.md frontmatter requires a non-empty description.",
    },
  ],
  skippedSources: [],
};

export const fileIndex = {
  entries: [
    {
      modifiedAt: "2026-05-20T00:00:00Z",
      path: "docs/product-spec.md",
      size: 128,
    },
    {
      modifiedAt: "2026-05-20T00:00:00Z",
      path: "web/src/features/vibe/components/composer.tsx",
      size: 256,
    },
  ],
  nextCursor: null,
};

export const toolCall = {
  batchId: "batch_1",
  createdAt: "2026-05-20T00:00:00Z",
  decision: null,
  finishedAt: null,
  id: "evt_1",
  input:
    '{"summary":"Update example","diff":"diff --git a/example.txt b/example.txt\\n"}',
  name: "apply_patch",
  output: "{}",
  providerCallId: "call_1",
  requiresApproval: true,
  sequence: 0,
  startedAt: null,
  status: "waiting_approval" as const,
  runId: "run_1",
  workspaceId: "ws_1",
};

export function renderVibePage(initialEntry: string, page: ReactNode) {
  queryState.clear();
  const url = new URL(initialEntry, "http://localhost");
  url.searchParams.forEach((value, key) => {
    if (value.length > 0) {
      queryState.set(key, value);
    }
  });

  const queryClient = new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
      },
    },
  });

  return render(
    <QueryClientProvider client={queryClient}>
      <ThemeProvider>
        <MemoryRouter initialEntries={[initialEntry]}>{page}</MemoryRouter>
      </ThemeProvider>
    </QueryClientProvider>,
  );
}

export async function openExistingConversation() {
  const title = await screen.findByText(conversation.title);
  const button = title.closest("button");
  if (button === null) {
    throw new Error("Conversation button was not found");
  }
  fireEvent.click(button);
}

export async function submitPrompt(
  user: ReturnType<typeof userEvent.setup>,
  content: string,
) {
  await waitFor(() => {
    expect(screen.getByLabelText("Ask AI")).toBeEnabled();
  });
  const promptInput = screen.getByLabelText("Ask AI");
  await user.clear(promptInput);
  await user.type(promptInput, content);
  const startButton = screen.getByRole("button", { name: "Start run" });
  await waitFor(() => {
    expect(startButton).toBeEnabled();
  });
  await user.click(startButton);
}

export async function waitForRunEventSource() {
  await waitFor(() => {
    expect(
      MockEventSource.instances.some((source) =>
        source.url.includes("/runs/run_1/events"),
      ),
    ).toBe(true);
  });
  const source = MockEventSource.instances.find((item) =>
    item.url.includes("/runs/run_1/events"),
  );
  if (source === undefined) {
    throw new Error("Run EventSource was not found");
  }
  return source;
}

export async function waitForWorkspaceEventSource() {
  await waitFor(() => {
    expect(
      MockEventSource.instances.some((source) =>
        source.url.includes("/workspaces/ws_1/events"),
      ),
    ).toBe(true);
  });
  const source = MockEventSource.instances.find((item) =>
    item.url.includes("/workspaces/ws_1/events"),
  );
  if (source === undefined) {
    throw new Error("Workspace EventSource was not found");
  }
  return source;
}

export function setScrollMetrics(
  element: HTMLElement,
  metrics: {
    clientHeight: number;
    scrollHeight: number;
    scrollTop: number;
  },
) {
  Object.defineProperty(element, "clientHeight", {
    configurable: true,
    value: metrics.clientHeight,
  });
  Object.defineProperty(element, "scrollHeight", {
    configurable: true,
    value: metrics.scrollHeight,
  });
  Object.defineProperty(element, "scrollTop", {
    configurable: true,
    value: metrics.scrollTop,
    writable: true,
  });
}
