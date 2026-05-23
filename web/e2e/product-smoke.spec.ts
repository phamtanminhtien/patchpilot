import { expect, type Page, type Route, test } from "@playwright/test";

const workspace = {
  createdAt: "2026-05-20T00:00:00Z",
  id: "ws_1",
  name: "patchpilot",
  rootPath: "/workspace/patchpilot",
  status: "ready",
  updatedAt: "2026-05-20T00:00:00Z",
};

const conversation = {
  createdAt: "2026-05-20T00:00:00Z",
  id: "conv_1",
  lastMessageAt: "2026-05-20T00:00:00Z",
  title: "Finish product smoke test",
  updatedAt: "2026-05-20T00:00:00Z",
  workspaceId: "ws_1",
};

const message = {
  content: "Finish product smoke test",
  conversationId: "conv_1",
  createdAt: "2026-05-20T00:00:00Z",
  id: "msg_1",
  role: "user",
  runId: "run_1",
  workspaceId: "ws_1",
};

const run = {
  conversationId: "conv_1",
  createdAt: "2026-05-20T00:00:00Z",
  error: null,
  finishedAt: null,
  id: "run_1",
  model: "gpt-5.5",
  reasoningEffort: "medium",
  startedAt: "2026-05-20T00:00:00Z",
  status: "waiting_tool_approval",
  summary: "",
  triggerMessageId: "msg_1",
  updatedAt: "2026-05-20T00:00:00Z",
  workspaceId: "ws_1",
};

const toolCall = {
  batchId: "batch_1",
  createdAt: "2026-05-20T00:00:00Z",
  decision: null,
  finishedAt: null,
  id: "tool_1",
  input:
    '{"summary":"Update product checklist","diff":"diff --git a/docs/product-release-checklist.md b/docs/product-release-checklist.md\\n"}',
  name: "apply_patch",
  output: "{}",
  providerCallId: "call_1",
  requiresApproval: true,
  sequence: 0,
  startedAt: null,
  status: "waiting_approval",
  runId: "run_1",
  workspaceId: "ws_1",
};

test("signs in and opens a recent workspace to a new Vibe chat", async ({
  page,
}) => {
  await mockPatchPilotApi(page);
  await page.goto("/vibe");

  await page.getByLabel("Admin token").fill("local-admin-token");
  await page.getByRole("button", { name: "Sign in" }).click();
  await page.getByRole("button", { name: /patchpilot/i }).click();

  await expect(page.getByLabel("Ask AI")).toBeVisible();
  await expect(
    page.getByRole("heading", { name: "PatchPilot conversation" }),
  ).toBeVisible();
});

test("covers workspace files, Git, commands, and preview smoke flows", async ({
  page,
}) => {
  await mockPatchPilotApi(page, { authenticated: true });
  await page.goto("/workspace?workspaceId=ws_1");

  await page.getByRole("treeitem", { name: "README.md" }).click();
  await expect(page.getByText("PatchPilot smoke file")).toBeVisible();

  await page.getByRole("button", { name: "Git" }).click();
  await expect(page.getByText("full workspace diff")).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Stage all changes" }),
  ).toBeVisible();

  await page.getByRole("button", { name: "Commands" }).click();
  await page
    .getByRole("textbox", { name: "Command" })
    .fill("pnpm --dir web test");
  await page.getByRole("button", { name: "Run" }).click();
  await expect(page.getByText("tests passed", { exact: true })).toBeVisible();
  await expect(page.getByText(/exit 0/i)).toBeVisible();

  await page.getByRole("button", { name: "Preview" }).click();
  const previewPorts = page.getByRole("region", { name: "Preview ports" });
  await expect(previewPorts.getByText("localhost:5173")).toBeVisible();
  await previewPorts.getByRole("button", { name: "Expose port" }).click();
  await expect(
    previewPorts.getByRole("link", { name: "Open preview" }),
  ).toHaveAttribute("href", "/workspaces/ws_1/ports/5173/proxy/");
});

async function mockPatchPilotApi(
  page: Page,
  options: { authenticated?: boolean } = {},
) {
  let authenticated = options.authenticated ?? false;
  let exposedPreview = false;

  await page.addInitScript(() => {
    class MockEventSource extends EventTarget {
      static CONNECTING = 0;
      static OPEN = 1;
      static CLOSED = 2;

      readyState = MockEventSource.OPEN;
      url: string;
      withCredentials: boolean;

      constructor(url: string | URL, init?: EventSourceInit) {
        super();
        this.url = String(url);
        this.withCredentials = init?.withCredentials ?? false;
      }

      close() {
        this.readyState = MockEventSource.CLOSED;
      }
    }

    window.EventSource = MockEventSource as unknown as typeof EventSource;
  });

  await page.route("**/*", async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    const path = url.pathname;
    const method = request.method();

    if (!path.startsWith("/api/")) {
      await route.continue();
      return;
    }

    if (path === "/api/auth/session" && method === "GET") {
      if (!authenticated) {
        await json(
          route,
          {
            error: {
              code: "unauthorized",
              details: {},
              message: "Authentication is required",
            },
          },
          401,
        );
        return;
      }
      await json(route, {
        session: {
          expiresAt: "2026-05-21T00:00:00Z",
          id: "auth_1",
          lastSeenAt: "2026-05-20T00:00:00Z",
        },
      });
      return;
    }

    if (path === "/api/auth/login" && method === "POST") {
      authenticated = true;
      await json(route, {
        session: {
          expiresAt: "2026-05-21T00:00:00Z",
          id: "auth_1",
          lastSeenAt: "2026-05-20T00:00:00Z",
        },
      });
      return;
    }

    if (path === "/api/health" && method === "GET") {
      await json(route, { status: "ok" });
      return;
    }

    if (path === "/api/workspaces" && method === "GET") {
      await json(route, { workspaces: [workspace] });
      return;
    }

    if (path === "/api/workspaces/ws_1" && method === "GET") {
      await json(route, workspace);
      return;
    }

    if (path === "/api/workspaces/ws_1/events" && method === "GET") {
      await route.fulfill({
        body: ": connected\n\n",
        contentType: "text/event-stream",
        status: 200,
      });
      return;
    }

    if (path === "/api/workspaces/ws_1/files/index" && method === "GET") {
      await json(route, {
        entries: [
          {
            modifiedAt: "2026-05-20T00:00:00Z",
            path: "README.md",
            size: 24,
          },
        ],
      });
      return;
    }

    if (path === "/api/workspaces/ws_1/file" && method === "GET") {
      await json(route, {
        content: "# PatchPilot smoke file\n",
        path: "README.md",
      });
      return;
    }

    if (path === "/api/workspaces/ws_1/git/status" && method === "GET") {
      await json(route, { porcelain: " M README.md" });
      return;
    }

    if (path === "/api/workspaces/ws_1/git/diff" && method === "GET") {
      await json(route, {
        diff: "full workspace diff",
        path: url.searchParams.get("path") ?? undefined,
      });
      return;
    }

    if (path === "/api/workspaces/ws_1/commands" && method === "POST") {
      await json(route, commandRecord("cmd_2", "queued"));
      return;
    }

    if (path === "/api/workspaces/ws_1/processes" && method === "GET") {
      await json(route, { processes: [commandRecord("cmd_1", "exited")] });
      return;
    }

    if (path === "/api/workspaces/ws_1/processes/cmd_1" && method === "GET") {
      await json(route, {
        command: commandRecord("cmd_1", "exited"),
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
      return;
    }

    if (path === "/api/workspaces/ws_1/processes/cmd_2" && method === "GET") {
      await json(route, {
        command: commandRecord("cmd_2", "exited"),
        output: [
          {
            chunk: "tests passed\n",
            commandId: "cmd_2",
            createdAt: "2026-05-20T00:00:01Z",
            id: "out_2",
            stream: "stdout",
          },
        ],
      });
      return;
    }

    if (path === "/api/workspaces/ws_1/ports" && method === "GET") {
      await json(route, {
        ports: [previewPort(exposedPreview ? "exposed" : "detected")],
      });
      return;
    }

    if (
      path === "/api/workspaces/ws_1/ports/5173/expose" &&
      method === "POST"
    ) {
      exposedPreview = true;
      await json(route, { port: previewPort("exposed") });
      return;
    }

    if (path === "/api/workspaces/ws_1/conversations" && method === "GET") {
      await json(route, { conversations: [conversation] });
      return;
    }

    if (
      path === "/api/workspaces/ws_1/conversations/conv_1" &&
      method === "GET"
    ) {
      await json(route, {
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
      return;
    }

    await json(
      route,
      { error: { code: "not_found", details: {}, message: path } },
      404,
    );
  });
}

async function json(route: Route, body: unknown, status = 200) {
  await route.fulfill({
    body: JSON.stringify(body),
    contentType: "application/json",
    status,
  });
}

function commandRecord(id: string, status: "queued" | "exited") {
  return {
    command: "pnpm --dir web test",
    createdAt: "2026-05-20T00:00:00Z",
    cwd: "/workspace/patchpilot",
    durationMs: status === "exited" ? 1200 : null,
    exitCode: status === "exited" ? 0 : null,
    finishedAt: status === "exited" ? "2026-05-20T00:00:02Z" : null,
    id,
    startedAt: "2026-05-20T00:00:00Z",
    status,
    workspaceId: "ws_1",
  };
}

function previewPort(status: "detected" | "exposed") {
  return {
    closedAt: null,
    createdAt: "2026-05-20T00:00:00Z",
    exposedPath:
      status === "exposed" ? "/workspaces/ws_1/ports/5173/proxy/" : null,
    exposedUrl:
      status === "exposed" ? "/workspaces/ws_1/ports/5173/proxy/" : null,
    id: "port_5173",
    port: 5173,
    processId: "cmd_1",
    status,
    updatedAt: "2026-05-20T00:00:00Z",
    workspaceId: "ws_1",
  };
}
