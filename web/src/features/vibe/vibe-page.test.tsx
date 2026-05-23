import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
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

  it("starts on a new conversation instead of opening the newest existing one", async () => {
    vi.mocked(listConversations).mockResolvedValue({
      conversations: [conversation],
    });
    renderVibe("/vibe?workspaceId=ws_1");

    const promptInput = await screen.findByLabelText("Ask AI");
    await waitFor(() => {
      expect(promptInput).toBeEnabled();
    });
    expect(await screen.findByText("Fix the failing test")).toBeInTheDocument();
    const timeline = screen.getByRole("region", {
      name: "Conversation timeline",
    });
    expect(
      within(timeline).queryByText("Fix the failing test"),
    ).not.toBeInTheDocument();
    expect(getConversation).not.toHaveBeenCalled();
  });

  it("keeps the run list in a bounded scroll region", async () => {
    vi.mocked(listConversations).mockResolvedValue({
      conversations: [conversation],
    });
    renderVibe("/vibe?workspaceId=ws_1");
    await openExistingConversation();

    const taskList = await screen.findByRole("region", {
      name: "Agent conversations",
    });
    const timeline = screen.getByRole("region", {
      name: "Conversation timeline",
    });

    expect(taskList).toHaveClass("min-h-0", "min-w-0", "overflow-auto");
    expect(taskList.parentElement).toHaveClass("grid", "overflow-hidden");
    expect(timeline).toHaveClass("overflow-auto");
  });

  it("auto-scrolls to the latest activity when the timeline is near the bottom", async () => {
    const user = userEvent.setup();
    const scrollIntoView = vi.fn();
    Object.defineProperty(Element.prototype, "scrollIntoView", {
      configurable: true,
      value: scrollIntoView,
    });
    vi.mocked(listConversations).mockResolvedValue({
      conversations: [conversation],
    });
    vi.mocked(createMessage).mockResolvedValue({
      message: {
        ...message,
        content: "Apply the fix",
        createdAt: "2026-05-20T00:00:02Z",
        id: "msg_2",
        runId: "run_2",
      },
      run: {
        ...run,
        createdAt: "2026-05-20T00:00:02Z",
        id: "run_2",
        triggerMessageId: "msg_2",
        updatedAt: "2026-05-20T00:00:02Z",
      },
    });
    renderVibe("/vibe?workspaceId=ws_1");
    await openExistingConversation();

    const timeline = await screen.findByRole("region", {
      name: "Conversation timeline",
    });
    await within(timeline).findByText("Fix the failing test");
    await waitFor(() => {
      expect(scrollIntoView).toHaveBeenCalled();
    });
    setScrollMetrics(timeline, {
      clientHeight: 320,
      scrollHeight: 960,
      scrollTop: 576,
    });
    fireEvent.scroll(timeline);
    scrollIntoView.mockClear();

    await submitPrompt(user, "Apply the fix");

    await waitFor(() => {
      expect(scrollIntoView).toHaveBeenCalled();
    });
    expect(
      screen.queryByRole("button", { name: "Jump to latest" }),
    ).not.toBeInTheDocument();
  });

  it("pauses auto-scroll and shows a jump control when newer activity arrives off-bottom", async () => {
    const user = userEvent.setup();
    const scrollIntoView = vi.fn();
    Object.defineProperty(Element.prototype, "scrollIntoView", {
      configurable: true,
      value: scrollIntoView,
    });
    vi.mocked(listConversations).mockResolvedValue({
      conversations: [conversation],
    });
    vi.mocked(createMessage).mockResolvedValue({
      message: {
        ...message,
        content: "Investigate logs",
        createdAt: "2026-05-20T00:00:02Z",
        id: "msg_2",
        runId: "run_2",
      },
      run: {
        ...run,
        createdAt: "2026-05-20T00:00:02Z",
        id: "run_2",
        triggerMessageId: "msg_2",
        updatedAt: "2026-05-20T00:00:02Z",
      },
    });
    renderVibe("/vibe?workspaceId=ws_1");
    await openExistingConversation();

    const timeline = await screen.findByRole("region", {
      name: "Conversation timeline",
    });
    await within(timeline).findByText("Fix the failing test");
    await waitFor(() => {
      expect(scrollIntoView).toHaveBeenCalled();
    });
    setScrollMetrics(timeline, {
      clientHeight: 320,
      scrollHeight: 960,
      scrollTop: 120,
    });
    fireEvent.scroll(timeline);
    scrollIntoView.mockClear();

    expect(
      screen.queryByRole("button", { name: "Jump to latest" }),
    ).not.toBeInTheDocument();

    await submitPrompt(user, "Investigate logs");

    await waitFor(() => {
      expect(
        screen.getByRole("button", { name: "Jump to latest" }),
      ).toBeInTheDocument();
    });
    expect(scrollIntoView).not.toHaveBeenCalled();
  });

  it("jumps to the latest activity and resumes follow mode", async () => {
    const user = userEvent.setup();
    const scrollIntoView = vi.fn();
    Object.defineProperty(Element.prototype, "scrollIntoView", {
      configurable: true,
      value: scrollIntoView,
    });
    vi.mocked(listConversations).mockResolvedValue({
      conversations: [conversation],
    });
    vi.mocked(createMessage)
      .mockResolvedValueOnce({
        message: {
          ...message,
          content: "First follow-up",
          createdAt: "2026-05-20T00:00:02Z",
          id: "msg_2",
          runId: "run_2",
        },
        run: {
          ...run,
          createdAt: "2026-05-20T00:00:02Z",
          id: "run_2",
          triggerMessageId: "msg_2",
          updatedAt: "2026-05-20T00:00:02Z",
        },
      })
      .mockResolvedValueOnce({
        message: {
          ...message,
          content: "Second follow-up",
          createdAt: "2026-05-20T00:00:03Z",
          id: "msg_3",
          runId: "run_3",
        },
        run: {
          ...run,
          createdAt: "2026-05-20T00:00:03Z",
          id: "run_3",
          triggerMessageId: "msg_3",
          updatedAt: "2026-05-20T00:00:03Z",
        },
      });
    renderVibe("/vibe?workspaceId=ws_1");
    await openExistingConversation();

    const timeline = await screen.findByRole("region", {
      name: "Conversation timeline",
    });
    await within(timeline).findByText("Fix the failing test");
    await waitFor(() => {
      expect(scrollIntoView).toHaveBeenCalled();
    });
    setScrollMetrics(timeline, {
      clientHeight: 320,
      scrollHeight: 960,
      scrollTop: 120,
    });
    fireEvent.scroll(timeline);
    scrollIntoView.mockClear();

    await submitPrompt(user, "First follow-up");

    const jumpButton = await screen.findByRole("button", {
      name: "Jump to latest",
    });
    await user.click(jumpButton);

    await waitFor(() => {
      expect(scrollIntoView).toHaveBeenCalled();
    });
    expect(
      screen.queryByRole("button", { name: "Jump to latest" }),
    ).not.toBeInTheDocument();

    setScrollMetrics(timeline, {
      clientHeight: 320,
      scrollHeight: 1080,
      scrollTop: 760,
    });
    fireEvent.scroll(timeline);
    scrollIntoView.mockClear();

    await submitPrompt(user, "Second follow-up");

    await waitFor(() => {
      expect(scrollIntoView).toHaveBeenCalled();
    });
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
    await openExistingConversation();

    expect(await screen.findByText("example.txt")).toBeInTheDocument();
    expect(screen.getByText("Waiting approval")).toBeInTheDocument();
    expect(screen.queryByText("apply_patch")).not.toBeInTheDocument();
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
    await openExistingConversation();

    expect(await screen.findByText("example.txt")).toBeInTheDocument();
    expect(
      screen.queryByRole("group", { name: "agent.approval_required" }),
    ).not.toBeInTheDocument();
  });

  it("renders read file tool calls as one-line activity without output detail", async () => {
    const readToolCall = {
      ...toolCall,
      finishedAt: "2026-05-20T00:00:02Z",
      input: '{"path":"README.md"}',
      name: "read_file",
      output: '{"content":"PatchPilot smoke file"}',
      requiresApproval: false,
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
      toolCalls: [readToolCall],
    });

    renderVibe("/vibe?workspaceId=ws_1");
    await openExistingConversation();

    const path = await screen.findByText("README.md");

    expect(screen.getByText("Read")).toBeInTheDocument();
    expect(path.closest("[data-tool-call]")).toBeNull();
    expect(screen.queryByText("PatchPilot smoke file")).not.toBeInTheDocument();
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

  return render(
    <QueryClientProvider client={queryClient}>
      <ThemeProvider>
        <MemoryRouter initialEntries={[initialEntry]}>
          <VibePage />
        </MemoryRouter>
      </ThemeProvider>
    </QueryClientProvider>,
  );
}

async function openExistingConversation() {
  const title = await screen.findByText(conversation.title);
  const button = title.closest("button");
  if (button === null) {
    throw new Error("Conversation button was not found");
  }
  fireEvent.click(button);
}

async function submitPrompt(
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

function setScrollMetrics(
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
