# PatchPilot Concept

Product direction only. Locked rules live in `docs/project-rules.md`; active
scope, API, and data contracts live in `docs/product-spec.md`.

## Vision

PatchPilot is a self-hosted, single-user AI coding workspace for running the
coding-agent loop against local Git repositories from mobile, iPad, or browser.

It is not a VS Code clone. The product is optimized for:

- Chat-driven agent work.
- Composer controls for choosing workspace permissions before work starts.
- Clear review of approval-required tools.
- Running agent-command verification and using a workspace terminal.
- Fast indexed file search, command palette navigation, and readable diffs.
- Previewing local apps through a same-host proxy.
- Inspecting Git status/diffs and committing selected paths.

Tagline: fastest practical way to collaborate with coding agents from any
device.

## Product Direction

Main loop:

```txt
open repo -> create/open conversation -> chat with agent
-> set permissions -> stream text and tool activity -> approve/reject mutating tools
-> run or review verification -> use terminal/preview when needed
-> commit selected paths
```

The server owns source access, Git, the agent runtime, process execution,
workspace terminal sessions, app metadata, and preview proxying. The browser
client owns the interaction surface and never bypasses backend safety checks.

## UX Modes

Vibe Mode is the default mobile/iPad experience and the primary AI workflow. It
focuses on the prompt, conversation history, agent progress, patch summaries,
approval decisions, verification results, and quick commits. Users should not
need to constantly manage files.

Workspace Mode is a lightweight repository control surface. It supports files,
search, small edits, diffs, interactive terminal sessions, preview, and Git
status. It provides more control without growing into full IDE parity.

Users can switch modes inside the same workspace. Both modes share backend
state, conversations, agent runs, terminal metadata, Git state, and preview
state.

## Target Users

- Indie hackers.
- Developers working from tablets or phones.
- Small self-hosted teams operating as a single local user.
- AI-first developers who prefer reviewing agent output over driving an IDE.

## Differentiation

Compared with code-server or VS Code Web, PatchPilot is lighter, mobile-first,
agent-first, and intentionally smaller than a desktop IDE.

Compared with Cursor/Windsurf, PatchPilot is self-hosted, browser-first, and does
not require a desktop app.

Compared with Codespaces, PatchPilot is cheaper to self-run, simpler, and better
suited to individual or small private workflows.

## Current Scope

In scope for the active product:

- Open and index a local Git repository under configured allowed roots.
- Create, list, open, rename, and continue conversations per workspace.
- Send user messages that start agent runs with model and reasoning choices.
- Review and adjust workspace permission controls from the composer before a run.
- Stream agent activity, tool calls, agent command output, terminal metadata, and
  workspace events.
- Let agents inspect approved workspace context and propose reviewable patches.
- Require user approval before mutating tools run.
- Apply or reject patch tool calls through backend-controlled approval flow.
- Run classified agent commands without a shell.
- Create, switch, resize, rename, and close workspace terminal sessions.
- Keep active terminal replay bounded in memory without persisting transcripts.
- Search indexed files and open Workspace actions from the command palette.
- Detect and expose same-host preview ports.
- Show Git status, syntax-highlighted collapsible diffs, status-bar controls,
  and commit explicit selected paths.
- Cancel pending approvals when an agent run is canceled.
- Configure appearance, default agent settings, local skills, MCP status/config,
  and safe runtime status from Settings.
- Keep the mobile/iPad Vibe Mode loop complete and usable.

Out of scope for the active product:

- Full IDE behavior, multi-tab editor, LSP, inline diagnostics, terminal emulator
  parity outside the Workspace bottom terminal panel.
- Push/pull/branch/merge/rebase management.
- Non-terminal WebSocket, public tunnels, Docker-required runtime, remote skill
  marketplace or MCP public discovery.
- Multi-user/team/RBAC, hosted SaaS, billing, enterprise administration.

## Architecture Shape

Backend:

- Go single binary serving REST, SSE, terminal WebSocket, preview proxy, and
  embedded frontend.
- SQLite stores PatchPilot metadata; Git remains the source history.
- Source files stay in their original workspace repositories.
- Agent commands run as the server OS user from the workspace root, without a
  shell; Workspace terminal sessions run PTY shells at the workspace root.
- File access, Git, process execution, and port proxying are backend-controlled.

Frontend:

- React/Vite app with two product modes.
- TanStack Query for server state, Zustand/React state for local UI only.
- Shared typed API client; no direct frontend API bypass.
- Tailwind token-based styling and shared UI primitives.

Agent model:

- Agent runs belong to conversations.
- Agents return assistant text and tool calls.
- File listing/search and approved command reads enforce workspace, ignore,
  secret, and size checks.
- Mutating tools require explicit approval and run only through the backend.
- Runs end with changed files, verification result, and remaining risks.

## Safety Principles

- Single-user auth gates all APIs except health and login.
- Admin token and secrets never enter logs, events, or agent context.
- Workspace paths are validated, workspace-relative, and blocked from traversal.
- Secret-like files are blocked from agent reads by default.
- Agent commands are classified, run without shell expansion, and block
  destructive patterns. Workspace terminal input stays inside user-created PTY
  sessions and is not persisted.
- Preview proxy is same-host only; agents do not expose ports.

## Roadmap

1. Core server, auth, workspace validation, SQLite migrations, REST/SSE.
2. Vibe Mode conversations, agent runs, tool approval, patch flow.
3. Mobile-first polish for the full chat-driven AI coding loop.
4. Workspace terminal sessions, port detection, same-host preview.
5. Workspace Mode files/search/diff/Git controls and Settings.
6. Optional future expansion only after the active scope is stable; do not pull
   post-scope ideas into the product without updating locked rules/specs first.

## Main Rule

PatchPilot should help a developer ask an agent for work, review exactly what it
will do, run or inspect verification, and commit selected results from any
device. Every feature should strengthen that loop or stay out.
