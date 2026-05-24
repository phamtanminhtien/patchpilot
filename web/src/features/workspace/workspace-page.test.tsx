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
  commitGitChanges,
  createWorkspace,
  discardGitChanges,
  exposePort,
  getGitDiff,
  getGitStatus,
  getHealth,
  getProcess,
  getWorkspace,
  listFileIndex,
  listPorts,
  listProcesses,
  listWorkspaces,
  queueCommand,
  readFile,
  refreshFileIndex,
  searchFiles,
  stageGitFiles,
  stopProcess,
  unstageGitFiles,
  writeFile,
} from "@/shared/api";

import { WorkspacePage } from "./workspace-page";

const queryState = vi.hoisted(() => new Map<string, string>());

vi.mock("@/shared/api", () => ({
  apiErrorCode: (error: unknown) =>
    (error as { response?: { data?: { error?: { code?: string } } } }).response
      ?.data?.error?.code,
  apiErrorMessage: (error: unknown) =>
    error instanceof Error ? error.message : "Request failed",
  commitGitChanges: vi.fn(),
  createWorkspace: vi.fn(),
  discardGitChanges: vi.fn(),
  exposePort: vi.fn(),
  getHealth: vi.fn(),
  getProcess: vi.fn(),
  getGitDiff: vi.fn(),
  getGitStatus: vi.fn(),
  getWorkspace: vi.fn(),
  listFileIndex: vi.fn(),
  listPorts: vi.fn(),
  listProcesses: vi.fn(),
  listWorkspaces: vi.fn(),
  queueCommand: vi.fn(),
  readFile: vi.fn(),
  refreshFileIndex: vi.fn(),
  searchFiles: vi.fn(),
  stageGitFiles: vi.fn(),
  stopProcess: vi.fn(),
  unstageGitFiles: vi.fn(),
  writeFile: vi.fn(),
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
    vi.mocked(searchFiles).mockResolvedValue({
      results: [
        {
          kind: "filename",
          path: "README.md",
          preview: "README.md",
        },
        {
          kind: "content",
          line: 12,
          path: "docs/product-spec.md",
          preview: "Workspace Mode supports files",
        },
      ],
    });
    vi.mocked(writeFile).mockResolvedValue({
      content: "export function App() {\n  return true;\n}",
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
    vi.mocked(listProcesses).mockResolvedValue({
      processes: [
        {
          command: "pnpm --dir web test",
          createdAt: "2026-05-20T00:00:00Z",
          cwd: "/workspace/patchpilot",
          durationMs: 1200,
          exitCode: 0,
          finishedAt: "2026-05-20T00:00:02Z",
          id: "cmd_1",
          startedAt: "2026-05-20T00:00:00Z",
          status: "exited",
          workspaceId: "ws_1",
        },
      ],
    });
    vi.mocked(listPorts).mockResolvedValue({
      ports: [
        {
          closedAt: null,
          createdAt: "2026-05-20T00:00:00Z",
          exposedPath: null,
          exposedUrl: null,
          id: "port_5173",
          port: 5173,
          processId: "cmd_1",
          status: "detected",
          updatedAt: "2026-05-20T00:00:00Z",
          workspaceId: "ws_1",
        },
        {
          closedAt: null,
          createdAt: "2026-05-20T00:00:00Z",
          exposedPath: "/workspaces/ws_1/ports/8080/proxy/",
          exposedUrl: "/workspaces/ws_1/ports/8080/proxy/",
          id: "port_8080",
          port: 8080,
          processId: "cmd_1",
          status: "exposed",
          updatedAt: "2026-05-20T00:00:00Z",
          workspaceId: "ws_1",
        },
      ],
    });
    vi.mocked(exposePort).mockResolvedValue({
      closedAt: null,
      createdAt: "2026-05-20T00:00:00Z",
      exposedPath: "/workspaces/ws_1/ports/5173/proxy/",
      exposedUrl: "/workspaces/ws_1/ports/5173/proxy/",
      id: "port_5173",
      port: 5173,
      processId: "cmd_1",
      status: "exposed",
      updatedAt: "2026-05-20T00:00:01Z",
      workspaceId: "ws_1",
    });
    vi.mocked(getProcess).mockResolvedValue({
      command: {
        command: "pnpm --dir web test",
        createdAt: "2026-05-20T00:00:00Z",
        cwd: "/workspace/patchpilot",
        durationMs: 1200,
        exitCode: 0,
        finishedAt: "2026-05-20T00:00:02Z",
        id: "cmd_1",
        startedAt: "2026-05-20T00:00:00Z",
        status: "exited",
        workspaceId: "ws_1",
      },
      output: [
        {
          chunk: "tests passed\n",
          commandId: "cmd_1",
          createdAt: "2026-05-20T00:00:01Z",
          id: "out_1",
          stream: "stdout",
        },
      ],
    });
    vi.mocked(stopProcess).mockResolvedValue({
      command: "pnpm --dir web dev",
      createdAt: "2026-05-20T00:00:00Z",
      cwd: "/workspace/patchpilot",
      durationMs: 500,
      exitCode: null,
      finishedAt: "2026-05-20T00:00:01Z",
      id: "cmd_2",
      startedAt: "2026-05-20T00:00:00Z",
      status: "stopped",
      workspaceId: "ws_1",
    });
    vi.mocked(queueCommand).mockResolvedValue({
      command: "pnpm --dir web test",
      createdAt: "2026-05-20T00:00:00Z",
      cwd: "/workspace/patchpilot",
      durationMs: null,
      exitCode: null,
      finishedAt: null,
      id: "cmd_1",
      status: "queued",
      startedAt: null,
      workspaceId: "ws_1",
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

  it("searches workspace files and opens a selected result", async () => {
    const user = userEvent.setup();
    renderWorkspace("/workspace?workspaceId=ws_1&panel=files");

    fireEvent.change(screen.getByLabelText("Search files"), {
      target: { value: "workspace" },
    });

    expect(await screen.findAllByText("README.md")).toHaveLength(2);
    expect(screen.getByText("Filename")).toBeInTheDocument();
    expect(screen.getByText("Content line 12")).toBeInTheDocument();
    expect(
      screen.getByText("Workspace Mode supports files"),
    ).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: /README\.md/i }));

    await waitFor(() => {
      expect(searchFiles).toHaveBeenLastCalledWith("ws_1", "workspace");
      expect(readFile).toHaveBeenLastCalledWith("ws_1", "README.md");
    });
  });

  it("shows empty and error states for file search", async () => {
    vi.mocked(searchFiles).mockResolvedValueOnce({ results: [] });
    renderWorkspace("/workspace?workspaceId=ws_1&panel=files");

    fireEvent.change(screen.getByLabelText("Search files"), {
      target: { value: "missing" },
    });

    expect(await screen.findByText("No matching files.")).toBeInTheDocument();

    vi.mocked(searchFiles).mockRejectedValueOnce(new Error("Search failed"));
    fireEvent.change(screen.getByLabelText("Search files"), {
      target: { value: "broken" },
    });

    expect(await screen.findByText("Search failed")).toBeInTheDocument();
  });

  it("shows file search loading state", async () => {
    vi.mocked(searchFiles).mockReturnValue(new Promise(() => {}));
    renderWorkspace("/workspace?workspaceId=ws_1&panel=files");

    fireEvent.change(screen.getByLabelText("Search files"), {
      target: { value: "slow" },
    });

    expect(await screen.findByText(/Searching files/)).toBeInTheDocument();
  });

  it("edits and saves a selected text file", async () => {
    const user = userEvent.setup();
    renderWorkspace(
      "/workspace?workspaceId=ws_1&panel=files&path=web/src/app.tsx",
    );

    expect(
      await screen.findByText(/export function App\(\)/),
    ).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Edit file" }));

    const editor = screen.getByLabelText("File content");
    fireEvent.change(editor, {
      target: { value: "export function App() {\n  return true;\n}" },
    });
    await user.click(screen.getByRole("button", { name: "Save file" }));

    await waitFor(() => {
      expect(writeFile).toHaveBeenCalledWith("ws_1", {
        content: "export function App() {\n  return true;\n}",
        path: "web/src/app.tsx",
      });
    });
    expect(await screen.findByText(/return true/)).toBeInTheDocument();
  });

  it("shows save rejection and keeps edited content", async () => {
    const user = userEvent.setup();
    vi.mocked(writeFile).mockRejectedValue(new Error("File is too large"));
    renderWorkspace(
      "/workspace?workspaceId=ws_1&panel=files&path=web/src/app.tsx",
    );

    await screen.findByText(/export function App\(\)/);
    await user.click(screen.getByRole("button", { name: "Edit file" }));
    const editor = screen.getByLabelText("File content");
    fireEvent.change(editor, {
      target: { value: "oversized content" },
    });
    await user.click(screen.getByRole("button", { name: "Save file" }));

    expect(await screen.findByText("File is too large")).toBeInTheDocument();
    expect(screen.getByLabelText("File content")).toHaveValue(
      "oversized content",
    );
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
      name: "Stage all changes",
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

  it("runs a submitted command and shows output replay", async () => {
    const user = userEvent.setup();
    renderWorkspace("/workspace?workspaceId=ws_1&panel=commands");

    await user.type(screen.getByLabelText("Command"), "pnpm --dir web test");
    await user.click(screen.getByRole("button", { name: "Run" }));

    await waitFor(() => {
      expect(queueCommand).toHaveBeenCalledWith(
        "ws_1",
        "pnpm --dir web test",
        false,
      );
    });
    expect(await screen.findByText("tests passed")).toBeInTheDocument();
    expect(screen.getAllByText(/cmd_1/).length).toBeGreaterThan(0);
  });

  it("confirms risky commands before running", async () => {
    const user = userEvent.setup();
    vi.mocked(queueCommand)
      .mockRejectedValueOnce(
        Object.assign(new Error("Command requires confirmation"), {
          response: {
            data: {
              error: {
                code: "confirmation_required",
                details: {},
                message: "Command requires confirmation",
              },
            },
          },
        }),
      )
      .mockResolvedValueOnce({
        command: "node scripts/check.js",
        createdAt: "2026-05-20T00:00:00Z",
        cwd: "/workspace/patchpilot",
        durationMs: null,
        exitCode: null,
        finishedAt: null,
        id: "cmd_3",
        startedAt: null,
        status: "queued",
        workspaceId: "ws_1",
      });
    renderWorkspace("/workspace?workspaceId=ws_1&panel=commands");

    await user.type(screen.getByLabelText("Command"), "node scripts/check.js");
    await user.click(screen.getByRole("button", { name: "Run" }));

    expect(
      await screen.findByText("Confirm risky command"),
    ).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Run anyway" }));

    await waitFor(() => {
      expect(queueCommand).toHaveBeenLastCalledWith(
        "ws_1",
        "node scripts/check.js",
        true,
      );
    });
  });

  it("shows preview ports and exposes detected ports from the main panel", async () => {
    const user = userEvent.setup();
    renderWorkspace("/workspace?workspaceId=ws_1&panel=preview");

    const previewPorts = await screen.findByRole("region", {
      name: "Preview ports",
    });
    expect(
      within(previewPorts).getByText("localhost:5173"),
    ).toBeInTheDocument();
    expect(
      within(previewPorts).getByText("localhost:8080"),
    ).toBeInTheDocument();
    expect(
      within(previewPorts).getByRole("link", { name: "Open preview" }),
    ).toHaveAttribute("href", "/workspaces/ws_1/ports/8080/proxy/");

    await user.click(
      within(previewPorts).getByRole("button", { name: "Expose port" }),
    );

    await waitFor(() => {
      expect(exposePort).toHaveBeenCalledWith("ws_1", 5173);
    });
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
