# PatchPilot Product Spec

`docs/project-rules.md` owns locked rules. This file owns active v0.4 scope, flows, API, data, and acceptance.

## Objective

PatchPilot v0.4 is a self-hosted, single-user AI coding workspace centered on workspace conversations with managed agent context/runtime:

```txt
open repo -> index instructions/skills/MCP -> open/create conversation
-> send message -> build agent context -> stream agent text/tools
-> approve/reject mutating or unknown tools -> execute approved tools
-> summarize outcome -> run/review verification -> commit selected paths
```

Core decisions:

- Local filesystem + SQLite; multiple conversations per workspace.
- Public model: `conversation -> message -> agent run`. Product APIs, DTOs, and DB tables use conversation/run naming; `session` is auth-only.
- REST mutations, SSE realtime, and WebSocket only for Workspace terminal sessions.
- Admin-token login with HTTP-only session cookie.
- Agent commands run as the server OS user at workspace root, without a shell. Workspace terminal sessions run a real shell through PTY at workspace root.
- Agent changes happen through tool calls; mutating tools require approval.
- Agent context may include repo `AGENTS.md`, enabled local skills, MCP tool metadata, bounded conversation context, and active-run tool history.
- Skills are local PatchPilot-managed directories; remote install/marketplaces are outside v0.4.
- MCP supports explicit per-workspace stdio/HTTP server configs; tools execute only through the backend bridge.
- Workspace Mode supports files, search, diffs, small edits, interactive terminal sessions, preview, and Git status.
- Settings is a compact app-wide local/server configuration screen for appearance, agent defaults, local skills, MCP status/config, and safe runtime status.
- Manual edits are limited to small text files under workspace root.
- Agent commands auto-run only when exactly allowlisted below.
- Agent command replay keeps latest 1 MiB per command. Terminal sessions keep a bounded in-memory 1 MiB replay buffer per active session and do not persist transcripts.
- Commits require explicit selected paths; no push.
- Schema changes use explicit manual migrations; GORM models are persistence structs, not schema sources.

## Flows

Open workspace:

```txt
choose local repo -> validate allowed Git repo -> create/restore metadata
-> refresh recursive file index -> open Vibe Mode with recent conversations
```

Readiness covers file indexing, effective instructions, skill metadata, MCP server status, and Git status.

Conversation/run:

```txt
open/create conversation -> load messages/activity
-> send user message with model + reasoning effort
-> build bounded context from effective instructions + selected skills
   + MCP registry + conversation summary + recent messages
-> create linked run -> stream assistant text/tool progress
-> execute safe tools or wait for approval -> append final assistant outcome
```

- Conversation records persist `hasRunningRun` so sidebars show in-flight state without listing runs for every row.
- Runs inspect relevant workspace context, produce a short plan for non-trivial work, propose reviewable patches or answer directly, then run/recommend narrow verification.
- Final output reports changed files, verification, and remaining risks.
- Context is assembled server-side from SQLite/workspace metadata. PatchPilot reserves room for provider control instructions, repo instructions, enabled skills, MCP registry, current prompt, tool schemas, and active-run tool history before prior conversation content.
- Provider control instructions are sent as explicit developer-role messages whose `content` array contains XML-tagged system prompt blocks. Agent generation keeps PatchPilot rules and skill instructions in developer content blocks; repo `AGENTS.md` instructions, structured environment context (`cwd`, `shell`, `current_date`, `timezone`), and context warnings are sent separately as a user-role context message before conversation history. Summarization uses developer content blocks for the summary task and rules.
- Older history over budget is summarized onto the conversation; newest messages stay verbatim. Developer instructions are separate from conversation messages so history cannot displace the control prompt.

Composer assist (v0.4):

- `/` opens a grouped command list. The first v0.4 group is local skills from
  the agent context snapshot.
- `@` opens a grouped mention list. Skills appear immediately; derived folders
  and indexed files appear only after the user types a mention query.
- Selecting a skill inserts Markdown text `[$skill-name](skill-path)`, using the
  skill key/name as the label and the skill path exposed by agent context.
- Selecting a file or folder inserts Markdown text `[name](path)`, using the
  basename as the label and a workspace-relative path as the target. Folder
  targets keep a trailing `/`.
- The composer rich input uses a Tiptap editor with atomic inline tokens for
  inserted skills, folders, and files. Tokens render as compact chips, preserve
  their original Markdown text for serialization, and delete as single units.
- Composer suggestions only edit the submitted prompt text. They never submit a
  run, read file contents, attach files, or change the message API payload.
- Suggestions support keyboard navigation with arrow keys, Enter/Tab to insert,
  and Escape to close. Group headings and disabled rows are skipped by keyboard
  selection.

Agent instructions:

```txt
workspace opened/context refreshed -> discover applicable AGENTS.md
-> validate path/size/secret rules -> include effective instructions in future runs
```

PatchPilot reads root and task-relevant descendant `AGENTS.md` files. Effective instructions preserve source order, precedence, and skipped-file warnings. Files outside root, symlink escapes, secret-like paths, binaries, and oversized files are rejected. Discovery reads filesystem during context refresh/run creation; v0.4 has no DB registry/cache table for instruction sources.

Skills:

```txt
add local skill directory -> index SKILL.md -> enable/disable
-> select relevant enabled skill metadata/path for a run
-> agent reads SKILL.md through run_command cat/sed when needed
```

Skills are local directories with `SKILL.md`. A valid `SKILL.md` starts with YAML frontmatter containing non-empty `name` and `description` strings, followed by a non-empty instruction body. Discovery roots: `~/.patchpilot/skills` (user override) then `~/.agents/skills` (fallback). Duplicate keys use only the `~/.patchpilot/skills` copy for effective skills and agent context. Enablement comes from `~/.patchpilot/config.json`; skills missing from `config.skills` default enabled. Runs inject enabled valid skill names, descriptions, and `~` paths into the prompt; the agent reads the listed path with `run_command` using `cat` or `sed` when it needs the body. Duplicate enabled valid skill names after precedence are marked invalid.

MCP:

```txt
add stdio/http MCP server -> check health -> discover tools/resources
-> expose namespaced tools to provider -> execute through approval policy
```

MCP configs come only from `mcpServers` in `~/.patchpilot/config.json`. Stdio servers are managed backend child processes. HTTP servers use configured URLs only; no network scanning or public discovery. MCP tools use namespaced IDs and the same durable tool-call, event, and approval flow as built-ins.

Provider/Vibe settings:

- Backend-controlled tools enforce path, secret, ignore, and size checks.
- Vibe sends `model` and `reasoningEffort` with each user message.
- Assistant text renders as GFM Markdown; raw HTML is escaped.
- URL state holds `workspaceId` and optional `conversationId`; absent `conversationId` starts a new conversation.
- Timeline auto-follow runs only while the user is at/near bottom; scrolling up pauses follow until returning to bottom or using the jump-to-latest control.
- New activity while paused shows a compact jump-to-latest control.
- Fenced code blocks show syntax highlighting, language, and copy action.
- Initial models: `gpt-5.5`, `gpt-5.4`, `gpt-5.4-mini`; default `gpt-5.5`.
- Initial reasoning: `low`, `medium`, `high`, `xhigh`; default `medium`.
- `PATCHPILOT_OPENAI_API_KEY` is backend-only.
- `PATCHPILOT_OPENAI_BASE_URL` defaults to `https://api.openai.com/v1`; provider calls `/responses`.
- `PATCHPILOT_LIGHT_MODEL` defaults to `gpt-5.4-mini` and is used for lightweight AI tasks such as conversation title generation.

Tool approval:

```txt
approval-required batch -> show approvals one at a time
-> record approve/reject decisions -> execute only approved tools
-> append tool results
```

Patch approval verifies clean apply, applies server-side, updates Git status, and returns the result. Reject leaves files unchanged; invalid applies fail safely. MCP tools require approval unless PatchPilot policy and server/tool metadata both mark the tool read-only and safe. Approval review shows server, tool name, source, input summary, and policy reason.

Workspace terminal:

```txt
create/open terminal session -> start PTY shell at workspace root
-> bridge input/output/resize over WebSocket -> close session deliberately
```

Users may create multiple terminal sessions per workspace. The Workspace UI keeps the terminal in a persistent bottom panel while users switch the primary Files, Git, and Preview panels; terminal session tabs live on the right side of the bottom panel. Each session starts in the workspace root using the server OS user and shell fallback `$SHELL`, `/bin/zsh`, `/bin/bash`, then `/bin/sh`. Terminal transcripts are not persisted; active sessions keep only the latest 1 MiB in memory for reconnect. Terminal WebSocket messages are the only bidirectional realtime channel in v0.3.

Preview:

```txt
run dev server -> poll listening sockets every 1s -> user exposes
-> open backend-origin preview URL
```

Agents never expose ports. Proxy route: `/workspaces/:workspaceId/ports/:port/proxy/*`. Port responses include backend-generated absolute `exposedUrl`.

Git/commit:

```txt
review status/diff -> stage explicit paths -> enter message
-> commit selected paths -> return hash
```

Stage/commit requests send explicit paths from visible Git sections. Discard requires confirmation naming affected path count. Commit dialog shows exact message and staged paths. Push/pull/branch management are outside scope.

## API

All endpoints except `GET /api/health` and `POST /api/auth/login` require a session cookie. Workspace APIs are scoped by `workspaceId`. Responses are JSON except SSE/proxy.

```txt
GET  /api/health

POST /api/auth/login
GET  /api/auth/session
POST /api/auth/logout

GET    /api/settings
PATCH  /api/settings/preferences
GET    /api/settings/fonts
POST   /api/settings/fonts
GET    /api/settings/fonts/:fontId/file
DELETE /api/settings/fonts/:fontId

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

GET  /api/workspaces/:workspaceId/agent/context
POST /api/workspaces/:workspaceId/agent/context/refresh

GET   /api/workspaces/:workspaceId/skills
POST  /api/workspaces/:workspaceId/skills
PATCH /api/workspaces/:workspaceId/skills/:skillId
POST  /api/workspaces/:workspaceId/skills/refresh

GET   /api/workspaces/:workspaceId/mcp/servers
POST  /api/workspaces/:workspaceId/mcp/servers
PATCH /api/workspaces/:workspaceId/mcp/servers/:serverId
POST  /api/workspaces/:workspaceId/mcp/servers/:serverId/refresh
GET   /api/workspaces/:workspaceId/mcp/servers/:serverId/tools

GET   /api/workspaces/:workspaceId/terminal/sessions
POST  /api/workspaces/:workspaceId/terminal/sessions
PATCH /api/workspaces/:workspaceId/terminal/sessions/:sessionId
POST  /api/workspaces/:workspaceId/terminal/sessions/:sessionId/close
GET   /api/workspaces/:workspaceId/terminal/sessions/:sessionId/socket

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

Pagination:

- Large REST lists use `limit`/`cursor`; default `limit=50`, max `100`.
- Larger/non-positive limits return `400 invalid_limit`; invalid cursors return `400 invalid_cursor`.
- Cursors are opaque strings from previous responses.
- Paginated responses keep their array field and add optional `nextCursor`.
- Sorts: workspaces newest by `updatedAt,id`; file index by `path`; search by `path,kind,line`; conversations newest by `lastMessageAt,updatedAt,id`; skills/MCP servers/tools by `name,id`; terminal sessions newest by `updatedAt,id`; ports by port number.

Response contracts:

- REST errors: `{ "error": { "code": "snake_case", "message": "...", "details": {} } }`.
- Health: `{"status":"ok"}`; DB unavailable returns `503` with error envelope.
- Auth login accepts `{"token":"..."}`, returns `{"session":{"id":"auth_...","expiresAt":"...","lastSeenAt":"..."}}`, and sets HTTP-only `patchpilot_session` scoped to `/`; `Secure` is set on HTTPS. Invalid token: `401 invalid_auth_token`.
- Auth session returns same session shape for valid cookie; missing/expired/invalid cookie: `401 unauthorized`. Logout clears cookie and returns `{"status":"ok"}`.
- Workspace create accepts `{"rootPath":"/absolute/git/repo"}`. Invalid/disallowed/non-Git roots return `400 invalid_workspace_root`, `400 workspace_root_not_allowed`, or `400 not_git_repository`.
- Workspace list returns `{"workspaces":[],"nextCursor":"..."}`. Get returns one workspace and refreshes file index. Delete removes PatchPilot metadata and returns `{"status":"deleted"}`. Unknown workspace: `404 workspace_not_found`.
- Files list: `{"entries":[]}` for a workspace-relative directory.
- File index: `{"entries":[],"nextCursor":"..."}` with `path`, `size`, `modifiedAt`; refresh rebuilds and returns same shape.
- File read: `{"path":"...","content":"..."}` for readable text files up to 1 MiB.
- File write accepts `{"path":"...","content":"..."}` for an existing readable text file up to 1 MiB and returns written content. It does not create files. It rejects invalid paths, workspace escapes, ignored/symlink paths, secret-like names (`.env`, `.env.*`, `*.pem`, `*.key`, `id_rsa`, `id_ed25519`, `.npmrc`, `.pypirc`, `.netrc`), binary content, and oversized existing/replacement content. Missing files: `404 path_not_found`. Success refreshes index and emits `git.changed`.
- File APIs ignore `.git`, `node_modules`, `build`, symlinks, and files over 1 MiB; invalid reads use standard error envelope.
- Search returns `{"results":[],"nextCursor":"..."}` for filename/content matches.
- Conversation create accepts optional `{"title":"..."}`; empty/missing title stores `New conversation`. Conversation update accepts non-empty `{"title":"..."}`. List returns `{"conversations":[],"nextCursor":"..."}` newest-first; optional `q` trims whitespace and filters title case-insensitively. Detail returns `{"conversation":{...},"messages":[],"runs":[],"toolCalls":[]}`.
- Message create accepts `{"content":"...","model":"gpt-5.5","reasoningEffort":"medium"}`, returns `202` with user message and run, and backend run continues if client disconnects.
- The first message in an untitled conversation triggers best-effort asynchronous title generation with `PATCHPILOT_LIGHT_MODEL`; generation failure never fails message/run creation. Successful generated title updates emit `conversation.updated` with the updated conversation object.
- Backend shutdown finalizes active runs (`queued`, `running`, `waiting_tool_approval`) as `failed` with durable shutdown error, stops active run-owned commands, and closes active terminal sessions.
- Run cancel marks non-terminal runs `canceled`, stops active run-owned command tools, is idempotent, and returns the run. Terminal runs return current state. Missing run: `404 agent_run_not_found`.
- Tool approve/reject accept no body and return `{"toolCall":{...}}`. Approve runs the selected pending approval-required tool; reject records rejection. Missing/non-waiting calls return `404 agent_tool_call_not_found` or `409 agent_tool_not_approvable`.
- Agent context returns effective instruction sources, skill summaries and bodies for UI detail, MCP server/tool summaries, context-budget warnings, and refresh time. Refresh rereads instructions, enabled skills, and MCP discovery state where possible; failures use standard errors without leaking host paths.
- Skill create accepts a local directory path. Patch accepts `{"enabled":true|false}` plus optional display metadata. Refresh reparses enabled/disabled directories. Invalid skill directories stay visible with warnings.
- MCP server create accepts `{"name":"...","transport":"stdio|http",...}` with transport-specific config. Patch can enable/disable, update policy, or replace config. Refresh updates health/tools/resources. Tool list returns cached metadata, source server, read-only hints, and effective approval policy.
- Settings returns safe local/server configuration status and user preferences from PatchPilot-owned config. It never returns secrets, raw env values, secret placeholders, or host paths outside safe workspace/config summaries. Preferences may update only non-secret app-owned values: theme, app/code/terminal font selections, custom OS-resolved font-family stacks, and default model/reasoning effort.
- Font install stores uploaded `.woff2`, `.woff`, `.ttf`, or `.otf` files under PatchPilot-owned data, with metadata in `~/.patchpilot/config.json`. Font file responses are same-origin and scoped by generated `font_` ids. Upload rejects invalid names, extensions, MIME/content shape, oversized files, traversal, and duplicate unsafe filenames. Deleting a font selected by any font role returns `409 font_in_use` unless the role is reassigned first.
- Run event stream replays durable run events after `Last-Event-ID`; without it, replays durable run events from the beginning, then continues live.
- Git status returns `{"porcelain":"..."}` with expanded untracked files. Optional params: `ignored` boolean default `false`; `untracked` `"all"|"normal"|"no"` default `"all"`; `ignore_submodules` `"none"|"untracked"|"dirty"|"all"` default `""`; `paths` workspace-relative array.
- Git diff returns `{"path":"...","diff":"..."}` for workspace/path; untracked diffs show without staging.
- Git stage/unstage/discard accept explicit non-empty `{"paths":["..."]}` and return updated status; discard affects only selected unstaged paths and selected untracked paths.
- Git commit accepts exact user `message` plus explicit non-empty `paths`, stages only those paths, commits, returns `{"hash":"..."}`, and never pushes.
- Terminal session list returns `{"sessions":[],"nextCursor":"..."}`. Create accepts optional `{"title":"...","rows":24,"cols":80}` and returns `201 {"session":{...}}`. Patch accepts optional `title`, `rows`, and `cols` and returns the updated session. Close is idempotent and returns the current session. Unknown sessions return `404 terminal_session_not_found`.
- Terminal WebSocket upgrades at `/socket`. Client messages are `{"type":"input","data":"..."}`, `{"type":"resize","rows":24,"cols":80}`, and `{"type":"ping"}`. Server messages are `{"type":"ready","session":{...}}`, `{"type":"output","data":"..."}`, `{"type":"closed","exitCode":0}`, and `{"type":"error","message":"..."}`.
- Port list refreshes reachability and returns `{"ports":[],"nextCursor":"..."}`. Expose returns `{"port":{...}}` with `exposedPath` and same-origin `exposedUrl`. Closed/unreachable ports return `502 port_unreachable` and are marked closed; unknown `404 port_not_found`; invalid path value `400 invalid_port`.

Primary fields:

- Conversation: `id`, `workspaceId`, `title`, `createdAt`, `updatedAt`, `lastMessageAt`.
- Message: `id`, `workspaceId`, `conversationId`, `role`, `content`, `runId?`, `createdAt`.
- Agent run: `id`, `workspaceId`, `conversationId`, `triggerMessageId`, `model`, `reasoningEffort`, `status`, `summary`, `error?`, timestamps.
- Tool call: `id`, `workspaceId`, `conversationId`, `runId`, `batchId`, `sequence`, `name`, `source(builtin|mcp|skill)`, `sourceRef?`, `input`, `output`, `status`, `requiresApproval`, `decision?`, timestamps.
- Agent instruction source: `id`, `workspaceId`, `path`, `precedence`, `status`, `warning?`, `indexedAt`.
- Skill: `id`, `workspaceId`, `name`, `description`, `directory`, `enabled`, `status`, `warning?`, `updatedAt`.
- MCP server: `id`, `workspaceId`, `name`, `transport`, `enabled`, `status`, `approvalPolicy`, `lastError?`, timestamps.
- MCP tool: `id`, `workspaceId`, `serverId`, `name`, `description`, `inputSchema`, `readOnlyHint?`, `approvalPolicy`, `discoveredAt`.
- Terminal session: `id`, `workspaceId`, `title`, `cwd`, `status(open|closed|failed)`, `pid?`, `rows`, `cols`, `exitCode?`, timestamps.

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

Events: `workspace.ready`, `workspace.indexing`, `conversation.created`, `conversation.updated`, `conversation.message.created`, `agent.delta`, `agent.output.snapshot`, `agent.tool.started`, `agent.tool.finished`, `agent.approval_required`, `agent.run.status_changed`, `terminal.session.created`, `terminal.session.updated`, `terminal.session.closed`, `port.opened`, `port.exposed`, `git.changed`, `agent.context.refreshed`, `skill.changed`, `mcp.server.status_changed`, `mcp.tools.refreshed`.

- `agent.delta` carries live token/text, is transient, and is not stored. Durable recovery source: final assistant messages, run summaries, tool calls, and run status.
- `agent.output.snapshot` is transient, in-memory, not stored, and only restores in-flight text while the same backend process owns the run.
- `conversation.updated` is a workspace-level event with the updated conversation object as payload.
- After backend restart, active runs and terminal sessions do not resume; durable `failed` run state and `closed` terminal session state from shutdown cleanup are source of truth.
- Conversation responses include `hasRunningRun` derived from durable run state.
- Workspace stream `GET /api/workspaces/:workspaceId/events` covers workspace/terminal/git/port events. Terminal output uses its session WebSocket. Run stream covers run activity. Run streams replay durable events via `Last-Event-ID` and exclude transient `agent.delta`. Historical conversation state comes from conversation detail.

## Agent Tools And Commands

Tools: `list_files`, `search_files`, `run_command`, approval-required `apply_patch`, and `mcp:<server>:<tool>` through backend bridge/policy. Agents inspect Git through `run_command` with allowlisted commands such as `git status`, `git diff`, and `git log`; dedicated agent Git status/diff tools are not exposed.

Agents read file contents through `run_command`. Use `sed -n '1,160p' path/to/file` for ranged reads and `cat path/to/file` only when a full file is needed. Safe relative non-secret workspace file reads may auto-run. Skill file reads under `~/.patchpilot/skills` and `~/.agents/skills` may auto-run. Other outside-workspace file reads, absolute paths, and secret-like read paths (`.env`, `.env.*`, `*.pem`, `*.key`, `id_rsa`, `id_ed25519`, `.npmrc`, `.pypirc`, `.netrc`) require approval. Workspace escapes, unsupported read shapes, globs, extra flags, broad directory reads, and shell syntax are blocked.

`search_files` accepts a text `query` plus optional workspace-relative `path`. Empty or omitted `path` searches from workspace root; non-empty `path` must stay inside the workspace and may target either a directory subtree or one file.

Vibe renders tool calls as compact activity rows with icons, human status text, concise labels, source metadata, and grouped consecutive calls per run. Approval, patch, command, diff, search, status, and list calls can expand. `run_command` calls that match the safe file-read command shapes render as one-line read-file activity and do not expose file output. Groups/calls open by default when attention is needed: waiting approval, running, or failed. Completed calls stay collapsed.

Agents must not expose ports, call MCP servers directly, or run approval-required tools without approval. Outside-workspace and secret-like file reads require explicit approval except enabled skill files under configured skill roots. Backend preserves provider tool-call order. If any tool in a batch requires approval, no tool in that batch runs until all approval-required calls have decisions. Rejected tools do not run.

Agent auto-run requires exact allowlist match and no shell control operators:

- `git status`, `git diff`, `git log`
- `cat <safe-relative-file>`, `sed -n '<start>,<end>p' <safe-relative-file>`
- `cat ~/.patchpilot/skills/<key>/SKILL.md`, `cat ~/.agents/skills/<key>/SKILL.md`, and equivalent `sed -n '<start>,<end>p' ...`
- `npm run test|lint|build|dev`
- `pnpm test|lint|build|dev`
- `yarn test|lint|build|dev`
- `bun test`, `bun run lint|build|dev`
- `go test ./...`, `go test <package>`, `go build ./...`
- `pytest`, `python -m pytest`, `python3 -m pytest`
- `cargo test`, `cargo build`
- `make test|lint|build|dev`

Agent command classification:

- Safe auto-run: `git status|diff|log`; non-secret `cat <safe-relative-file>` and `sed -n '<start>,<end>p' <safe-relative-file>`; `cat`/`sed` reads under `~/.patchpilot/skills` and `~/.agents/skills`; `npm run test|lint|build|dev`; `pnpm test|lint|build|dev`; `pnpm --dir <safe-relative-dir> test|lint|build|dev`; `yarn test|lint|build|dev`; `yarn --dir <safe-relative-dir> test|lint|build|dev`; `bun test`; `bun run lint|build|dev`; `go test ./...`; `go test ./<package>`; `go build ./...`; `pytest`; `python -m pytest`; `python3 -m pytest`; `cargo test|build`; `make test|lint|build|dev`.
- Risky: syntactically valid but outside allowlist, including secret-like file reads and non-skill outside-workspace file reads; returns `409 confirmation_required` unless `confirmed:true`.
- Blocked `400 blocked_command`: shell control/expansion (`&&`, `||`, `;`, `|`, `>`, `<`, backticks, `$(`, newlines), workspace escapes, globs, broad directory reads, unsupported `cat`/`sed` flags, `sudo`, `su`, forced recursive `rm`, `git clean`, `git reset --hard`, `chmod -R`, `chown -R`.

Agent command execution always parses arguments without a shell, runs at workspace root, and rejects traversal/shell operators. `confirmation_required` and `blocked_command` include `details.decision` with `level`, `reason`, and parsed `parts`.

## Data Model

SQLite stores app state; source files stay in original repos; Git owns repo history. PatchPilot-owned state may live under `~/.patchpilot`.

Runtime config uses OS env, falling back to local `.env`: `PATCHPILOT_ADDR`, `PATCHPILOT_ALLOWED_ROOTS`, `PATCHPILOT_STATIC_DIR`, `PATCHPILOT_LOG_FORMAT`, `PATCHPILOT_ADMIN_TOKEN`, `PATCHPILOT_OPENAI_API_KEY`, `PATCHPILOT_OPENAI_BASE_URL`, `PATCHPILOT_LIGHT_MODEL`. PatchPilot-owned state always lives under `~/.patchpilot`; SQLite is always `~/.patchpilot/patchpilot.db`.

Global agent runtime config lives at `~/.patchpilot/config.json`, loaded at startup and explicit agent-context refresh. Missing `enabled` fields default `true`.

```json
{
  "skills": {
    "coding": { "enabled": true },
    "review": { "enabled": false }
  },
  "mcpServers": {
    "filesystem": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-filesystem", "."],
      "enabled": true
    },
    "github": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-github"],
      "env": { "GITHUB_TOKEN": "${GITHUB_TOKEN}" },
      "enabled": false
    }
  }
}
```

`skills` controls enablement by discovered key. `mcpServers` is authoritative. `${ENV_NAME}` placeholders resolve from server OS env at runtime; unresolved placeholders mark the server with safe warning and do not expose secret values.

Migrations are explicit, versioned, manually authored, tracked in SQLite metadata, run before API traffic, and fail startup on error. GORM `AutoMigrate` is not used for product schema changes.

Active tables:

- `app_metadata`: migration/version metadata and app key-value state.
- `auth_sessions`: `id`, `session_hash`, `created_at`, `last_seen_at`, `expires_at`.
- `workspaces`: `id`, `name`, `root_path`, `git_remote?`, `default_branch?`, `status(indexing|ready|error)`, timestamps.
- `file_index`: `workspace_id`, `path`, `size`, `modified_at`, `indexed_at`.
- `conversations`: `id`, `workspace_id`, `title`, timestamps, `last_message_at`, `context_summary`, `context_summary_through_message_id?`, `context_summary_updated_at?`.
- `messages`: `id`, `workspace_id`, `conversation_id`, `run_id?`, `role`, `content`, `created_at`.
- `agent_runs`: `id`, `workspace_id`, `conversation_id`, `trigger_message_id`, `model`, `reasoning_effort`, `status`, `summary?`, `error?`, timestamps.
- `agent_run_events`: `id`, `workspace_id`, `run_id`, `type`, `payload_json`, `created_at`.
- `agent_tool_calls`: `id`, `workspace_id`, `run_id`, `batch_id`, `sequence`, `name`, `source`, `source_ref?`, `input_json`, `output_json`, `status`, `requires_approval`, `decision?`, timestamps.
- Optional skill/MCP cache tables may store metadata, health, and discovery results for efficiency; `~/.patchpilot/config.json` plus filesystem skill discovery remain source of truth.
- `terminal_sessions`: `id`, `workspace_id`, `title`, `cwd`, `status`, `pid?`, `rows`, `cols`, `exit_code?`, timestamps.
- PatchPilot-owned user config (`~/.patchpilot/config.json`): local skill enablement, MCP server config, settings preferences, and installed font metadata. Installed font binaries live under `~/.patchpilot/fonts`; source files are not copied into repositories.
- Legacy `commands` and `command_output` tables may remain on upgraded installs for old workspace command history, but v0.3 Workspace APIs no longer create or expose them.
- `ports`: `id`, `workspace_id`, `process_id?`, `port`, `status(detected|exposed|closed)`, `exposed_path?`, timestamps.
- `git_snapshots`: `id`, `workspace_id`, `commit_sha?`, `status_json`, `created_at`.

Statuses:

```txt
agent:   queued -> running -> waiting_tool_approval -> running -> done
agent:   queued -> running -> done|failed|canceled
terminal: open -> closed|failed
```

## Frontend Structure

Route entry files stay thin. `web/src/features/vibe` uses `hooks` for orchestration, `layout` for shell regions, `components` for Vibe-only UI, and `lib` for pure helpers. Vibe owns context, instructions, skills, MCP, approvals, and run details. Workspace Mode stays a compact support console for files, Git, terminal sessions, and preview. Settings UI lives under `web/src/features/settings`. Appearance preferences are applied through app-level providers and CSS variables: app font maps to the global sans role, code font maps to code/diff/file monospace surfaces, and terminal font is read by xterm session creation.

## Acceptance

- Open local Git repo workspace.
- Create, list, open, rename, and continue conversations per workspace.
- Send chat messages that start agent runs.
- Workspace refresh reads applicable `AGENTS.md` files and shows effective sources, precedence, and warnings in Vibe.
- Agent-context refresh reloads `~/.patchpilot/config.json`, scans `~/.patchpilot/skills` and `~/.agents/skills`, and shows safe warnings for invalid config.
- Users can open Skills from the Vibe sidebar, see skill name/description/body detail from YAML frontmatter, and enable/disable discovered local skills through config without remote installs; missing `config.skills` entries default enabled.
- Duplicate skill keys select only the `~/.patchpilot/skills` copy for effective list/context.
- Enabled skills influence future runs through name, description, and `~` path metadata in the prompt; disabled/invalid skills are not injected.
- Users can inspect stdio/HTTP MCP servers from `config.mcpServers`.
- MCP discovery shows server, transport, tool/resource metadata, health, disabled state, last error, read-only hints, and effective approval policy.
- Agent starts non-trivial work with a short plan, reads/searches approved files before changes, returns messages/tool calls rather than direct mutations, produces small reviewable patches, reports changed files, and runs/recommends narrow verification.
- Users approve/reject approval-required tools; server executes only approved mutating tools.
- MCP tools execute only through the backend bridge and share durable tool-call/event/approval flow with built-ins.
- Users create, switch, resize, rename, and close Workspace terminal sessions from the persistent bottom terminal panel, view Git status/diff, commit explicit non-empty selected paths, and preview through same-host proxy.
- Users open Settings from the shared top bar, update theme and app/code/terminal font preferences, enter custom OS-resolved font-family fallback stacks, install local font files, and see live font previews without network font loading. Verification: settings API tests, frontend settings tests, and browser smoke at mobile/desktop widths.
- Mobile/iPad users complete a Vibe Mode chat-driven AI coding loop and inspect the agent cockpit through tabs/sheets without losing primary flow.
- Auth/session expiry: expired/missing/invalid cookies return `401 unauthorized`; valid logout clears cookie. Verification: backend auth/API handler tests.
- Indexing failure: workspace create/get/index refresh return standard error envelope without host-path leakage and do not send stale successful index responses. Verification: backend API handler tests.
- SSE replay: run streams replay durable events after `Last-Event-ID`, exclude transient `agent.delta`, and emit in-memory `agent.output.snapshot` only for active local runs. Verification: backend SSE handler tests.
- Terminal replay: active sessions keep only the latest 1 MiB in memory; transcripts are not persisted and closed sessions do not replay output. Verification: terminal service and WebSocket API tests.
- Closed ports: unreachable exposed/detected ports become `closed`, emit `port.closed`, and expose/proxy returns `502 port_unreachable`. Verification: backend port/API handler tests.
- Patch conflict: failed apply marks tool call failed, leaves files unchanged, records actionable error, and keeps run recoverable without executing later approval-required tools in that batch. Verification: backend agent/tool approval tests.
- Invalid paths: file, Git, terminal, and agent tool paths reject traversal and symlink escapes with standard errors and no host-path leakage except explicit approved outside-workspace read commands. Verification: filestore, gitrepo, terminal, runner, agent, API tests.
- Secret protection: agent reads of secret-like paths require approval; manual writes reject `.env`, `.env.*`, `*.pem`, `*.key`, `id_rsa`, `id_ed25519`, `.npmrc`, `.pypirc`, `.netrc`. Verification: runner, agent, filestore, API tests.
- Instruction context safety: `AGENTS.md` discovery rejects escapes, external symlinks, secret-like paths, binaries, oversized files, and shows safe warnings. Verification: agent context and API handler tests.
- Skill manager safety: parser validates YAML-frontmatter `SKILL.md`, preserves invalid-skill warnings, respects config enablement, applies `~/.patchpilot/skills` duplicate precedence, rejects duplicate enabled names after precedence, and injects skill bodies only through `cat`/`sed` command output. Verification: skill repository/parser and agent context tests.
- MCP safety: stdio/HTTP fake servers can be added, refreshed, listed, and called from config; disabled servers do not start; unresolved env placeholders produce safe warnings; unknown/mutating tools require approval; read-only auto-run requires both metadata and PatchPilot policy. Verification: MCP client, approval-policy, API handler, agent manager tests.
- Agent cockpit UI: context, skills, MCP, approvals, and run details are visible on desktop/mobile; long paths/tool names/server names/JSON summaries wrap or truncate without layout shifts. Verification: frontend component tests and Playwright smoke.
