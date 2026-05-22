# PatchPilot Product Spec

`docs/project-rules.md` owns locked rules. This file owns active v0.2 product
scope, flows, API, data, and acceptance.

## Objective

PatchPilot v0.2 is a self-hosted, single-user AI coding workspace centered on
workspace conversations:

```txt
open repo -> open/create conversation -> send message
-> stream agent text/tools -> approve/reject mutating tools
-> execute approved tools -> summarize outcome
-> run/review verification -> commit selected paths
```

Core decisions:

- Local filesystem + SQLite; multiple conversations per workspace.
- REST mutations, SSE realtime, no WebSocket.
- Admin-token login with HTTP-only session cookie.
- Commands run as the server OS user at the workspace root, without a shell.
- Agent changes happen through tool calls; mutating tools require approval.
- Workspace Mode supports files, search, diffs, small edits, command output,
  preview, and Git status.
- Manual edits are limited to small text files under the workspace root.
- Agent commands auto-run only when exactly allowlisted below.
- Command replay keeps the latest 1 MiB per command.
- Commits require explicit selected paths; no push.
- Schema changes use explicit manual migrations; GORM models are persistence
  structs, not automatic schema sources.

## Flows

Open workspace:

```txt
choose local repo -> validate allowed Git repo -> create/restore metadata
-> refresh recursive file index -> open Vibe Mode with recent conversations
```

Show readiness while indexing and Git status after load.

Conversation and agent run:

```txt
open/create conversation -> load messages and visible activity
-> send user message with model + reasoning effort
-> create linked agent run -> stream assistant text/tool progress
-> execute safe tools or wait for approval -> append final assistant outcome
```

Agent runs should inspect relevant workspace context, produce a short plan for
non-trivial work, propose small reviewable patches or answer directly, then run
or recommend narrow verification. Final output reports changed files,
verification result, and remaining risks.

Provider settings:

- Backend-controlled tools enforce path, secret, ignore, and size checks.
- Vibe sends `model` and `reasoningEffort` with each user message.
- Initial models: `gpt-5.5`, `gpt-5.4`, `gpt-5.4-mini`; default `gpt-5.5`.
- Initial reasoning: `low`, `medium`, `high`, `xhigh`; default `medium`.
- `PATCHPILOT_OPENAI_API_KEY` is backend-only.
- `PATCHPILOT_OPENAI_BASE_URL` defaults to `https://api.openai.com/v1`; provider
  calls `/responses` under that base URL.

Tool approval:

```txt
approval-required batch -> show approvals one at a time
-> record approve/reject decisions -> execute only approved tools
-> append tool results to the run
```

Patch approval verifies clean apply, applies server-side, updates Git status, and
returns the result. Reject leaves files unchanged. Invalid applies fail safely.

Commands:

```txt
enter/select command -> classify -> start at workspace root
-> stream stdout/stderr -> show exit code/duration
```

Common user commands run immediately, risky commands require confirmation, and
obvious destructive commands are blocked. Dangerous agent commands need approval.

Preview:

```txt
run dev server -> poll listening sockets every 1s -> user exposes
-> open backend-origin preview URL
```

Agents never expose ports. Proxy route:
`/workspaces/:workspaceId/ports/:port/proxy/*`. Port responses include an
absolute backend-generated `exposedUrl`.

Commit:

```txt
review staged/unstaged status and diff -> stage explicit paths
-> enter message -> commit selected paths -> return hash
```

Stage/commit requests always send explicit paths from visible Git sections.
Push/pull/branch management are outside current scope.

## API

All endpoints except `GET /api/health` and `POST /api/auth/login` require a
session cookie. Workspace APIs are scoped by `workspaceId`. Responses are JSON
except SSE/proxy.

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

POST  /api/workspaces/:workspaceId/conversations
GET   /api/workspaces/:workspaceId/conversations
GET   /api/workspaces/:workspaceId/conversations/:conversationId
PATCH /api/workspaces/:workspaceId/conversations/:conversationId
POST  /api/workspaces/:workspaceId/conversations/:conversationId/messages
POST  /api/workspaces/:workspaceId/conversations/:conversationId/runs/:runId/cancel
POST  /api/workspaces/:workspaceId/conversations/:conversationId/runs/:runId/tool-calls/:toolCallId/approve
POST  /api/workspaces/:workspaceId/conversations/:conversationId/runs/:runId/tool-calls/:toolCallId/reject

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

Response contracts:

- REST errors: `{ "error": { "code": "snake_case", "message": "...", "details": {} } }`.
- Health: `{"status":"ok"}`; returns `503` with error envelope if DB is unavailable.
- Workspace list: `{"workspaces":[]}` newest-first.
- Files list: `{"entries":[]}` for a workspace-relative directory.
- File index: `{"entries":[]}` with workspace-relative `path`, `size`,
  `modifiedAt`; refresh rebuilds and returns the same shape.
- File read: `{"path":"...","content":"..."}` for readable text files up to
  1 MiB.
- Search: `{"results":[]}` for filename/content matches.
- File APIs ignore `.git`, `node_modules`, and `build`, skip symlinks and files
  over 1 MiB; direct invalid reads return the standard error envelope.
- Conversation create/update accept `{"title":"..."}`.
- Conversation list: `{"conversations":[]}` newest-first by last activity.
- Conversation detail:
  `{"conversation":{...},"messages":[],"runs":[],"toolCalls":[]}`.
- Message create accepts
  `{"content":"...","model":"gpt-5.5","reasoningEffort":"medium"}` and returns
  `202` with the user message and agent run.
- Git status returns `{"porcelain":"..."}` including ignored paths and expanded
  untracked files.
- Git diff returns `{"path":"...","diff":"..."}` for workspace or path; untracked
  diffs are shown without staging.
- Git stage/unstage/discard accept explicit non-empty workspace-relative
  `{"paths":["..."]}` and return updated status; discard only affects unstaged
  selected paths and removes selected untracked paths.
- Git commit accepts exact user `message` plus explicit non-empty `paths`, stages
  only those paths, commits, returns `{"hash":"..."}`, and never pushes.
- Commands accept `{"command":"...","confirmed":false}`; safe commands return
  `202`, risky commands return `409 confirmation_required`, blocked commands
  return `400 blocked_command`.
- Process list returns `{"processes":[]}` newest-first; process detail returns
  `{"command":{...},"output":[]}` with latest output replay.
- Process stop stops running commands; finished commands return current state.

Primary fields:

- Conversation: `id`, `workspaceId`, `title`, `createdAt`, `updatedAt`,
  `lastMessageAt`.
- Message: `id`, `workspaceId`, `conversationId`, `role`, `content`, `runId?`,
  `createdAt`.
- Agent run: `id`, `workspaceId`, `conversationId`, `triggerMessageId`, `model`,
  `reasoningEffort`, `status`, `summary`, `error?`, timestamps.
- Tool call: `id`, `workspaceId`, `conversationId`, `runId`, `batchId`,
  `sequence`, `name`, `input`, `output`, `status`, `requiresApproval`,
  `decision?`, timestamps.
- Command: `id`, `workspaceId`, `runId?`, `command`, `cwd`, `status`,
  `exitCode?`, `startedAt?`, `finishedAt?`, `createdAt`, `durationMs?`.
- Command output: `id`, `commandId`, `stream(stdout|stderr)`, `chunk`,
  `createdAt`.

SSE envelope:

```json
{
  "id": "evt_123",
  "workspaceId": "ws_123",
  "type": "agent.run.status_changed",
  "createdAt": "2026-05-20T10:00:00Z",
  "payload": {}
}
```

Events: `workspace.ready`, `workspace.indexing`, `conversation.created`,
`conversation.updated`, `conversation.message.created`, `agent.delta`,
`agent.tool.started`, `agent.tool.finished`, `agent.approval_required`,
`agent.run.status_changed`, `command.output`, `process.started`,
`process.exited`, `port.opened`, `port.exposed`, `git.changed`.

## Agent Tools And Commands

Tools:

- `list_files`, `read_file`, `search_files`.
- `git_status`, `git_diff`.
- `run_command` for approved commands with possible side effects.
- `apply_patch` for approval-required server-side patch apply.

Agents must not read outside workspace root, access secrets, expose ports, or run
approval-required tools without approval. Backend preserves provider tool-call
order. If any tool in a batch requires approval, no tool in that batch runs
until all approval-required calls have decisions. Rejected tools do not run.

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

Everything else requires explicit approval. Direct user commands use a broader
common-command allowlist but still execute without a shell, block control
operators/redirection/substitution, block workspace escapes, and block patterns
such as `rm -rf`, `git reset --hard`, `git clean`, `sudo`, `chmod -R`, and
`chown -R`.

## Data Model

SQLite stores app state; source files stay in their original repositories; Git
owns repo history. PatchPilot-owned state may live under `~/.patchpilot`.

Runtime config uses OS environment variables, falling back to local `.env`.
Variables:
`PATCHPILOT_ADDR`, `PATCHPILOT_ALLOWED_ROOTS`, `PATCHPILOT_STATIC_DIR`,
`PATCHPILOT_LOG_FORMAT`, `PATCHPILOT_ADMIN_TOKEN`,
`PATCHPILOT_OPENAI_API_KEY`, `PATCHPILOT_OPENAI_BASE_URL`,
`PATCHPILOT_DB_PATH`, `PATCHPILOT_DATA_DIR`. If `PATCHPILOT_DB_PATH` is unset,
`PATCHPILOT_DATA_DIR` controls the directory for `patchpilot.db`; otherwise the
default is `~/.patchpilot/patchpilot.db`.

Migrations are explicit, versioned, manually authored, tracked in SQLite
metadata, run before API traffic, and fail startup on error. GORM `AutoMigrate`
is not used for product schema changes.

Active tables:

- `app_metadata`: migration/version metadata and app key-value state.
- `auth_sessions`: `id`, `session_hash`, `created_at`, `last_seen_at`, `expires_at`.
- `workspaces`: `id`, `name`, `root_path`, `git_remote?`, `default_branch?`,
  `status(indexing|ready|error)`, timestamps.
- `file_index`: `workspace_id`, `path`, `size`, `modified_at`, `indexed_at`.
- `conversations`: `id`, `workspace_id`, `title`, timestamps, `last_message_at`.
- `conversation_messages`: `id`, `workspace_id`, `conversation_id`, `run_id?`,
  `role`, `content`, `created_at`.
- `agent_runs`: `id`, `workspace_id`, `conversation_id`, `trigger_message_id`,
  `model`, `reasoning_effort`, `status`, `summary?`, `error?`, timestamps.
- `agent_run_events`: `id`, `workspace_id`, `conversation_id`, `run_id`, `type`,
  `payload_json`, `created_at`.
- `agent_tool_calls`: `id`, `workspace_id`, `conversation_id`, `run_id`,
  `batch_id`, `sequence`, `name`, `input_json`, `output_json`, `status`,
  `requires_approval`, `decision?`, timestamps.
- `commands`: `id`, `workspace_id`, `run_id?`, `command`, `cwd`, `status`,
  `exit_code?`, `started_at?`, `finished_at?`, `created_at`.
- `command_output`: `id`, `command_id`, `stream(stdout|stderr)`, `chunk`,
  `created_at`.
- `ports`: `id`, `workspace_id`, `process_id?`, `port`,
  `status(detected|exposed|closed)`, `exposed_path?`, timestamps.
- `git_snapshots`: `id`, `workspace_id`, `commit_sha?`, `status_json`, `created_at`.

Statuses:

```txt
agent:   queued -> running -> waiting_tool_approval -> running -> done
agent:   queued -> running -> done|failed|canceled
command: queued -> running -> exited|stopped
command: queued -> failed
command: running -> failed
```

## Acceptance

- Open local Git repo workspace.
- Create, list, open, rename, and continue conversations per workspace.
- Send chat messages that start agent runs.
- Agent starts non-trivial work with a short plan.
- Agent reads/searches approved files before code changes.
- Agent returns assistant messages and tool calls, not direct file mutations.
- User approves/rejects approval-required tools.
- Server executes only approved mutating tools.
- Agent produces small reviewable patches and reports changed files.
- Agent runs or recommends narrow verification after edits.
- User sees streamed command output plus exit status.
- User views Git status/diff.
- User commits explicit non-empty selected paths.
- User previews app through same-host proxy.
- Mobile/iPad user completes a chat-driven AI coding loop from Vibe Mode.
