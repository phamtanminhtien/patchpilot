import { screen, waitFor } from "@testing-library/react";
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
  message,
  MockEventSource,
  openExistingConversation,
  renderVibePage,
  run,
  toolCall,
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

it("renders file read commands as one-line activity without output detail", async () => {
  const readToolCall = {
    ...toolCall,
    finishedAt: "2026-05-20T00:00:02Z",
    input: `{"command":"sed -n '1,160p' README.md"}`,
    name: "run_command",
    output: '{"output":"PatchPilot smoke file"}',
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

it("opens approval-required file read commands for review", async () => {
  const secretReadToolCall = {
    ...toolCall,
    input: '{"command":"cat .env"}',
    name: "run_command",
    output: "{}",
    policyReason: "Secret-like file read requires approval",
    requiresApproval: true,
    status: "waiting_approval" as const,
  };
  vi.mocked(listConversations).mockResolvedValue({
    conversations: [conversation],
  });
  vi.mocked(getConversation).mockResolvedValue({
    conversation,
    events: [],
    messages: [message],
    runs: [{ ...run, status: "waiting_tool_approval" }],
    toolCalls: [secretReadToolCall],
  });

  renderVibe("/vibe?workspaceId=ws_1");
  await openExistingConversation();

  const group = (await screen.findByText(".env")).closest("[data-tool-call]");

  expect(screen.getByText("Waiting approval")).toBeInTheDocument();
  expect(group).toHaveAttribute("data-state", "open");
  expect(screen.getByText("Approve tool")).toBeInTheDocument();
  expect(
    screen.getByText("Secret-like file read requires approval"),
  ).toBeInTheDocument();
});

it("renders use_skill tool calls with human-readable skill names", async () => {
  const user = userEvent.setup();
  const useSkillToolCall = {
    ...toolCall,
    finishedAt: "2026-05-20T00:00:02Z",
    input: '{"name":"incremental-implementation"}',
    name: "use_skill",
    output: '{"instruction":"Implement in small verified steps."}',
    requiresApproval: false,
    source: "skill" as const,
    sourceRef: "incremental-implementation",
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
    toolCalls: [useSkillToolCall],
  });

  renderVibe("/vibe?workspaceId=ws_1");
  await openExistingConversation();

  const summary = await screen.findByRole("button", {
    name: "Loaded Incremental Implementation",
  });
  await user.click(summary);

  expect(
    screen.getByText("Source: skill/Incremental Implementation"),
  ).toBeInTheDocument();
  expect(
    screen.queryByText("incremental-implementation"),
  ).not.toBeInTheDocument();
});

it("renders skill file read commands with legacy skill UI", async () => {
  const user = userEvent.setup();
  const skillReadToolCall = {
    ...toolCall,
    finishedAt: "2026-05-20T00:00:02Z",
    input:
      '{"command":"cat ~/.patchpilot/skills/incremental-implementation/SKILL.md"}',
    name: "run_command",
    output: '{"output":"Implement in small verified steps."}',
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
    toolCalls: [skillReadToolCall],
  });

  renderVibe("/vibe?workspaceId=ws_1");
  await openExistingConversation();

  const summary = await screen.findByRole("button", {
    name: "Loaded Incremental Implementation",
  });
  await user.click(summary);

  expect(
    screen.getByText("Source: skill/Incremental Implementation"),
  ).toBeInTheDocument();
  expect(
    screen.getByText("Implement in small verified steps."),
  ).toBeInTheDocument();
});

it("opens approval-required outside-workspace file reads for review", async () => {
  const outsideReadToolCall = {
    ...toolCall,
    input: '{"command":"cat /tmp/outside.txt"}',
    name: "run_command",
    output: "{}",
    policyReason: "Outside-workspace file read requires approval",
    requiresApproval: true,
    status: "waiting_approval" as const,
  };
  vi.mocked(listConversations).mockResolvedValue({
    conversations: [conversation],
  });
  vi.mocked(getConversation).mockResolvedValue({
    conversation,
    events: [],
    messages: [message],
    runs: [{ ...run, status: "waiting_tool_approval" }],
    toolCalls: [outsideReadToolCall],
  });

  renderVibe("/vibe?workspaceId=ws_1");
  await openExistingConversation();

  const group = (await screen.findByText("/tmp/outside.txt")).closest(
    "[data-tool-call]",
  );

  expect(screen.getByText("Waiting approval")).toBeInTheDocument();
  expect(group).toHaveAttribute("data-state", "open");
  expect(screen.getByText("Approve tool")).toBeInTheDocument();
  expect(
    screen.getByText("Outside-workspace file read requires approval"),
  ).toBeInTheDocument();
});
