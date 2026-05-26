import { act, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, expect, it, vi } from "vitest";

import {
  approveAgentToolCall,
  cancelAgentRun,
  createConversation,
  createMessage,
  createWorkspace,
  getAgentContext,
  getConversation,
  getHealth,
  getWorkspace,
  listConversations,
  listFileIndex,
  listWorkspaces,
  refreshAgentContext,
  rejectAgentToolCall,
  setAgentSkillEnabled,
} from "@/shared/api";
import {
  closeRunEventConnectionsForTest,
  closeWorkspaceEventConnectionsForTest,
} from "@/shared/events";

import { VibePage } from "./vibe-page";
import {
  agentContext,
  conversation,
  fileIndex,
  message,
  MockEventSource,
  openExistingConversation,
  renderVibePage,
  run,
  toolCall,
  waitForRunEventSource,
  workspace,
} from "./vibe-page.test-utils";

vi.mock("@/shared/api", () => ({
  apiErrorMessage: (error: unknown) =>
    error instanceof Error ? error.message : "Request failed",
  approveAgentToolCall: vi.fn(),
  cancelAgentRun: vi.fn(),
  createConversation: vi.fn(),
  createMessage: vi.fn(),
  createWorkspace: vi.fn(),
  getAgentContext: vi.fn(),
  getConversation: vi.fn(),
  getHealth: vi.fn(),
  getWorkspace: vi.fn(),
  listFileIndex: vi.fn(),
  listConversations: vi.fn(),
  listWorkspaces: vi.fn(),
  refreshAgentContext: vi.fn(),
  rejectAgentToolCall: vi.fn(),
  setAgentSkillEnabled: vi.fn(),
}));

function renderVibe(initialEntry: string) {
  return renderVibePage(initialEntry, <VibePage />);
}

beforeEach(() => {
  closeRunEventConnectionsForTest();
  closeWorkspaceEventConnectionsForTest();
  vi.clearAllMocks();
  MockEventSource.instances = [];
  vi.stubGlobal("EventSource", MockEventSource);
  vi.mocked(createWorkspace).mockResolvedValue(workspace);
  vi.mocked(getHealth).mockResolvedValue({ status: "ok" });
  vi.mocked(getWorkspace).mockResolvedValue(workspace);
  vi.mocked(listFileIndex).mockResolvedValue(fileIndex);
  vi.mocked(getAgentContext).mockResolvedValue(agentContext);
  vi.mocked(refreshAgentContext).mockResolvedValue(agentContext);
  vi.mocked(setAgentSkillEnabled).mockResolvedValue({
    skill: agentContext.skills[0]!,
  });
  vi.mocked(listWorkspaces).mockResolvedValue({ workspaces: [] });
  vi.mocked(listConversations).mockResolvedValue({ conversations: [] });
  vi.mocked(approveAgentToolCall).mockResolvedValue(toolCall);
  vi.mocked(cancelAgentRun).mockResolvedValue({
    ...run,
    status: "canceled",
  });
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
  vi.mocked(createMessage).mockResolvedValue({ conversation, message, run });
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
  await openExistingConversation();

  expect(await screen.findByText("Then update the UI")).toBeInTheDocument();
  expect(screen.getAllByText("Fix the failing test").length).toBeGreaterThan(0);
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
    input: '{"query":"Docker"}',
    name: "search_files",
    requiresApproval: false,
    status: "finished" as const,
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
  await openExistingConversation();

  const progress = await screen.findByText("I will inspect the workspace.");
  const tool = screen.getByText("Docker");
  const final = screen.getByText("The workspace has Docker files.");

  expect(
    progress.compareDocumentPosition(tool) & Node.DOCUMENT_POSITION_FOLLOWING,
  ).toBeTruthy();
  expect(
    tool.compareDocumentPosition(final) & Node.DOCUMENT_POSITION_FOLLOWING,
  ).toBeTruthy();
});

it("renders output snapshot after an existing assistant message", async () => {
  const progressMessage = {
    ...message,
    content: "I will inspect the workspace.",
    createdAt: "2026-05-20T00:00:01Z",
    id: "msg_2",
    role: "assistant" as const,
    runId: "run_1",
  };
  vi.mocked(listConversations).mockResolvedValue({
    conversations: [conversation],
  });
  vi.mocked(getConversation).mockResolvedValue({
    conversation,
    events: [
      {
        createdAt: "2026-05-20T00:00:00Z",
        id: "evt_old",
        payload: { runId: "run_1", text: "I will inspect the workspace." },
        runId: "run_1",
        type: "agent.delta",
        workspaceId: "ws_1",
      },
      {
        createdAt: "2026-05-20T00:00:02Z",
        id: "evt_snapshot",
        payload: { runId: "run_1", text: "Final " },
        runId: "run_1",
        type: "agent.output.snapshot",
        workspaceId: "ws_1",
      },
      {
        createdAt: "2026-05-20T00:00:03Z",
        id: "evt_stream_2",
        payload: { runId: "run_1", text: "answer" },
        runId: "run_1",
        type: "agent.delta",
        workspaceId: "ws_1",
      },
    ],
    messages: [message, progressMessage],
    runs: [{ ...run, status: "running" }],
    toolCalls: [],
  });
  renderVibe("/vibe?workspaceId=ws_1");
  await openExistingConversation();

  expect(
    await screen.findByText("I will inspect the workspace."),
  ).toBeInTheDocument();
  expect(screen.getByText("Final answer")).toBeInTheDocument();
});

it("replaces stale transient output with a snapshot before appending deltas", async () => {
  vi.mocked(listConversations).mockResolvedValue({
    conversations: [conversation],
  });
  vi.mocked(getConversation).mockResolvedValue({
    conversation,
    events: [],
    messages: [message],
    runs: [{ ...run, status: "running" }],
    toolCalls: [],
  });
  renderVibe("/vibe?workspaceId=ws_1");
  await openExistingConversation();
  await screen.findByRole("button", { name: "Stop run" });

  const source = await waitForRunEventSource();
  act(() => {
    source.emit("agent.delta", {
      createdAt: "2026-05-20T00:00:02Z",
      id: "evt_stale",
      payload: { runId: "run_1", text: "stale" },
      type: "agent.delta",
      workspaceId: "ws_1",
    });
  });
  expect(await screen.findByText("stale")).toBeInTheDocument();

  act(() => {
    source.emit("agent.output.snapshot", {
      createdAt: "2026-05-20T00:00:03Z",
      id: "evt_snapshot",
      payload: { runId: "run_1", text: "Fresh " },
      type: "agent.output.snapshot",
      workspaceId: "ws_1",
    });
    source.emit("agent.delta", {
      createdAt: "2026-05-20T00:00:04Z",
      id: "evt_delta",
      payload: { runId: "run_1", text: "text" },
      type: "agent.delta",
      workspaceId: "ws_1",
    });
  });

  expect(await screen.findByText("Fresh text")).toBeInTheDocument();
  expect(screen.queryByText("stale")).not.toBeInTheDocument();
});

it("replaces transient output with the durable assistant message", async () => {
  vi.mocked(listConversations).mockResolvedValue({
    conversations: [conversation],
  });
  vi.mocked(getConversation).mockResolvedValue({
    conversation,
    events: [],
    messages: [message],
    runs: [{ ...run, status: "running" }],
    toolCalls: [],
  });
  renderVibe("/vibe?workspaceId=ws_1");
  await openExistingConversation();
  await screen.findByRole("button", { name: "Stop run" });

  const source = await waitForRunEventSource();
  act(() => {
    source.emit("agent.delta", {
      createdAt: "2026-05-20T00:00:02Z",
      id: "evt_delta",
      payload: { runId: "run_1", text: "Durable answer" },
      type: "agent.delta",
      workspaceId: "ws_1",
    });
  });
  expect(await screen.findByText("Durable answer")).toBeInTheDocument();

  act(() => {
    source.emit("conversation.message.created", {
      createdAt: "2026-05-20T00:00:03Z",
      id: "evt_message",
      payload: {
        ...message,
        content: "Durable answer",
        createdAt: "2026-05-20T00:00:03Z",
        id: "msg_2",
        role: "assistant",
        runId: "run_1",
      },
      type: "conversation.message.created",
      workspaceId: "ws_1",
    });
  });

  await waitFor(() => {
    expect(screen.getAllByText("Durable answer")).toHaveLength(1);
  });
});

it("groups adjacent tool calls behind a collapsed activity block", async () => {
  const finishedPatch = {
    ...toolCall,
    finishedAt: "2026-05-20T00:00:02Z",
    output: '{"status":"applied"}',
    status: "finished" as const,
  };
  const finishedSearch = {
    ...toolCall,
    createdAt: "2026-05-20T00:00:01Z",
    finishedAt: "2026-05-20T00:00:02Z",
    id: "evt_2",
    input: '{"query":"docs"}',
    name: "search_files",
    output: '{"results":[]}',
    requiresApproval: false,
    sequence: 1,
    status: "finished" as const,
  };
  vi.mocked(listConversations).mockResolvedValue({
    conversations: [conversation],
  });
  vi.mocked(getConversation).mockResolvedValue({
    conversation,
    events: [],
    messages: [message],
    runs: [{ ...run, status: "done" }],
    toolCalls: [finishedPatch, finishedSearch],
  });

  renderVibe("/vibe?workspaceId=ws_1");
  await openExistingConversation();

  const groupLabel = await screen.findByText("2 tool calls");
  const group = groupLabel.closest("[data-tool-call-group]");

  expect(group).toBeInTheDocument();
  expect(group).toHaveAttribute("data-state", "closed");
  expect(screen.getAllByText("example.txt").length).toBeGreaterThan(0);
  expect(screen.getAllByText("docs").length).toBeGreaterThan(0);
});

it("opens grouped tool calls by default when a child needs attention", async () => {
  const runningSearch = {
    ...toolCall,
    createdAt: "2026-05-20T00:00:01Z",
    id: "evt_2",
    input: '{"query":"docs"}',
    name: "search_files",
    output: "{}",
    requiresApproval: false,
    sequence: 1,
    status: "running" as const,
  };
  vi.mocked(listConversations).mockResolvedValue({
    conversations: [conversation],
  });
  vi.mocked(getConversation).mockResolvedValue({
    conversation,
    events: [],
    messages: [message],
    runs: [{ ...run, status: "waiting_tool_approval" }],
    toolCalls: [toolCall, runningSearch],
  });

  renderVibe("/vibe?workspaceId=ws_1");
  await openExistingConversation();

  const groupLabel = await screen.findByText("2 tool calls");
  const group = groupLabel.closest("[data-tool-call-group]");
  const approvalItem = screen
    .getByRole("button", { name: "Approve tool" })
    .closest("[data-tool-call]");
  const runningItem = screen
    .getAllByText("docs")
    .find((item) => item.closest("[data-tool-call]"))
    ?.closest("[data-tool-call]");

  expect(group).toHaveAttribute("data-state", "open");
  expect(approvalItem).toHaveAttribute("data-state", "open");
  expect(runningItem).toHaveAttribute("data-state", "open");
});

it("renders assistant markdown code blocks with language labels and copy", async () => {
  const user = userEvent.setup();
  const writeText = vi.fn().mockResolvedValue(undefined);
  Object.defineProperty(navigator, "clipboard", {
    configurable: true,
    value: { writeText },
  });
  const markdownMessage = {
    ...message,
    content:
      "## Done\n\n- Updated **tests**\n\n1. Run `pnpm test`\n\n```ts\nconst value: string = \"ok\";\n```\n\n```go\npackage main\n```\n\n```dockerfile\nFROM node:24\n```\n\n<script>alert('x')</script>",
    id: "msg_2",
    role: "assistant" as const,
    runId: "run_1",
  };
  vi.mocked(listConversations).mockResolvedValue({
    conversations: [conversation],
  });
  vi.mocked(getConversation).mockResolvedValue({
    conversation,
    events: [],
    messages: [message, markdownMessage],
    runs: [{ ...run, status: "done", summary: markdownMessage.content }],
    toolCalls: [],
  });
  const { container } = renderVibe("/vibe?workspaceId=ws_1");
  await openExistingConversation();

  expect(
    await screen.findByRole("heading", { name: "Done" }),
  ).toBeInTheDocument();
  expect(screen.getByText("Updated")).toBeInTheDocument();
  expect(screen.getByText("pnpm test")).toBeInTheDocument();
  expect(container.querySelector("ul")).toBeInTheDocument();
  expect(container.querySelector("ol")).toBeInTheDocument();
  expect(screen.getByText("TypeScript")).toBeInTheDocument();
  expect(screen.getByText("go")).toBeInTheDocument();
  expect(screen.getByText("Dockerfile")).toBeInTheDocument();
  expect(
    container.querySelector(".pp-code-block__language img"),
  ).toBeInTheDocument();
  const dockerIcon = screen
    .getByText("Dockerfile")
    .closest(".pp-code-block__language")
    ?.querySelector("img");
  expect(dockerIcon).toHaveAttribute("src", "/icons/file_type_docker.svg");
  expect(container.querySelector(".token.keyword")).toBeInTheDocument();
  const copyButtons = screen.getAllByRole("button", { name: "Copy code" });
  expect(copyButtons[0]).toBeInTheDocument();
  await user.click(copyButtons[0] as HTMLElement);
  expect(writeText).toHaveBeenCalledWith('const value: string = "ok";');
  expect(
    await screen.findByRole("button", { name: "Copied code" }),
  ).toBeInTheDocument();
  expect(container.querySelector("script")).not.toBeInTheDocument();
});
