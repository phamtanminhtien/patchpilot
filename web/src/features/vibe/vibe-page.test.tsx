import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type * as NuqsModule from "nuqs";
import type * as ReactModule from "react";
import { MemoryRouter } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ThemeProvider } from "@/app/theme";
import {
  approveAgentToolCall,
  createConversation,
  createMessage,
  createWorkspace,
  getConversation,
  getHealth,
  getWorkspace,
  listConversations,
  listWorkspaces,
  rejectAgentToolCall,
} from "@/shared/api";

import { VibePage } from "./vibe-page";

const queryState = vi.hoisted(() => new Map<string, string>());

vi.mock("@/shared/api", () => ({
  apiErrorMessage: (error: unknown) =>
    error instanceof Error ? error.message : "Request failed",
  approveAgentToolCall: vi.fn(),
  createConversation: vi.fn(),
  createMessage: vi.fn(),
  createWorkspace: vi.fn(),
  getConversation: vi.fn(),
  getHealth: vi.fn(),
  getWorkspace: vi.fn(),
  listConversations: vi.fn(),
  listWorkspaces: vi.fn(),
  rejectAgentToolCall: vi.fn(),
}));

vi.mock("nuqs", async () => {
  const React = await vi.importActual<typeof ReactModule>("react");
  const actual = await vi.importActual<typeof NuqsModule>("nuqs");

  return {
    ...actual,
    useQueryState: (key: string) => {
      const [value, setValue] = React.useState(queryState.get(key) ?? "");
      return [
        value,
        (nextValue: string) => {
          queryState.set(key, nextValue);
          setValue(nextValue);
          return Promise.resolve(new URLSearchParams([...queryState]));
        },
      ] as const;
    },
  };
});

const workspace = {
  createdAt: "2026-05-20T00:00:00Z",
  id: "ws_1",
  name: "patchpilot",
  rootPath: "/workspace/patchpilot",
  status: "ready" as const,
  updatedAt: "2026-05-20T00:00:00Z",
};

const run = {
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

const conversation = {
  createdAt: "2026-05-20T00:00:00Z",
  id: "conv_1",
  lastMessageAt: "2026-05-20T00:00:00Z",
  title: "Fix the failing test",
  updatedAt: "2026-05-20T00:00:00Z",
  workspaceId: "ws_1",
};

const message = {
  content: "Fix the failing test",
  conversationId: "conv_1",
  createdAt: "2026-05-20T00:00:00Z",
  id: "msg_1",
  role: "user" as const,
  runId: "run_1",
  workspaceId: "ws_1",
};

describe("VibePage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(createWorkspace).mockResolvedValue(workspace);
    vi.mocked(getHealth).mockResolvedValue({ status: "ok" });
    vi.mocked(getWorkspace).mockResolvedValue(workspace);
    vi.mocked(listWorkspaces).mockResolvedValue({ workspaces: [] });
    vi.mocked(listConversations).mockResolvedValue({ conversations: [] });
    vi.mocked(approveAgentToolCall).mockResolvedValue(toolCall);
    vi.mocked(rejectAgentToolCall).mockResolvedValue({
      ...toolCall,
      decision: "rejected",
      status: "rejected",
    });
    vi.mocked(getConversation).mockResolvedValue({
      conversation,
      events: [],
      messages: [message],
      runs: [run],
      toolCalls: [],
    });
    vi.mocked(createConversation).mockResolvedValue(conversation);
    vi.mocked(createMessage).mockResolvedValue({ message, run });
  });

  it("creates an agent run with selected model and reasoning effort", async () => {
    const user = userEvent.setup();
    renderVibe("/vibe?workspaceId=ws_1");

    const promptInput = await screen.findByLabelText("Ask AI");
    await waitFor(() => {
      expect(promptInput).toBeEnabled();
    });
    await user.type(promptInput, "Fix the failing test");
    await user.click(screen.getByRole("combobox", { name: "Model" }));
    await user.click(screen.getByRole("option", { name: "gpt-5.4-mini" }));
    await user.click(screen.getByRole("combobox", { name: "Reasoning" }));
    await user.click(screen.getByRole("option", { name: "high" }));
    const startButton = screen.getByRole("button", { name: "Start run" });
    await waitFor(() => {
      expect(startButton).toBeEnabled();
    });
    await user.click(startButton);

    await waitFor(() => {
      expect(createConversation).toHaveBeenCalledWith("ws_1", {
        title: "Fix the failing test",
      });
      expect(createMessage).toHaveBeenCalledWith("ws_1", "conv_1", {
        content: "Fix the failing test",
        model: "gpt-5.4-mini",
        reasoningEffort: "high",
      });
    });
    expect(await screen.findAllByText("Fix the failing test")).toHaveLength(3);
  });

  it("keeps the run list in a bounded scroll region", async () => {
    vi.mocked(listConversations).mockResolvedValue({
      conversations: [conversation],
    });
    renderVibe("/vibe?workspaceId=ws_1");

    const taskList = await screen.findByRole("region", {
      name: "Agent conversations",
    });

    expect(taskList).toHaveClass("min-h-0", "min-w-0", "overflow-auto");
    expect(taskList.parentElement).toHaveClass("grid", "overflow-hidden");
  });

  it("renders and approves an approval-required tool call", async () => {
    const user = userEvent.setup();
    vi.mocked(listConversations).mockResolvedValue({
      conversations: [conversation],
    });
    vi.mocked(getConversation).mockResolvedValue({
      conversation,
      events: [],
      messages: [message],
      runs: [{ ...run, status: "waiting_tool_approval" }],
      toolCalls: [toolCall],
    });
    renderVibe("/vibe?workspaceId=ws_1");

    expect(await screen.findByText("apply_patch")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Approve tool" }));

    await waitFor(() => {
      expect(approveAgentToolCall).toHaveBeenCalledWith(
        "ws_1",
        "conv_1",
        "run_1",
        "evt_1",
      );
    });
  });

  it("renders tool calls as collapsed details instead of raw event blocks", async () => {
    vi.mocked(listConversations).mockResolvedValue({
      conversations: [conversation],
    });
    vi.mocked(getConversation).mockResolvedValue({
      conversation,
      events: [
        {
          createdAt: "2026-05-20T00:00:00Z",
          id: "event_1",
          payload: toolCall,
          runId: "run_1",
          type: "agent.approval_required",
          workspaceId: "ws_1",
        },
      ],
      messages: [message],
      runs: [run],
      toolCalls: [toolCall],
    });
    renderVibe("/vibe?workspaceId=ws_1");

    expect(await screen.findByText("apply_patch")).toBeInTheDocument();
    expect(
      screen.queryByRole("group", { name: "agent.approval_required" }),
    ).not.toBeInTheDocument();
  });

  it("keeps previous conversation messages visible after newer runs", async () => {
    const secondMessage = {
      ...message,
      content: "Then update the UI",
      id: "msg_2",
      runId: "run_2",
    };
    const firstAssistantMessage = {
      ...message,
      content: "Fixed the failing test",
      id: "msg_3",
      role: "assistant" as const,
      runId: "run_1",
    };
    const secondAssistantMessage = {
      ...message,
      content: "Updated the UI",
      id: "msg_4",
      role: "assistant" as const,
      runId: "run_2",
    };
    const secondRun = {
      ...run,
      id: "run_2",
      summary: "Updated the UI",
      triggerMessageId: "msg_2",
    };
    vi.mocked(listConversations).mockResolvedValue({
      conversations: [conversation],
    });
    vi.mocked(getConversation).mockResolvedValue({
      conversation,
      events: [],
      messages: [
        message,
        firstAssistantMessage,
        secondMessage,
        secondAssistantMessage,
      ],
      runs: [
        { ...run, status: "done", summary: "Fixed the failing test" },
        secondRun,
      ],
      toolCalls: [],
    });
    renderVibe("/vibe?workspaceId=ws_1");

    expect(await screen.findByText("Then update the UI")).toBeInTheDocument();
    expect(screen.getAllByText("Fix the failing test").length).toBeGreaterThan(
      0,
    );
    expect(screen.getByText("Fixed the failing test")).toBeInTheDocument();
    expect(screen.getByText("Updated the UI")).toBeInTheDocument();
  });

  it("renders assistant messages and tool calls for the same run in timeline order", async () => {
    const progressMessage = {
      ...message,
      content: "I will inspect the workspace.",
      createdAt: "2026-05-20T00:00:01Z",
      id: "msg_2",
      role: "assistant" as const,
      runId: "run_1",
    };
    const finalMessage = {
      ...message,
      content: "The workspace has Docker files.",
      createdAt: "2026-05-20T00:00:03Z",
      id: "msg_3",
      role: "assistant" as const,
      runId: "run_1",
    };
    const timelineToolCall = {
      ...toolCall,
      createdAt: "2026-05-20T00:00:02Z",
      id: "evt_2",
      name: "search_files",
    };
    vi.mocked(listConversations).mockResolvedValue({
      conversations: [conversation],
    });
    vi.mocked(getConversation).mockResolvedValue({
      conversation,
      events: [],
      messages: [message, finalMessage, progressMessage],
      runs: [
        { ...run, status: "done", summary: "The workspace has Docker files." },
      ],
      toolCalls: [timelineToolCall],
    });
    renderVibe("/vibe?workspaceId=ws_1");

    const progress = await screen.findByText("I will inspect the workspace.");
    const tool = screen.getByText("search_files");
    const final = screen.getByText("The workspace has Docker files.");

    expect(
      progress.compareDocumentPosition(tool) & Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
    expect(
      tool.compareDocumentPosition(final) & Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
  });
});

const toolCall = {
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

function renderVibe(initialEntry: string) {
  queryState.clear();
  const url = new URL(initialEntry, "http://localhost");
  const workspaceId = url.searchParams.get("workspaceId");
  if (workspaceId !== null) {
    queryState.set("workspaceId", workspaceId);
  }

  const queryClient = new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
      },
    },
  });

  render(
    <QueryClientProvider client={queryClient}>
      <ThemeProvider>
        <MemoryRouter initialEntries={[initialEntry]}>
          <VibePage />
        </MemoryRouter>
      </ThemeProvider>
    </QueryClientProvider>,
  );
}
