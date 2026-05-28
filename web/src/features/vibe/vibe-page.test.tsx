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
  getWorkspacePermissions,
  listConversations,
  listFileIndex,
  listWorkspaces,
  patchWorkspacePermissions,
  refreshAgentContext,
  rejectAgentToolCall,
  setAgentSkillEnabled,
} from "@/shared/api";
import {
  closeRunEventConnectionsForTest,
  closeWorkspaceEventConnectionsForTest,
} from "@/shared/events";

import { timeAgo } from "./lib/time";
import { VibePage } from "./vibe-page";
import {
  agentContext,
  conversation,
  doneRun,
  fileIndex,
  message,
  MockEventSource,
  openExistingConversation,
  queryStateValue,
  renderVibePage,
  run,
  searchConversation,
  submitPrompt,
  toolCall,
  waitForWorkspaceEventSource,
  workspace,
  workspacePermissions,
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
  getWorkspacePermissions: vi.fn(),
  listFileIndex: vi.fn(),
  listConversations: vi.fn(),
  listWorkspaces: vi.fn(),
  patchWorkspacePermissions: vi.fn(),
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
  vi.mocked(getWorkspacePermissions).mockResolvedValue(workspacePermissions);
  vi.mocked(patchWorkspacePermissions).mockResolvedValue(workspacePermissions);
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
    expect(createConversation).toHaveBeenCalledWith("ws_1", {});
    expect(createMessage).toHaveBeenCalledWith("ws_1", "conv_1", {
      content: "Fix the failing test",
      model: "gpt-5.4-mini",
      reasoningEffort: "high",
    });
  });
  expect(await screen.findAllByText("Fix the failing test")).toHaveLength(3);
});

it("submits with enter and inserts a line break with ctrl enter", async () => {
  const user = userEvent.setup();
  renderVibe("/vibe?workspaceId=ws_1");

  const promptInput = await screen.findByLabelText("Ask AI");
  await waitFor(() => {
    expect(promptInput).toBeEnabled();
  });
  await user.type(promptInput, "First line");
  await user.keyboard("{Control>}{Enter}{/Control}");
  await user.type(promptInput, "Second line");
  expect(createMessage).not.toHaveBeenCalled();

  await user.keyboard("{Enter}");

  await waitFor(() => {
    expect(createMessage).toHaveBeenCalledWith("ws_1", "conv_1", {
      content: "First line\nSecond line",
      model: "gpt-5.5",
      reasoningEffort: "medium",
    });
  });
});

it("configures workspace permissions from the composer", async () => {
  const user = userEvent.setup();
  vi.mocked(patchWorkspacePermissions).mockImplementation(
    (_workspaceId, permissions) =>
      Promise.resolve({
        ...workspacePermissions,
        ...permissions,
      }),
  );
  renderVibe("/vibe?workspaceId=ws_1");

  await user.click(
    await screen.findByRole("button", { name: /Balanced permissions/i }),
  );

  expect(screen.getByRole("group", { name: "Permission mode" })).toBeVisible();
  expect(screen.getByText("apply_patch needs approval")).toBeInTheDocument();
  expect(
    screen.getByText(
      "safe commands auto-run; confirmation commands need approval",
    ),
  ).toBeInTheDocument();
  expect(screen.getByText("MCP tools need approval")).toBeInTheDocument();

  await user.click(screen.getByRole("button", { name: "Autonomous" }));
  await waitFor(() => {
    expect(patchWorkspacePermissions).toHaveBeenCalledWith("ws_1", {
      editFiles: true,
      gitOperations: true,
      mode: "autonomous",
      runCommands: true,
    });
  });
  expect(
    screen.getByRole("button", { name: /Autonomous permissions/i }),
  ).toBeInTheDocument();

  await user.click(screen.getByRole("switch", { name: "Git operations" }));
  await waitFor(() => {
    expect(patchWorkspacePermissions).toHaveBeenLastCalledWith("ws_1", {
      gitOperations: false,
    });
  });
  expect(screen.getByText("git commands are blocked")).toBeInTheDocument();
});

it("shows workspace permission loading and save errors", async () => {
  const user = userEvent.setup();
  vi.mocked(getWorkspacePermissions).mockResolvedValue(workspacePermissions);
  vi.mocked(patchWorkspacePermissions).mockRejectedValue(
    new Error("Save failed"),
  );
  renderVibe("/vibe?workspaceId=ws_1");

  await user.click(
    await screen.findByRole("button", { name: /Balanced permissions/i }),
  );
  await user.click(screen.getByRole("button", { name: "Safe" }));

  expect(await screen.findByText("Save failed")).toBeInTheDocument();
});

it("inserts slash skill links without submitting", async () => {
  const user = userEvent.setup();
  renderVibe("/vibe?workspaceId=ws_1");

  const promptInput = await screen.findByLabelText("Ask AI");
  await waitFor(() => {
    expect(promptInput).toBeEnabled();
  });
  await user.type(promptInput, "/bro");

  const slashList = await screen.findByRole("listbox", {
    name: "Slash suggestions",
  });
  expect(within(slashList).getByText("Skills")).toBeInTheDocument();
  await user.click(screen.getByRole("option", { name: /Browser/i }));

  expect(screen.getByText("Browser")).toBeInTheDocument();
  expect(composerToken("Browser")).toHaveAttribute(
    "data-markdown",
    "[$browser](patchpilot/browser)",
  );
  expect(createMessage).not.toHaveBeenCalled();
});

it("submits composer tokens as plain markdown content", async () => {
  const user = userEvent.setup();
  renderVibe("/vibe?workspaceId=ws_1");

  const promptInput = await screen.findByLabelText("Ask AI");
  await waitFor(() => {
    expect(promptInput).toBeEnabled();
  });
  await user.type(promptInput, "/bro");
  await user.click(await screen.findByRole("option", { name: /Browser/i }));
  await user.click(screen.getByRole("button", { name: "Start run" }));

  await waitFor(() => {
    expect(createMessage).toHaveBeenCalledWith("ws_1", "conv_1", {
      content: "[$browser](patchpilot/browser)",
      model: "gpt-5.5",
      reasoningEffort: "medium",
    });
  });
});

it("shows invalid slash skills as disabled", async () => {
  const user = userEvent.setup();
  renderVibe("/vibe?workspaceId=ws_1");

  const promptInput = await screen.findByLabelText("Ask AI");
  await waitFor(() => {
    expect(promptInput).toBeEnabled();
  });
  await user.type(promptInput, "/broken");

  const brokenSkill = await screen.findByRole("option", {
    name: /Broken Skill/i,
  });
  expect(brokenSkill).toBeDisabled();
});

it("deletes composer tokens as atomic units", async () => {
  const user = userEvent.setup();
  renderVibe("/vibe?workspaceId=ws_1");

  const promptInput = await screen.findByLabelText("Ask AI");
  await waitFor(() => {
    expect(promptInput).toBeEnabled();
  });
  await user.type(promptInput, "/bro");
  await user.click(await screen.findByRole("option", { name: /Browser/i }));

  expect(composerToken("Browser")).toBeInTheDocument();
  await user.keyboard("{Backspace}");
  expect(screen.queryByText("Browser")).not.toBeInTheDocument();
});

it("inserts grouped mention links for skills folders and files", async () => {
  const user = userEvent.setup();
  renderVibe("/vibe?workspaceId=ws_1");

  const promptInput = await screen.findByLabelText("Ask AI");
  await waitFor(() => {
    expect(promptInput).toBeEnabled();
  });
  await user.type(promptInput, "@");

  const mentionList = await screen.findByRole("listbox", {
    name: "Mention suggestions",
  });
  expect(within(mentionList).getByText("Skills")).toBeInTheDocument();
  expect(within(mentionList).queryByText("Folders")).not.toBeInTheDocument();
  expect(within(mentionList).queryByText("Files")).not.toBeInTheDocument();

  await user.type(promptInput, "docs");
  expect(await within(mentionList).findByText("Folders")).toBeInTheDocument();
  expect(within(mentionList).getByText("Files")).toBeInTheDocument();
  await user.click(optionButton(within(mentionList).getByText("docs")));
  expect(composerToken("docs/")).toHaveAttribute(
    "data-markdown",
    "[docs](docs/)",
  );

  await user.clear(promptInput);
  await user.type(promptInput, "@composer");
  await user.click(optionButton(await screen.findByText("composer.tsx")));
  expect(composerToken("composer.tsx")).toHaveAttribute(
    "data-markdown",
    "[composer.tsx](web/src/features/vibe/components/composer.tsx)",
  );

  await user.clear(promptInput);
  await user.type(promptInput, "@browser");
  await user.click(screen.getByRole("option", { name: /Browser/i }));
  expect(screen.getByText("Browser")).toBeInTheDocument();
  expect(composerToken("Browser")).toHaveAttribute(
    "data-markdown",
    "[$browser](patchpilot/browser)",
  );
});

it("accepts composer suggestions with the keyboard and closes with escape", async () => {
  const user = userEvent.setup();
  const scrollIntoView = vi.fn();
  Element.prototype.scrollIntoView = scrollIntoView;
  renderVibe("/vibe?workspaceId=ws_1");

  const promptInput = await screen.findByLabelText("Ask AI");
  await waitFor(() => {
    expect(promptInput).toBeEnabled();
  });
  await user.type(promptInput, "@docs");
  await screen.findByRole("listbox", { name: "Mention suggestions" });

  await user.keyboard("{Enter}");
  expect(composerToken("docs/")).toHaveAttribute(
    "data-markdown",
    "[docs](docs/)",
  );

  await user.clear(promptInput);
  await user.type(promptInput, "/");
  await screen.findByRole("listbox", { name: "Slash suggestions" });
  await user.keyboard("{ArrowDown}");
  expect(scrollIntoView).toHaveBeenCalled();
  await user.keyboard("{Escape}");
  expect(
    screen.queryByRole("listbox", { name: "Slash suggestions" }),
  ).not.toBeInTheDocument();
});

function optionButton(element: HTMLElement) {
  const button = element.closest("button");
  if (button === null) {
    throw new Error("Suggestion button was not found");
  }
  return button;
}

function composerToken(label: string) {
  const token = screen.getByText(label).closest("[data-composer-link-token]");
  if (token === null) {
    throw new Error(`Composer token was not found for ${label}`);
  }
  return token;
}

it("updates conversation title from workspace events", async () => {
  vi.mocked(listConversations).mockResolvedValue({
    conversations: [{ ...conversation, title: "New conversation" }],
  });
  vi.mocked(getConversation).mockResolvedValue({
    conversation: { ...conversation, title: "New conversation" },
    events: [],
    messages: [message],
    runs: [run],
    toolCalls: [],
  });
  renderVibe("/vibe?workspaceId=ws_1&conversationId=conv_1");

  await waitFor(() => {
    expect(screen.getAllByText("New conversation").length).toBeGreaterThan(0);
  });
  const source = await waitForWorkspaceEventSource();
  act(() => {
    source.emit("conversation.updated", {
      createdAt: "2026-05-20T00:00:02Z",
      id: "evt_title",
      payload: {
        ...conversation,
        title: "Investigate flaky tests",
        updatedAt: "2026-05-20T00:00:02Z",
      },
      type: "conversation.updated",
      workspaceId: "ws_1",
    });
  });

  await waitFor(() => {
    expect(
      screen.getAllByText("Investigate flaky tests").length,
    ).toBeGreaterThan(0);
  });
});

it("shows stop in the send position for an active run", async () => {
  const user = userEvent.setup();
  vi.mocked(listConversations).mockResolvedValue({
    conversations: [conversation],
  });
  renderVibe("/vibe?workspaceId=ws_1");
  await openExistingConversation();

  const stopButton = await screen.findByRole("button", {
    name: "Stop run",
  });
  await user.click(stopButton);

  await waitFor(() => {
    expect(cancelAgentRun).toHaveBeenCalledWith("ws_1", "conv_1", "run_1");
  });
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

it("opens the conversation from URL state", async () => {
  vi.mocked(listConversations).mockResolvedValue({
    conversations: [conversation],
  });
  renderVibe("/vibe?workspaceId=ws_1&conversationId=conv_1");

  const timeline = await screen.findByRole("region", {
    name: "Conversation timeline",
  });
  expect(
    await within(timeline).findByText("Fix the failing test"),
  ).toBeInTheDocument();
  await waitFor(() => {
    expect(getConversation).toHaveBeenCalledWith("ws_1", "conv_1");
  });
});

it("stores selected conversations in URL state", async () => {
  vi.mocked(listConversations).mockResolvedValue({
    conversations: [conversation],
  });
  renderVibe("/vibe?workspaceId=ws_1");

  await openExistingConversation();

  await waitFor(() => {
    expect(queryStateValue("conversationId")).toBe("conv_1");
  });
});

it("shows relative time for idle conversations in the sidebar", async () => {
  vi.mocked(listConversations).mockResolvedValue({
    conversations: [conversation],
  });

  renderVibe("/vibe?workspaceId=ws_1");

  expect(
    await screen.findByText(timeAgo(conversation.lastMessageAt)),
  ).toBeInTheDocument();
  expect(
    screen.queryByLabelText("Conversation run in progress"),
  ).not.toBeInTheDocument();
});

it("shows a loading spinner in the sidebar when a conversation run is active", async () => {
  const user = userEvent.setup();
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
  await submitPrompt(user, "Continue the fix");

  expect(
    await screen.findByLabelText("Conversation run in progress"),
  ).toBeInTheDocument();
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

it("opens conversation search from the sidebar and searches by title", async () => {
  const user = userEvent.setup();
  vi.mocked(listConversations).mockImplementation((_workspaceId, params) =>
    Promise.resolve({
      conversations: params?.q ? [searchConversation] : [conversation],
    }),
  );
  renderVibe("/vibe?workspaceId=ws_1");

  await user.click(await screen.findByRole("button", { name: "Search" }));

  expect(
    await screen.findByRole("dialog", { name: "Search conversations" }),
  ).toBeInTheDocument();
  expect(screen.getByText("Search by conversation title.")).toBeInTheDocument();

  fireEvent.change(screen.getByPlaceholderText("Search conversations"), {
    target: { value: "search" },
  });

  expect(await screen.findByText(searchConversation.title)).toBeInTheDocument();
  await waitFor(() => {
    expect(listConversations).toHaveBeenCalledWith("ws_1", {
      limit: 50,
      q: "search",
    });
  });
});

it("selects a searched conversation and closes the search dialog", async () => {
  const user = userEvent.setup();
  vi.mocked(listConversations).mockImplementation((_workspaceId, params) =>
    Promise.resolve({
      conversations: params?.q ? [searchConversation] : [conversation],
    }),
  );
  vi.mocked(getConversation).mockResolvedValue({
    conversation: searchConversation,
    events: [],
    messages: [{ ...message, conversationId: searchConversation.id }],
    runs: [],
    toolCalls: [],
  });
  renderVibe("/vibe?workspaceId=ws_1");

  await user.click(await screen.findByRole("button", { name: "Search" }));
  fireEvent.change(screen.getByPlaceholderText("Search conversations"), {
    target: { value: "search" },
  });
  await user.click(await screen.findByText(searchConversation.title));

  await waitFor(() => {
    expect(
      screen.queryByRole("dialog", { name: "Search conversations" }),
    ).not.toBeInTheDocument();
    expect(getConversation).toHaveBeenCalledWith("ws_1", "conv_2");
  });
});

it("opens conversation search from the mobile header trigger", async () => {
  const user = userEvent.setup();
  vi.mocked(listConversations).mockResolvedValue({
    conversations: [conversation],
  });
  renderVibe("/vibe?workspaceId=ws_1");

  await user.click(
    await screen.findByRole("button", { name: "Search conversations" }),
  );

  expect(
    await screen.findByRole("dialog", { name: "Search conversations" }),
  ).toBeInTheDocument();
});

it("opens skills from the sidebar and shows metadata with body detail", async () => {
  const user = userEvent.setup();
  renderVibe("/vibe?workspaceId=ws_1");

  await user.click(await screen.findByRole("button", { name: "Skills" }));

  const dialog = await screen.findByRole("dialog", { name: "Skills" });
  expect(within(dialog).getAllByText("Browser")[0]).toBeInTheDocument();
  expect(
    within(dialog).getAllByText("Browser automation for local targets.")[0],
  ).toBeInTheDocument();
  expect(
    within(dialog).getByText("Use the in-app browser to inspect local UI."),
  ).toBeInTheDocument();
  expect(
    within(dialog).queryByText("patchpilot/browser"),
  ).not.toBeInTheDocument();
});

it("shows invalid skill warnings and toggles skills by internal key", async () => {
  const user = userEvent.setup();
  renderVibe("/vibe?workspaceId=ws_1");

  await user.click(await screen.findByRole("button", { name: "Skills" }));
  const dialog = await screen.findByRole("dialog", { name: "Skills" });
  await user.click(
    within(dialog).getByRole("switch", { name: "Toggle Browser" }),
  );

  await waitFor(() => {
    expect(setAgentSkillEnabled).toHaveBeenCalledWith("ws_1", "browser", false);
  });
  expect(within(dialog).getByText("Broken Skill")).toBeInTheDocument();
  expect(
    within(dialog).getAllByText(
      "SKILL.md frontmatter requires a non-empty description.",
    )[0],
  ).toBeInTheDocument();
});

it("shows loading, empty, and error states while searching conversations", async () => {
  const user = userEvent.setup();
  vi.mocked(listConversations).mockImplementation((_workspaceId, params) => {
    if (params?.q === "loading") {
      return new Promise(() => {});
    }
    if (params?.q === "error") {
      return Promise.reject(new Error("Search failed"));
    }
    return Promise.resolve({ conversations: [] });
  });
  renderVibe("/vibe?workspaceId=ws_1");

  await user.click(await screen.findByRole("button", { name: "Search" }));
  fireEvent.change(screen.getByPlaceholderText("Search conversations"), {
    target: { value: "missing" },
  });
  expect(
    await screen.findByText("No matching conversations."),
  ).toBeInTheDocument();

  fireEvent.change(screen.getByPlaceholderText("Search conversations"), {
    target: { value: "loading" },
  });
  expect(
    await screen.findByText("Searching conversations"),
  ).toBeInTheDocument();

  fireEvent.change(screen.getByPlaceholderText("Search conversations"), {
    target: { value: "error" },
  });
  expect(await screen.findByText("Search failed")).toBeInTheDocument();
});
