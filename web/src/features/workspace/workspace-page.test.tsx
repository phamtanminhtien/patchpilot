import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type * as NuqsModule from "nuqs";
import type * as ReactModule from "react";
import { MemoryRouter } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ThemeProvider } from "@/app/theme";
import {
  commitGitChanges,
  createWorkspace,
  discardGitChanges,
  getGitDiff,
  getGitStatus,
  getHealth,
  getWorkspace,
  listFileIndex,
  listWorkspaces,
  queueCommand,
  readFile,
  refreshFileIndex,
  stageGitFiles,
  unstageGitFiles,
} from "@/shared/api";

import { WorkspacePage } from "./workspace-page";

const queryState = vi.hoisted(() => new Map<string, string>());

vi.mock("@/shared/api", () => ({
  apiErrorMessage: (error: unknown) =>
    error instanceof Error ? error.message : "Request failed",
  commitGitChanges: vi.fn(),
  createWorkspace: vi.fn(),
  discardGitChanges: vi.fn(),
  getHealth: vi.fn(),
  getGitDiff: vi.fn(),
  getGitStatus: vi.fn(),
  getWorkspace: vi.fn(),
  listFileIndex: vi.fn(),
  listWorkspaces: vi.fn(),
  queueCommand: vi.fn(),
  readFile: vi.fn(),
  refreshFileIndex: vi.fn(),
  stageGitFiles: vi.fn(),
  unstageGitFiles: vi.fn(),
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
    vi.mocked(listFileIndex).mockResolvedValue({
      entries: [
        {
          modifiedAt: "2026-05-20T00:00:00Z",
          path: "web/src/app.tsx",
          size: 128,
        },
        {
          modifiedAt: "2026-05-20T00:00:00Z",
          path: "web/src/features/workspace-page.tsx",
          size: 256,
        },
        {
          modifiedAt: "2026-05-20T00:00:00Z",
          path: "dist/app.js",
          size: 512,
        },
      ],
    });
    vi.mocked(refreshFileIndex).mockResolvedValue({
      entries: [
        {
          modifiedAt: "2026-05-20T00:00:00Z",
          path: "README.md",
          size: 64,
        },
      ],
    });
    vi.mocked(readFile).mockResolvedValue({
      content: "export function App() {\n  return null;\n}",
      path: "web/src/app.tsx",
    });
    vi.mocked(getGitStatus).mockResolvedValue({
      porcelain: " M web/src/app.tsx\n?? scratch.md\n!! dist/",
    });
    vi.mocked(getGitDiff).mockImplementation((_workspaceId, path) =>
      Promise.resolve({
        diff: path ? `diff for ${path}` : "full workspace diff",
        path,
      }),
    );
    vi.mocked(stageGitFiles).mockResolvedValue({
      porcelain: "A  web/src/app.tsx\n?? scratch.md\n!! dist/",
    });
    vi.mocked(unstageGitFiles).mockResolvedValue({
      porcelain: " M web/src/app.tsx\n?? scratch.md\n!! dist/",
    });
    vi.mocked(discardGitChanges).mockResolvedValue({
      porcelain: " M web/src/app.tsx",
    });
    vi.mocked(commitGitChanges).mockResolvedValue({
      hash: "1234567890abcdef",
    });
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

    await user.click(await screen.findByRole("treeitem", { name: "web" }));
    await user.click(await screen.findByRole("treeitem", { name: "src" }));
    await user.click(
      await screen.findByRole("treeitem", { name: /app\.tsx/i }),
    );

    expect(readFile).toHaveBeenCalledWith("ws_1", "web/src/app.tsx");
    expect(
      await screen.findByText(/export function App\(\)/),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/export function App\(\)/).closest("pre"),
    ).toHaveClass("h-full", "min-h-0", "overflow-auto");
  });

  it("refreshes the recursive file index manually", async () => {
    const user = userEvent.setup();
    renderWorkspace("/workspace?workspaceId=ws_1&panel=files");

    await user.click(
      await screen.findByRole("button", { name: "Refresh index" }),
    );

    await waitFor(() => {
      expect(refreshFileIndex).toHaveBeenCalledWith("ws_1");
    });
    expect(
      await screen.findByRole("treeitem", { name: /README\.md/i }),
    ).toBeInTheDocument();
  });

  it("shows interactive file details that can copy paths", async () => {
    const user = userEvent.setup();
    const clipboardWriteText = vi
      .spyOn(navigator.clipboard, "writeText")
      .mockResolvedValue(undefined);
    renderWorkspace("/workspace?workspaceId=ws_1&panel=files");

    await user.click(await screen.findByRole("treeitem", { name: "web" }));
    await user.click(await screen.findByRole("treeitem", { name: "src" }));
    const fileRow = await screen.findByRole("treeitem", { name: /app\.tsx/i });
    await user.hover(fileRow);
    expect(await screen.findByText("web/src/app.tsx")).toBeInTheDocument();
    const copyPathButtons = screen.getAllByRole("button", {
      name: "Copy path",
    });
    const copyPathButton = copyPathButtons.at(-1);
    expect(copyPathButton).toBeDefined();

    await user.click(copyPathButton as HTMLElement);

    expect(clipboardWriteText).toHaveBeenCalledWith("web/src/app.tsx");
  });

  it("dims files and folders ignored by Git", async () => {
    const user = userEvent.setup();
    renderWorkspace("/workspace?workspaceId=ws_1&panel=files");

    const ignoredFolder = await screen.findByRole("treeitem", {
      name: /dist/i,
    });
    expect(ignoredFolder).toHaveClass("opacity-55");

    await user.click(ignoredFolder);
    expect(
      await screen.findByRole("treeitem", { name: /app\.js/i }),
    ).toHaveClass("opacity-55");
  });

  it("does not dim folders only because they contain ignored files", async () => {
    const user = userEvent.setup();
    vi.mocked(getGitStatus).mockResolvedValue({
      porcelain: "!! dist/app.js",
    });
    renderWorkspace("/workspace?workspaceId=ws_1&panel=files");

    const distFolder = await screen.findByRole("treeitem", {
      name: "dist",
    });
    expect(distFolder).not.toHaveClass("opacity-55");

    await user.click(distFolder);
    expect(await screen.findByRole("treeitem", { name: "app.js" })).toHaveClass(
      "opacity-55",
    );
  });

  it("loads a selected Git diff from the change list", async () => {
    const user = userEvent.setup();
    renderWorkspace("/workspace?workspaceId=ws_1&panel=git");

    expect(await screen.findByText("full workspace diff")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "web/src/app.tsx" }));

    await waitFor(() => {
      expect(getGitDiff).toHaveBeenLastCalledWith("ws_1", "web/src/app.tsx");
    });
    expect(
      await screen.findByText("diff for web/src/app.tsx"),
    ).toBeInTheDocument();
  });

  it("stages unstaged Git paths without checkboxes", async () => {
    const user = userEvent.setup();
    renderWorkspace("/workspace?workspaceId=ws_1&panel=git");

    const stageButton = await screen.findByRole("button", {
      name: "Stage changes",
    });
    expect(stageButton).toBeEnabled();
    expect(await screen.findByText("Staged Changes")).toBeInTheDocument();
    expect(screen.getByText("Changes")).toBeInTheDocument();
    expect(screen.queryByRole("checkbox")).not.toBeInTheDocument();

    await user.click(stageButton);

    await waitFor(() => {
      expect(stageGitFiles).toHaveBeenCalledWith("ws_1", {
        paths: ["web/src/app.tsx", "scratch.md"],
      });
    });
  });

  it("runs Git item actions from the change list", async () => {
    const user = userEvent.setup();
    const porcelain = "M  staged.txt\n M changed.txt\n?? scratch.md";
    vi.mocked(getGitStatus).mockResolvedValue({
      porcelain,
    });
    vi.mocked(stageGitFiles).mockResolvedValue({ porcelain });
    vi.mocked(unstageGitFiles).mockResolvedValue({ porcelain });
    vi.mocked(discardGitChanges).mockResolvedValue({ porcelain });
    renderWorkspace("/workspace?workspaceId=ws_1&panel=git");

    await user.click(
      await screen.findByRole("button", {
        name: "Unstage change staged.txt",
      }),
    );
    await waitFor(() => {
      expect(unstageGitFiles).toHaveBeenCalledWith("ws_1", {
        paths: ["staged.txt"],
      });
    });

    await user.click(
      await screen.findByRole("button", {
        name: "Discard change changed.txt",
      }),
    );
    await waitFor(() => {
      expect(discardGitChanges).toHaveBeenCalledWith("ws_1", {
        paths: ["changed.txt"],
      });
    });

    await user.click(
      await screen.findByRole("button", {
        name: "Stage change scratch.md",
      }),
    );
    await waitFor(() => {
      expect(stageGitFiles).toHaveBeenLastCalledWith("ws_1", {
        paths: ["scratch.md"],
      });
    });
  });

  it("runs Git section actions for all files in the section", async () => {
    const user = userEvent.setup();
    const porcelain = "M  staged.txt\n M changed.txt\n?? scratch.md\n!! dist/";
    vi.mocked(getGitStatus).mockResolvedValue({
      porcelain,
    });
    vi.mocked(stageGitFiles).mockResolvedValue({ porcelain });
    vi.mocked(unstageGitFiles).mockResolvedValue({ porcelain });
    vi.mocked(discardGitChanges).mockResolvedValue({ porcelain });
    renderWorkspace("/workspace?workspaceId=ws_1&panel=git");

    expect(
      screen.queryByRole("button", { name: "dist/" }),
    ).not.toBeInTheDocument();

    await user.click(
      await screen.findByRole("button", {
        name: "Unstage all staged changes",
      }),
    );
    await waitFor(() => {
      expect(unstageGitFiles).toHaveBeenCalledWith("ws_1", {
        paths: ["staged.txt"],
      });
    });

    await user.click(
      await screen.findByRole("button", { name: "Discard all changes" }),
    );
    await waitFor(() => {
      expect(discardGitChanges).toHaveBeenCalledWith("ws_1", {
        paths: ["changed.txt", "scratch.md"],
      });
    });

    await user.click(
      await screen.findByRole("button", { name: "Stage all changes" }),
    );
    await waitFor(() => {
      expect(stageGitFiles).toHaveBeenLastCalledWith("ws_1", {
        paths: ["changed.txt", "scratch.md"],
      });
    });
  });

  it("does not render ignored-only Git changes in the sidebar", async () => {
    vi.mocked(getGitStatus).mockResolvedValue({
      porcelain: "!! dist/",
    });
    renderWorkspace("/workspace?workspaceId=ws_1&panel=git");

    expect(
      await screen.findByText("Working tree is clean."),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "dist/" }),
    ).not.toBeInTheDocument();
  });

  it("commits staged Git paths with the exact message and shows the hash", async () => {
    const user = userEvent.setup();
    vi.mocked(getGitStatus).mockResolvedValue({
      porcelain: "M  web/src/app.tsx\n M scratch.md",
    });
    renderWorkspace("/workspace?workspaceId=ws_1&panel=git");

    const commitButton = await screen.findByRole("button", {
      name: "Commit",
    });
    expect(commitButton).toBeDisabled();

    await user.type(
      screen.getByLabelText("Commit message"),
      "keep workspace git flow",
    );
    await user.click(commitButton);

    await waitFor(() => {
      expect(commitGitChanges).toHaveBeenCalledWith("ws_1", {
        message: "keep workspace git flow",
        paths: ["web/src/app.tsx"],
      });
    });
    expect(
      await screen.findByText(/Committed 1234567890ab/),
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
