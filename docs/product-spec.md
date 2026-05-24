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
- v0.2 public product model is `conversation -> message -> agent run`;
  product APIs, DTOs, and database tables use conversation/run naming. `session`
  remains only for authentication cookies and auth session storage.
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
-> build bounded LLM context from conversation summary + recent messages
-> create linked agent run -> stream assistant text/tool progress
-> execute safe tools or wait for approval -> append final assistant outcome
```

Conversation records persist an active-run flag so the Vibe conversation list
can show in-flight work state from conversation data alone, without fetching
run lists for every sidebar row.

Agent runs should inspect relevant workspace context, produce a short plan for
non-trivial work, propose small reviewable patches or answer directly, then run
or recommend narrow verification. Final output reports changed files,
verification result, and remaining risks.

Conversation context is assembled server-side from SQLite before provider calls.
PatchPilot reserves room for agent instructions, the current prompt, tool
schemas, and active-run tool history before adding prior conversation content.
When older history would exceed the local context budget, the backend summarizes
only older messages, stores the summary on the conversation, and keeps the newest
messages verbatim. Agent instructions are sent separately from conversation
messages so conversation history cannot displace the system prompt.

Provider settings:

- Backend-controlled tools enforce path, secret, ignore, and size checks.
- Vibe sends `model` and `reasoningEffort` with each user message.
- Vibe renders assistant text as Markdown with GitHub-flavored Markdown support;
  raw HTML in messages is escaped.
- Vibe keeps the selected workspace and current conversation in URL state:
  `workspaceId` identifies the workspace and `conversationId` identifies the
  open conversation; an absent `conversationId` starts a new conversation.
- Vibe timeline auto-scroll follows the latest activity only while the user is
  already at or near the bottom of the thread.
- Scrolling up to read older activity pauses auto-follow until the user returns
  to the bottom or uses a visible jump-to-latest control.
- New activity that arrives while auto-follow is paused shows a compact control
  that jumps back to the latest activity and re-enables follow mode.
- Markdown fenced code blocks show syntax highlighting, language context, and a
  copy action for the raw code.
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

Workspace Git review:

```txt
review staged/unstaged paths -> choose section or row action
-> confirm discard when needed -> review commit paths -> commit staged paths
```

Discard always requires confirmation that names the affected path count before
calling the API. Commit opens a review dialog that shows the exact message and
staged paths that will be sent to the commit API.

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
GET   /api/workspaces/:workspaceId/conversations?q=
GET   /api/workspaces/:workspaceId/conversations/:conversationId
PATCH /api/workspaces/:workspaceId/conversations/:conversationId
POST  /api/workspaces/:workspaceId/conversations/:conversationId/messages
POST  /api/workspaces/:workspaceId/conversations/:conversationId/runs/:runId/cancel
GET   /api/workspaces/:workspaceId/conversations/:conversationId/runs/:runId/events
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

Large REST lists support cursor pagination with `limit` and `cursor`. The
default `limit` is `50`; the max is `100`, and larger or non-positive limits
return `400 invalid_limit`. Cursors are opaque strings from the previous
response. Invalid cursors return `400 invalid_cursor`. Paginated responses keep
their existing array field and add optional `nextCursor`.

Paginated endpoints:

- `GET /api/workspaces`: newest-first by `updatedAt`, then `id`.
- `GET /api/workspaces/:workspaceId/files/index`: ascending by `path`.
- `GET /api/workspaces/:workspaceId/search`: ascending by `path`, `kind`, then
  line.
- `GET /api/workspaces/:workspaceId/conversations`: newest-first by
  `lastMessageAt`, `updatedAt`, then `id`.
- `GET /api/workspaces/:workspaceId/processes`: newest-first by `createdAt`,
  then `id`.
- `GET /api/workspaces/:workspaceId/ports`: ascending by port number.

Response contracts:

- REST errors: `{ "error": { "code": "snake_case", "message": "...", "details": {} } }`.
- Health: `{"status":"ok"}`; returns `503` with error envelope if DB is unavailable.
- Auth login accepts `{"token":"..."}` and returns
  `{"session":{"id":"auth_...","expiresAt":"...","lastSeenAt":"..."}}`.
  It sets an HTTP-only `patchpilot_session` cookie scoped to `/`; `Secure` is
  set for HTTPS requests. Invalid tokens return `401 invalid_auth_token`.
- Auth session returns the same `{"session":{...}}` shape for a valid cookie and
  `401 unauthorized` when the cookie is missing, expired, or invalid. Logout
  accepts no body, clears the session cookie, and returns `{"status":"ok"}`.
- Workspace create accepts `{"rootPath":"/absolute/git/repo"}` and returns a
  workspace object. Invalid roots return `400 invalid_workspace_root`,
  disallowed roots return `400 workspace_root_not_allowed`, and non-Git roots
  return `400 not_git_repository`.
- Workspace list returns `{"workspaces":[],"nextCursor":"..."}` newest-first.
  Workspace get returns one workspace object and refreshes the file index.
  Workspace delete removes PatchPilot metadata for that workspace and returns
  `{"status":"deleted"}`. Unknown workspace IDs return `404 workspace_not_found`.
- Files list: `{"entries":[]}` for a workspace-relative directory.
- File index: `{"entries":[],"nextCursor":"..."}` with workspace-relative
  `path`, `size`, `modifiedAt`; refresh rebuilds and returns the same shape.
- File read: `{"path":"...","content":"..."}` for readable text files up to
  1 MiB.
- File write accepts `{"path":"...","content":"..."}` for an existing readable
  text file up to 1 MiB and returns `{"path":"...","content":"..."}` with the
  written content. It does not create files. It rejects invalid paths, workspace
  escapes, ignored paths, symlink paths, secret-like filenames (`.env`,
  `.env.*`, `*.pem`, `*.key`, `id_rsa`, `id_ed25519`, `.npmrc`, `.pypirc`,
  `.netrc`), binary content, and oversized existing or replacement content.
  Missing files return `404 path_not_found`; rejected writes use the standard
  error envelope. Successful writes refresh the file index and publish current
  Git status through `git.changed`.
- Search: `{"results":[],"nextCursor":"..."}` for filename/content matches.
- File APIs ignore `.git`, `node_modules`, and `build`, skip symlinks and files
  over 1 MiB; direct invalid reads return the standard error envelope.
- Conversation create/update accept `{"title":"..."}`.
- Conversation list: `{"conversations":[],"nextCursor":"..."}` newest-first by
  last activity. Optional `q` trims whitespace and filters conversations by
  case-insensitive title match while preserving cursor pagination.
- Conversation detail:
  `{"conversation":{...},"messages":[],"runs":[],"toolCalls":[]}`.
- Message create accepts
  `{"content":"...","model":"gpt-5.5","reasoningEffort":"medium"}` and returns
  `202` with the user message and agent run. The run continues on the backend
  if the request client disconnects.
- Backend shutdown finalizes active agent runs (`queued`, `running`,
  `waiting_tool_approval`) as `failed` with a durable shutdown error. It also
  finalizes backend-owned commands/processes in `queued` or `running` as
  `stopped`.
- Run cancel marks non-terminal runs `canceled`, stops active run-owned command
  tools, and is safe to call more than once. It returns the run object. Terminal
  runs return their current state. Missing runs return `404 agent_run_not_found`.
- Tool approve/reject endpoints accept no body and return
  `{"toolCall":{...}}`. Approve runs the selected pending approval-required
  tool, while reject records the rejection and does not run the tool. Missing
  calls return `404 agent_tool_call_not_found`; non-waiting calls return
  `409 agent_tool_not_approvable`.
- Run events stream returns SSE for one run. It replays durable stored run
  events after `Last-Event-ID`, then continues live. Without `Last-Event-ID`,
  it replays durable stored events for the run from the beginning.
- Git status returns `{"porcelain":"..."}` including expanded untracked files. By default, it does not include ignored paths. It can be configured using parameters:
  - `ignored` (boolean, default: false): whether to include ignored files in the status.
  - `untracked` (string: "all", "normal", "no", default: "all"): untracked files mode.
  - `ignore_submodules` (string: "none", "untracked", "dirty", "all", default: ""): ignore changes to submodules mode.
  - `paths` (string array / workspace-relative paths): limit status check to specific paths.
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
- Process list returns `{"processes":[],"nextCursor":"..."}` newest-first;
  process detail returns `{"command":{...},"output":[]}` with latest output
  replay.
- Process stop stops running commands; finished commands return current state.
- Port list refreshes current reachability and returns
  `{"ports":[],"nextCursor":"..."}` with detected, exposed, and closed ports.
  Port expose accepts no body and returns
  `{"port":{...}}` with `exposedPath` and same-origin `exposedUrl`. Closed or
  unreachable ports return `502 port_unreachable` and are marked closed. Unknown
  ports return `404 port_not_found`; invalid port path values return
  `400 invalid_port`.

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
`agent.output.snapshot`, `agent.tool.started`, `agent.tool.finished`,
`agent.approval_required`, `agent.run.status_changed`, `command.output`,
`process.started`, `process.exited`, `port.opened`, `port.exposed`,
`git.changed`.
`agent.delta` events carry live token/text deltas, are transient, and are not
stored in SQLite. Transient deltas may be lost during disconnects; final
assistant messages, run summaries, tool calls, and run status are the durable
recovery source.
`agent.output.snapshot` is a transient run-stream event emitted from in-memory
active-run draft text on reconnect. It is not stored in SQLite and only restores
in-flight text while the same backend process still owns the run.
After a backend restart, active runs do not resume; the durable `failed` run
state and durable `stopped` process state from shutdown cleanup are the
recovery source of truth.
Conversation responses include a `hasRunningRun` boolean derived from durable
run state for the same conversation.

`GET /api/workspaces/:workspaceId/events` streams workspace/process/git/port
events for the workspace. Run activity uses
`GET /api/workspaces/:workspaceId/conversations/:conversationId/runs/:runId/events`.
Run streams replay durable stored run events using `Last-Event-ID` and exclude
transient `agent.delta`. Historical conversation state comes from conversation
detail, and historical command output comes from process detail.

## Agent Tools And Commands

Tools:

- `list_files`, `read_file`, `search_files`.
- `git_status`, `git_diff`.
- `run_command` for approved commands with possible side effects.
- `apply_patch` for approval-required server-side patch apply.

Vibe Mode renders tool calls as compact activity rows with icons, human status
text, and concise tool-specific labels such as read path, edited path, command,
or search query. Consecutive tool calls in the same run are grouped into a
collapsible activity block. Individual tool calls may be expandable or
non-expandable by tool type: approval, patch, command, diff, search, status, and
list calls can expose relevant detail, while `read_file` remains a one-line
activity and does not expose file output. Tool call groups and expandable calls
open by default when a decision or attention is needed, including waiting
approval, running, or failed states; completed calls stay collapsed by default.

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

Direct user commands are classified before queueing:

- Safe commands run immediately with `202`: `git status|diff|log`,
  `npm run test|lint|build|dev`, `pnpm test|lint|build|dev`,
  `pnpm --dir <safe-relative-dir> test|lint|build|dev`,
  `yarn test|lint|build|dev`,
  `yarn --dir <safe-relative-dir> test|lint|build|dev`, `bun test`,
  `bun run lint|build|dev`, `go test ./...`, `go test ./<package>`,
  `go build ./...`, `pytest`, `python -m pytest`, `python3 -m pytest`,
  `cargo test|build`, and `make test|lint|build|dev`.
- Risky commands are syntactically valid but outside the exact allowlist. They
  return `409 confirmation_required` unless `confirmed:true` is supplied.
- Blocked commands always return `400 blocked_command`. Blocks include shell
  control or shell expansion syntax (`&&`, `||`, `;`, `|`, `>`,
  `<`, backticks, `$(`, or newlines); absolute executable paths; absolute path
  arguments; workspace escape arguments; `sudo`; `su`; `rm` with recursive
  forced flags; `git clean`; `git reset --hard`; `chmod -R`; and `chown -R`.

Command execution always uses argument parsing without a shell, runs at the
workspace root, and never accepts traversal or shell operators. Both
`confirmation_required` and `blocked_command` responses include
`details.decision` with `level`, `reason`, and parsed `parts`.

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
- `conversations`: `id`, `workspace_id`, `title`, timestamps,
  `last_message_at`, `context_summary`, `context_summary_through_message_id?`,
  `context_summary_updated_at?`.
- `messages`: `id`, `workspace_id`, `conversation_id`, `run_id?`, `role`,
  `content`, `created_at`.
- `agent_runs`: `id`, `workspace_id`, `conversation_id`, `trigger_message_id`,
  `model`, `reasoning_effort`, `status`, `summary?`, `error?`, timestamps.
- `agent_run_events`: `id`, `workspace_id`, `run_id`, `type`, `payload_json`,
  `created_at`.
- `agent_tool_calls`: `id`, `workspace_id`, `run_id`, `batch_id`, `sequence`,
  `name`, `input_json`, `output_json`, `status`, `requires_approval`,
  `decision?`, timestamps.
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

## Frontend Structure

Feature screens keep route entry files thin. Vibe feature code is grouped under
`web/src/features/vibe` with `hooks` for orchestration, `layout` for shell
regions, `components` for Vibe-only UI, and `lib` for local pure helpers.

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
- Auth/session expiry: expired, missing, or invalid session cookies return
  `401 unauthorized` and valid logout clears the session cookie. Verification:
  backend auth/API handler tests.
- Indexing failure: workspace create/get/index refresh return the standard error
  envelope without exposing host paths outside the workspace root, and no stale
  successful index response is sent for the failed refresh. Verification:
  backend API handler tests with a failing file index/read path.
- SSE replay: run event streams replay durable run events after
  `Last-Event-ID`, exclude transient `agent.delta`, and emit an in-memory
  `agent.output.snapshot` only for active local runs. Verification: backend SSE
  handler tests.
- Command truncation: command output persistence keeps only the latest 1 MiB per
  command and process detail replays only retained output. Verification:
  database command-output tests and API process detail tests.
- Closed ports: unreachable exposed or detected ports are marked `closed`, emit
  `port.closed`, and expose/proxy requests return `502 port_unreachable`.
  Verification: backend port/API handler tests.
- Patch apply conflict: failed patch application marks the tool call failed,
  leaves source files unchanged, records an actionable tool error, and keeps the
  run recoverable without executing later approval-required tools in that batch.
  Verification: backend agent/tool approval tests.
- Invalid paths: file, Git, command, and agent tool paths reject absolute paths,
  traversal, and symlink escapes with standard error envelopes and without host
  path leakage. Verification: filestore, gitrepo, runner, agent, and API tests.
- Secret protection: agent file reads reject `.env`, `.env.*`, `*.pem`,
  `*.key`, `id_rsa`, `id_ed25519`, `.npmrc`, `.pypirc`, and `.netrc`; manual
  file writes reject the same secret-like paths. Verification: agent tool,
  filestore, and API tests.
