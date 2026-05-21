# PatchPilot MVP Spec

`docs/project-rules.md` owns locked rules. This file owns MVP scope, flows, API, data, acceptance.

## Objective

Single self-hosted developer completes the mobile AI coding loop:

```txt
open repo -> ask AI -> agent inspects approved files -> proposes patch
-> user reviews/approves/rejects -> server applies approved patch
-> user runs commands/tests -> user commits selected paths
```

## Decisions

- Single-user, self-hosted, local filesystem + SQLite.
- Default app space is `~/.patchpilot`; it may store PatchPilot-owned state.
- REST mutations, SSE realtime, no WebSocket.
- Admin-token auth with HTTP-only session cookie.
- Commands run as server OS user at workspace root.
- Agent changes are patch-first and user-approved.
- Workspace Mode is lightweight support UI, not IDE.
- Manual edits: small text files under workspace root only.
- Agent commands: fixed allowlist below.
- Command replay: latest 1 MiB per command.
- Commits require explicit selected paths; UI may default to applied PatchPilot patch paths.

## Flows

Open workspace:

```txt
choose local repo -> validate allowed Git repo -> create/restore metadata
-> refresh recursive file index -> restore/create session -> open Vibe Mode
```

Show readiness while indexing and Git status after load.

AI task:

```txt
prompt -> create task -> agent reads/searches approved files
-> stream progress -> return plan/summary/patch -> wait for approval
```

Timeline includes deltas, tool starts/finishes, command output, patch creation, status changes.

Review patch:

```txt
patch -> mobile-usable diff -> approve or reject
```

Approve verifies clean apply, applies, updates Git status, records applied. Reject leaves files unchanged and may record reason. Applied PatchPilot patches can revert. Invalid applies fail safely.

Run command:

```txt
enter/select command -> start process at workspace root
-> stream stdout/stderr -> show exit code/duration
```

Refresh replays latest 1 MiB. Dangerous agent commands need approval.

Preview:

```txt
run dev server -> detect port -> user exposes -> same-host preview URL
```

Agents never expose ports. Proxy: `/workspaces/:workspaceId/ports/:port/proxy/*`.

Commit:

```txt
review status/diff grouped by staged and unstaged paths
-> stage unstaged paths when needed
-> message -> commit staged paths -> return hash
```

Commit and stage requests still send explicit paths derived from the visible Git
sections. No push. Push/pull/branch management are post-MVP.

## API

All endpoints except `GET /api/health` and `POST /api/auth/login` require session cookie. Workspace APIs are scoped by `workspaceId`. JSON except SSE/proxy.

```txt
GET  /api/health

POST /api/auth/login
GET  /api/auth/session
POST /api/auth/logout

POST   /api/workspaces
GET    /api/workspaces
GET    /api/workspaces/:workspaceId
DELETE /api/workspaces/:workspaceId

GET /api/workspaces/:workspaceId/files?path=
GET /api/workspaces/:workspaceId/files/index
POST /api/workspaces/:workspaceId/files/index/refresh
GET /api/workspaces/:workspaceId/file?path=
PUT /api/workspaces/:workspaceId/file
GET /api/workspaces/:workspaceId/search?q=

POST /api/workspaces/:workspaceId/agent/tasks
GET  /api/workspaces/:workspaceId/agent/tasks
GET  /api/workspaces/:workspaceId/agent/tasks/:taskId
POST /api/workspaces/:workspaceId/agent/tasks/:taskId/cancel

GET  /api/workspaces/:workspaceId/patches/:patchId
POST /api/workspaces/:workspaceId/patches/:patchId/apply
POST /api/workspaces/:workspaceId/patches/:patchId/reject
POST /api/workspaces/:workspaceId/patches/:patchId/revert

POST /api/workspaces/:workspaceId/commands
GET  /api/workspaces/:workspaceId/processes
GET  /api/workspaces/:workspaceId/processes/:processId
POST /api/workspaces/:workspaceId/processes/:processId/stop

GET  /api/workspaces/:workspaceId/git/status
GET  /api/workspaces/:workspaceId/git/diff?path=
POST /api/workspaces/:workspaceId/git/stage
POST /api/workspaces/:workspaceId/git/unstage
POST /api/workspaces/:workspaceId/git/discard
POST /api/workspaces/:workspaceId/git/commit

GET  /api/workspaces/:workspaceId/ports
POST /api/workspaces/:workspaceId/ports/:port/expose
GET  /workspaces/:workspaceId/ports/:port/proxy/*
GET  /api/workspaces/:workspaceId/events
```

Workspace list response:

```json
{
  "workspaces": []
}
```

The list is newest-first.

Health response:

```json
{
  "status": "ok"
}
```

Health returns `503` with the standard REST error envelope when the app database is unavailable.

File API response notes:

- `GET /api/workspaces/:workspaceId/files?path=` returns `{"entries":[]}` for a workspace-relative directory.
- `GET /api/workspaces/:workspaceId/files/index` returns `{"entries":[]}` for the current recursive file index, with workspace-relative `path`, `size`, and `modifiedAt` metadata.
- `POST /api/workspaces/:workspaceId/files/index/refresh` rebuilds the recursive file index and returns the same response shape.
- `GET /api/workspaces/:workspaceId/file?path=` returns `{"path":"...","content":"..."}` for readable text files up to 1 MiB.
- `GET /api/workspaces/:workspaceId/search?q=` returns `{"results":[]}` for basic filename/content matches under the workspace root.
- File APIs ignore `.git`, `node_modules`, and `build` directories, skip symlinks while walking, and skip files larger than 1 MiB. Direct reads of ignored paths or files over 1 MiB return the standard REST error envelope.

Git API response notes:

- `GET /api/workspaces/:workspaceId/git/status` returns `{"porcelain":"..."}` from Git porcelain status, including untracked and ignored paths.
- `GET /api/workspaces/:workspaceId/git/diff?path=` returns `{"path":"...","diff":"..."}` for the full workspace or a workspace-relative path. Untracked file diffs are shown without staging the file.
- `POST /api/workspaces/:workspaceId/git/stage` accepts `{"paths":["..."]}` with explicit non-empty workspace-relative paths and returns the updated Git status.
- `POST /api/workspaces/:workspaceId/git/unstage` accepts `{"paths":["..."]}` with explicit non-empty workspace-relative paths, unstages only those paths, and returns the updated Git status.
- `POST /api/workspaces/:workspaceId/git/discard` accepts `{"paths":["..."]}` with explicit non-empty workspace-relative paths, discards only unstaged changes for those paths, removes untracked paths, and returns the updated Git status.
- `POST /api/workspaces/:workspaceId/git/commit` accepts `{"message":"...","paths":["..."]}` with the exact user message and explicit non-empty workspace-relative paths, stages only those paths, commits them, and returns `{"hash":"..."}`. The API never pushes.

SSE envelope:

```json
{
  "id": "evt_123",
  "workspaceId": "ws_123",
  "type": "agent.task.status_changed",
  "createdAt": "2026-05-20T10:00:00Z",
  "payload": {}
}
```

Events: `workspace.ready`, `workspace.indexing`, `agent.delta`, `agent.tool.started`, `agent.tool.finished`, `agent.approval_required`, `agent.task.status_changed`, `patch.created`, `patch.applied`, `patch.rejected`, `command.output`, `process.started`, `process.exited`, `port.opened`, `port.exposed`, `git.changed`.

## Agent Tools

- `list_files`: list files/dirs.
- `read_file`: read approved text files under workspace root.
- `search_files`: search filenames/contents.
- `git_status`: inspect status.
- `git_diff`: inspect diff.
- `run_command`: run approved command; side effects possible.
- `propose_patch`: submit patch for review.

Agents must not read outside workspace root, access secrets, auto-apply patches, expose ports, or run non-allowlisted commands without approval.

## Command Allowlist

Agent auto-run requires exact allowlist match and no shell control operators:

- `git status`, `git diff`, `git log`
- `npm run test|lint|build|dev`
- `pnpm test|lint|build|dev`
- `yarn test|lint|build|dev`
- `bun test`, `bun run lint|build|dev`
- `go test ./...`, `go test <package>`, `go build ./...`
- `pytest`, `python -m pytest`, `python3 -m pytest`
- `cargo test`, `cargo build`
- `make test|lint|build|dev`

Everything else requires explicit approval.

## Data Model

SQLite app state; source files on disk; Git owns repo history.
PatchPilot-owned state may live in the default app space at `~/.patchpilot`.
Workspace source files stay in their original repositories and are not copied into app space.

Runtime config loads from OS environment variables, falling back to a local `.env` file in the process working directory. OS environment variables override `.env` values. `PATCHPILOT_ADDR` controls the backend listen address, `PATCHPILOT_ALLOWED_ROOTS` controls the OS path-list of allowed workspace roots, `PATCHPILOT_STATIC_DIR` optionally serves the built frontend, `PATCHPILOT_LOG_FORMAT` selects colorized `console` logs or `json` logs, and `PATCHPILOT_DB_PATH` overrides the SQLite database path. When `PATCHPILOT_DB_PATH` is unset, `PATCHPILOT_DATA_DIR` controls the directory for `patchpilot.db`; otherwise `~/.patchpilot/patchpilot.db` is used.

- `auth_sessions`: `id`, `session_hash`, `created_at`, `last_seen_at`, `expires_at`.
- `workspaces`: `id`, `name`, `root_path`, `git_remote?`, `default_branch?`, `status(indexing|ready|error)`, timestamps.
- `file_index`: `workspace_id`, `path`, `size`, `modified_at`, `indexed_at`.
- `sessions`: `id`, `workspace_id`, `title`, `mode(vibe|workspace)`, timestamps.
- `agent_tasks`: `id`, `workspace_id`, `session_id`, `prompt`, `status`, `plan?`, `summary?`, `error?`, timestamps.
- `task_events`: `id`, `task_id`, `type`, `payload_json`, `created_at`.
- `patches`: `id`, `workspace_id`, `task_id`, `base_commit?`, `diff`, `summary`, `status`, `applied_at?`, `created_at`.
- `commands`: `id`, `workspace_id`, `task_id?`, `command`, `cwd`, `status`, `exit_code?`, `started_at?`, `finished_at?`.
- `command_output`: `id`, `command_id`, `stream(stdout|stderr)`, `chunk`, `created_at`.
- `ports`: `id`, `workspace_id`, `process_id?`, `port`, `status(detected|exposed|closed)`, `exposed_path?`, timestamps.
- `git_snapshots`: `id`, `workspace_id`, `patch_id?`, `commit_sha?`, `status_json`, `created_at`.

Task status:

```txt
queued -> running -> waiting_approval -> applying -> done
running -> failed | cancelled
waiting_approval -> rejected
applying -> failed
```

Patch status:

```txt
proposed -> applied -> reverted
proposed -> rejected
proposed -> failed
```

Command status:

```txt
queued -> running -> exited
queued -> running -> stopped
queued -> failed
running -> failed
```

## Non-Goals

Full browser IDE, multi-tab editor, LSP, inline diagnostics, xterm terminal, push/pull/branch/merge/rebase, multi-user/team/RBAC, hosted SaaS, billing, plugins, public tunnels/share URLs, strong sandboxing guarantees, Docker-required runtime, remote scheduling, marketplace integrations, agent secret access, auto-apply, auto-expose ports, WebSocket.

## Acceptance

- Open local Git repo workspace.
- Ask AI from Vibe Mode.
- Agent reads/searches approved files.
- Agent returns patch, not file mutations.
- User reviews diff and approves/rejects.
- Server applies only approved patches.
- User runs command and sees streamed output plus exit status.
- User views Git status/diff.
- User commits explicit non-empty selected paths.
- User previews app through same-host proxy.
- Mobile/iPad user completes full AI coding loop from Vibe Mode.
