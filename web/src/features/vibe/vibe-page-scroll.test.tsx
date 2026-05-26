import {
  act,
  fireEvent,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
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
  doneRun,
  message,
  MockEventSource,
  openExistingConversation,
  renderVibePage,
  run,
  setScrollMetrics,
  submitPrompt,
  toolCall,
  waitForRunEventSource,
  waitForWorkspaceEventSource,
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
  vi.mocked(getConversation).mockResolvedValue({
    conversation,
    events: [],
    messages: [message],
    runs: [doneRun],
    toolCalls: [],
  });
  vi.mocked(createMessage).mockResolvedValue({
    conversation,
    message: {
      ...message,
      content: "Apply the fix",
      createdAt: "2026-05-20T00:00:02Z",
      id: "msg_2",
      runId: "run_2",
    },
    run: {
      ...doneRun,
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

it("keeps following when rapid activity pushes the bottom away before the effect runs", async () => {
  const scrollIntoView = vi.fn();
  Object.defineProperty(Element.prototype, "scrollIntoView", {
    configurable: true,
    value: scrollIntoView,
  });
  vi.mocked(listConversations).mockResolvedValue({
    conversations: [conversation],
  });
  vi.mocked(getConversation).mockResolvedValue({
    conversation,
    events: [],
    messages: [message],
    runs: [doneRun],
    toolCalls: [],
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

  const source = await waitForWorkspaceEventSource();
  setScrollMetrics(timeline, {
    clientHeight: 320,
    scrollHeight: 1240,
    scrollTop: 576,
  });
  act(() => {
    source.emit("conversation.message.created", {
      createdAt: "2026-05-20T00:00:02Z",
      id: "evt_message_2",
      payload: {
        ...message,
        content: "First rapid update",
        createdAt: "2026-05-20T00:00:02Z",
        id: "msg_2",
        role: "assistant",
        runId: "run_1",
      },
      type: "conversation.message.created",
      workspaceId: "ws_1",
    });
    source.emit("conversation.message.created", {
      createdAt: "2026-05-20T00:00:03Z",
      id: "evt_message_3",
      payload: {
        ...message,
        content: "Second rapid update",
        createdAt: "2026-05-20T00:00:03Z",
        id: "msg_3",
        role: "assistant",
        runId: "run_1",
      },
      type: "conversation.message.created",
      workspaceId: "ws_1",
    });
  });

  expect(await screen.findByText("Second rapid update")).toBeInTheDocument();
  await waitFor(() => {
    expect(scrollIntoView).toHaveBeenCalled();
  });
  expect(
    screen.queryByRole("button", { name: "Jump to latest" }),
  ).not.toBeInTheDocument();
});

it("auto-scrolls as streamed assistant text grows while following", async () => {
  const scrollIntoView = vi.fn();
  Object.defineProperty(Element.prototype, "scrollIntoView", {
    configurable: true,
    value: scrollIntoView,
  });
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

  const timeline = await screen.findByRole("region", {
    name: "Conversation timeline",
  });
  await waitFor(() => {
    expect(scrollIntoView).toHaveBeenCalled();
  });
  setScrollMetrics(timeline, {
    clientHeight: 320,
    scrollHeight: 960,
    scrollTop: 576,
  });
  fireEvent.scroll(timeline);

  const source = await waitForRunEventSource();
  act(() => {
    source.emit("agent.delta", {
      createdAt: "2026-05-20T00:00:02Z",
      id: "evt_delta_1",
      payload: { runId: "run_1", text: "Streaming" },
      type: "agent.delta",
      workspaceId: "ws_1",
    });
  });
  expect(await screen.findByText("Streaming")).toBeInTheDocument();

  scrollIntoView.mockClear();
  setScrollMetrics(timeline, {
    clientHeight: 320,
    scrollHeight: 1240,
    scrollTop: 576,
  });
  act(() => {
    source.emit("agent.delta", {
      createdAt: "2026-05-20T00:00:03Z",
      id: "evt_delta_2",
      payload: { runId: "run_1", text: " text" },
      type: "agent.delta",
      workspaceId: "ws_1",
    });
  });

  expect(await screen.findByText("Streaming text")).toBeInTheDocument();
  await waitFor(() => {
    expect(scrollIntoView).toHaveBeenCalled();
  });
  expect(
    screen.queryByRole("button", { name: "Jump to latest" }),
  ).not.toBeInTheDocument();
});

it("does not auto-scroll streamed assistant text when reading older activity", async () => {
  const scrollIntoView = vi.fn();
  Object.defineProperty(Element.prototype, "scrollIntoView", {
    configurable: true,
    value: scrollIntoView,
  });
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

  const timeline = await screen.findByRole("region", {
    name: "Conversation timeline",
  });
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

  const source = await waitForRunEventSource();
  act(() => {
    source.emit("agent.delta", {
      createdAt: "2026-05-20T00:00:02Z",
      id: "evt_delta",
      payload: { runId: "run_1", text: "Offscreen stream" },
      type: "agent.delta",
      workspaceId: "ws_1",
    });
  });

  expect(await screen.findByText("Offscreen stream")).toBeInTheDocument();
  await waitFor(() => {
    expect(
      screen.getByRole("button", { name: "Jump to latest" }),
    ).toBeInTheDocument();
  });
  expect(scrollIntoView).not.toHaveBeenCalled();
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
  vi.mocked(getConversation).mockResolvedValue({
    conversation,
    events: [],
    messages: [message],
    runs: [doneRun],
    toolCalls: [],
  });
  vi.mocked(createMessage).mockResolvedValue({
    conversation,
    message: {
      ...message,
      content: "Investigate logs",
      createdAt: "2026-05-20T00:00:02Z",
      id: "msg_2",
      runId: "run_2",
    },
    run: {
      ...doneRun,
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
  vi.mocked(getConversation).mockResolvedValue({
    conversation,
    events: [],
    messages: [message],
    runs: [doneRun],
    toolCalls: [],
  });
  vi.mocked(createMessage)
    .mockResolvedValueOnce({
      conversation,
      message: {
        ...message,
        content: "First follow-up",
        createdAt: "2026-05-20T00:00:02Z",
        id: "msg_2",
        runId: "run_2",
      },
      run: {
        ...doneRun,
        createdAt: "2026-05-20T00:00:02Z",
        id: "run_2",
        triggerMessageId: "msg_2",
        updatedAt: "2026-05-20T00:00:02Z",
      },
    })
    .mockResolvedValueOnce({
      conversation,
      message: {
        ...message,
        content: "Second follow-up",
        createdAt: "2026-05-20T00:00:03Z",
        id: "msg_3",
        runId: "run_3",
      },
      run: {
        ...doneRun,
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
