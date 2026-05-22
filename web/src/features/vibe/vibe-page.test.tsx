import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type * as NuqsModule from "nuqs";
import type * as ReactModule from "react";
import { MemoryRouter } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ThemeProvider } from "@/app/theme";
import {
  createAgentTask,
  createWorkspace,
  getAgentTask,
  getHealth,
  getWorkspace,
  listAgentTasks,
  listWorkspaces,
} from "@/shared/api";

import { VibePage } from "./vibe-page";

const queryState = vi.hoisted(() => new Map<string, string>());

vi.mock("@/shared/api", () => ({
  apiErrorMessage: (error: unknown) =>
    error instanceof Error ? error.message : "Request failed",
  createAgentTask: vi.fn(),
  createWorkspace: vi.fn(),
  getAgentTask: vi.fn(),
  getHealth: vi.fn(),
  getWorkspace: vi.fn(),
  listAgentTasks: vi.fn(),
  listWorkspaces: vi.fn(),
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

const task = {
  createdAt: "2026-05-20T00:00:00Z",
  error: null,
  finishedAt: null,
  generatedPatch: "",
  id: "task_1",
  model: "gpt-5.4-mini" as const,
  plan: "",
  prompt: "Fix the failing test",
  reasoningEffort: "high" as const,
  startedAt: null,
  status: "queued" as const,
  summary: "",
  updatedAt: "2026-05-20T00:00:00Z",
  workspaceId: "ws_1",
};

describe("VibePage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(createWorkspace).mockResolvedValue(workspace);
    vi.mocked(getHealth).mockResolvedValue({ status: "ok" });
    vi.mocked(getWorkspace).mockResolvedValue(workspace);
    vi.mocked(listWorkspaces).mockResolvedValue({ workspaces: [] });
    vi.mocked(listAgentTasks).mockResolvedValue({ tasks: [] });
    vi.mocked(getAgentTask).mockResolvedValue({
      events: [],
      patches: [],
      task,
      toolCalls: [],
    });
    vi.mocked(createAgentTask).mockResolvedValue(task);
  });

  it("creates an agent task with selected model and reasoning effort", async () => {
    const user = userEvent.setup();
    renderVibe("/vibe?workspaceId=ws_1");

    const promptInput = await screen.findByLabelText("Ask AI");
    await waitFor(() => {
      expect(promptInput).toBeEnabled();
    });
    await user.type(promptInput, "Fix the failing test");
    await user.selectOptions(screen.getByLabelText(/Model/), "gpt-5.4-mini");
    await user.selectOptions(screen.getByLabelText(/Reasoning/), "high");
    const startButton = screen.getByRole("button", { name: "Start task" });
    await waitFor(() => {
      expect(startButton).toBeEnabled();
    });
    await user.click(startButton);

    await waitFor(() => {
      expect(createAgentTask).toHaveBeenCalledWith("ws_1", {
        model: "gpt-5.4-mini",
        prompt: "Fix the failing test",
        reasoningEffort: "high",
      });
    });
    expect(await screen.findAllByText("Fix the failing test")).toHaveLength(2);
  });

  it("keeps the task list in a bounded scroll region", async () => {
    vi.mocked(listAgentTasks).mockResolvedValue({ tasks: [task] });
    renderVibe("/vibe?workspaceId=ws_1");

    const taskList = await screen.findByRole("region", {
      name: "Agent tasks",
    });

    expect(taskList).toHaveClass("min-h-0", "min-w-0", "overflow-auto");
    expect(taskList.parentElement).toHaveClass(
      "grid",
      "grid-rows-[auto_minmax(0,1fr)]",
      "min-w-0",
      "overflow-hidden",
    );
    expect(taskList.parentElement?.parentElement).toHaveClass(
      "grid-rows-[minmax(0,16rem)_minmax(0,1fr)]",
      "min-w-0",
      "overflow-hidden",
      "lg:grid-rows-1",
    );
  });
});

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
