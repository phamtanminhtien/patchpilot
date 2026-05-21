import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type * as NuqsModule from "nuqs";
import type * as ReactModule from "react";
import { MemoryRouter } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ThemeProvider } from "@/app/theme";
import {
  createWorkspace,
  getGitDiff,
  getGitStatus,
  getHealth,
  getWorkspace,
  listFiles,
  listWorkspaces,
  queueCommand,
  readFile,
} from "@/shared/api";

import { WorkspacePage } from "./workspace-page";

const queryState = vi.hoisted(() => new Map<string, string>());

vi.mock("@/shared/api", () => ({
  apiErrorMessage: (error: unknown) =>
    error instanceof Error ? error.message : "Request failed",
  createWorkspace: vi.fn(),
  getHealth: vi.fn(),
  getGitDiff: vi.fn(),
  getGitStatus: vi.fn(),
  getWorkspace: vi.fn(),
  listFiles: vi.fn(),
  listWorkspaces: vi.fn(),
  queueCommand: vi.fn(),
  readFile: vi.fn(),
}));

vi.mock("nuqs", async () => {
  const React = await vi.importActual<typeof ReactModule>("react");
  const actual = await vi.importActual<typeof NuqsModule>("nuqs");
  const defaults: Record<string, string> = {
    panel: "files",
    path: "",
    workspaceId: "",
  };

  return {
    ...actual,
    useQueryState: (key: string) => {
      const [value, setValue] = React.useState(
        queryState.get(key) ?? defaults[key] ?? "",
      );

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

describe("WorkspacePage", () => {
  beforeEach(() => {
    vi.clearAllMocks();

    vi.mocked(createWorkspace).mockResolvedValue(workspace);
    vi.mocked(getHealth).mockResolvedValue({ status: "ok" });
    vi.mocked(getWorkspace).mockResolvedValue(workspace);
    vi.mocked(listWorkspaces).mockResolvedValue({ workspaces: [] });
    vi.mocked(listFiles).mockResolvedValue({
      entries: [
        {
          isDir: false,
          name: "app.tsx",
          path: "web/src/app.tsx",
          size: 128,
        },
        {
          isDir: true,
          name: "features",
          path: "web/src/features",
          size: 0,
        },
      ],
    });
    vi.mocked(readFile).mockResolvedValue({
      content: "export function App() {\n  return null;\n}",
      path: "web/src/app.tsx",
    });
    vi.mocked(getGitStatus).mockResolvedValue({
      porcelain: " M web/src/app.tsx\n?? scratch.md",
    });
    vi.mocked(getGitDiff).mockImplementation((_workspaceId, path) =>
      Promise.resolve({
        diff: path ? `diff for ${path}` : "full workspace diff",
        path,
      }),
    );
    vi.mocked(queueCommand).mockResolvedValue({
      command: "pnpm --dir web test",
      createdAt: "2026-05-20T00:00:00Z",
      id: "cmd_1",
      status: "queued",
    });
  });

  it("renders the workspace shell and reads a selected file", async () => {
    const user = userEvent.setup();
    renderWorkspace("/workspace?workspaceId=ws_1&panel=files");

    await user.click(await screen.findByRole("button", { name: /app\.tsx/i }));

    expect(readFile).toHaveBeenCalledWith("ws_1", "web/src/app.tsx");
    expect(
      await screen.findByText(/export function App\(\)/),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/export function App\(\)/).closest("pre"),
    ).toHaveClass("h-full", "min-h-0", "overflow-auto");
  });

  it("loads a selected Git diff from the change list", async () => {
    const user = userEvent.setup();
    renderWorkspace("/workspace?workspaceId=ws_1&panel=git");

    expect(await screen.findByText("full workspace diff")).toBeInTheDocument();

    await user.click(
      screen.getByRole("button", { name: /web\/src\/app\.tsx/i }),
    );

    await waitFor(() => {
      expect(getGitDiff).toHaveBeenLastCalledWith("ws_1", "web/src/app.tsx");
    });
    expect(
      await screen.findByText("diff for web/src/app.tsx"),
    ).toBeInTheDocument();
  });

  it("queues a submitted command without faking command output", async () => {
    const user = userEvent.setup();
    renderWorkspace("/workspace?workspaceId=ws_1&panel=commands");

    await user.type(screen.getByLabelText("Command"), "pnpm --dir web test");
    await user.click(screen.getByRole("button", { name: "Queue command" }));

    await waitFor(() => {
      expect(queueCommand).toHaveBeenCalledWith("ws_1", "pnpm --dir web test");
    });
    expect(await screen.findByText("Command accepted")).toBeInTheDocument();
    expect(screen.getAllByText(/cmd_1/).length).toBeGreaterThan(0);
    expect(
      screen.getByText(/output replay is waiting on process endpoints/i),
    ).toBeInTheDocument();
  });
});

function renderWorkspace(initialEntry: string) {
  queryState.clear();
  const url = new URL(initialEntry, "http://localhost");
  for (const key of ["workspaceId", "panel", "path"]) {
    const value = url.searchParams.get(key);
    if (value !== null) {
      queryState.set(key, value);
    }
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
          <WorkspacePage />
        </MemoryRouter>
      </ThemeProvider>
    </QueryClientProvider>,
  );
}
