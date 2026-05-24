# PatchPilot Project Rules

Locked engineering contract. Precedence: this file > `docs/product-spec.md` > `docs/concept.md` > implementation.

## Change Control

- Rule changes need explicit human approval before implementation.
- The same change must update this file, affected specs, and task/PR summary.
- If a task conflicts with this file, stop and ask.

## Product Scope

- Self-hosted, single-user, mobile-first AI coding workspace.
- Active direction: Vibe Mode conversations + agent-run tool loop with managed agent context/runtime.
- Workspace Mode only supports files, search, diffs, small edits, command output, preview, and Git status.
- Do not build VS Code parity.
- Agents return assistant text and tool calls; mutating tools and risky commands require explicit user approval.
- Server executes approved mutating tools only after every approval-required tool in the current batch has a decision.
- Agent context may include repo `AGENTS.md`, enabled local skills, MCP registry metadata, conversation context, and durable tool history.

## Locked Stack

Backend:

- Go 1.26.x, `net/http`, `http.ServeMux`, REST mutations, SSE realtime.
- SQLite via GORM 1.x with `github.com/glebarez/sqlite` / pure-Go `modernc.org/sqlite`; product schema changes use explicit manual migrations, never GORM `AutoMigrate`.
- Logging via `go.uber.org/zap`; default local console logs colorize level/time/caller; `PATCHPILOT_LOG_FORMAT=json` enables JSON.
- Git only through `internal/gitrepo`; process execution only through `internal/runner`.
- MCP execution only through backend-managed stdio or explicitly configured HTTP clients; agents/frontend never call MCP servers directly.
- One Go binary serves API and embedded frontend.
- No Go web framework, GraphQL, gRPC, WebSocket, non-GORM ORM, or CGO-only default dependency for the active product.

Frontend:

- Node.js 24.x LTS, pnpm 10.x, TypeScript 5.x, React 19.x, Vite 8.x.
- React Router 7.x, TanStack Query 5.x, Axios 1.x, nuqs 2.x, Zustand 5.x, Radix UI primitives, CodeMirror 6, `lucide-react`, `react-markdown`, `react-syntax-highlighter`, `remark-gfm`.
- Tailwind CSS 4.x via `@tailwindcss/vite`; Vitest, React Testing Library, Playwright.
- No competing app framework, state store other than Zustand, non-Radix UI framework, CSS Modules, xterm.js, WebSocket, direct frontend API `fetch`, or generated SVG primary UI.

Runtime:

- Runs as host OS user on local/private/self-hosted VPS.
- Default app state directory: `~/.patchpilot`.
- Local `.env` is supported; OS env overrides `.env`.
- Logical isolation under configured workspace roots; same-host preview proxy only.
- Docker isolation, public tunnels, and multi-user cloud are out of scope.

## Structure

```txt
cmd/patchpilot/        startup/wiring only
internal/api/          HTTP handlers, middleware, routes, encoding, SSE, proxy
internal/auth/         admin-token login, session cookies, validation
internal/database/     SQLite setup, manual migrations, transactions
internal/events/       SSE fan-out and replay
internal/workspace/    workspace lifecycle and path validation
internal/filestore/    listing, reads, search, small manual writes
internal/agent/        conversation agent runs and tool execution
internal/skills/       local skill registry, indexing, context selection
internal/mcp/          MCP registry, discovery, backend tool bridge
internal/gitrepo/      only package that may execute Git
internal/runner/       only package that may execute workspace processes
internal/ports/        port detection and same-host proxy
web/src/app/           app shell, providers, cross-route context
web/src/features/vibe/ Vibe Mode screens and flows
web/src/features/workspace/ lightweight workspace support screens
web/src/shared/        shared api, events, ui, styles, url, utils
```

- Frontend imports may use `@/*` for `web/src/*`.
- Feature modules must not import each other directly; shared code belongs in `web/src/shared`.
- Domain packages must not import `internal/api` or write HTTP responses.
- Handlers call services and contain no business logic.
- Long-running state lives in managers, not handler locals.
- Normalize and validate all filesystem paths.

## API And SSE

- Routes use `/api`; workspace resources use `/api/workspaces/:workspaceId`.
- JSON bodies except SSE and preview proxy traffic.
- Reads use `GET`; mutations use `POST`/`PUT`/`DELETE`.
- Lists are newest-first unless naturally tree-ordered; lists over 100 records support `limit`/`cursor`, max `limit=100`.
- REST error shape: `{ "error": { "code": "snake_case", "message": "...", "details": {} } }`.
- UI errors must not expose stack traces, secrets, raw env, or host paths outside workspace root.
- ID prefixes: `ws_`, `auth_`, `sess_`, `conv_`, `msg_`, `run_`, `evt_`, `cmd_`, `port_`, `skill_`, `mcp_`.
- API and SQLite timestamps are UTC; API uses RFC3339.
- Frontend APIs use the shared Axios client at `web/src/shared/api/client.ts` with `baseURL: "/api"` and `withCredentials: true`.
- Features call typed APIs from `web/src/shared/api`; DTOs/errors live there and preserve backend error fields.
- Do not use raw `fetch` for PatchPilot APIs.
- SSE endpoint: `GET /api/workspaces/:workspaceId/events`; server-to-client only, no mutations.
- Every SSE event has ID and the `docs/product-spec.md` envelope.
- SSE replays `Last-Event-ID` for stored conversation/run events and latest 1 MiB command output.
- Non-replayable transient events need durable follow-up state events.

## Data And Security

- SQLite is the only app DB; source files stay on disk; Git is repo history source.
- App-owned runtime/state may live under `~/.patchpilot`; workspace source files must not be copied there.
- Manual migrations run before API traffic and enable foreign keys; multi-table writes use transactions.
- JSON columns only for event payloads, snapshots, and unindexed metadata; query-critical fields are columns.
- No plaintext secrets. Agent prompts, events, command lines/output are user data.
- Local skill and MCP runtime config lives in `~/.patchpilot/config.json`; Skills/MCP list APIs derive from config plus filesystem skill discovery, not DB registry rows.
- Auth: admin token -> `POST /api/auth/login` -> HTTP-only session cookie; all other APIs require a valid cookie.
- Cookies: `HttpOnly`, `SameSite=Lax`, `Secure` over HTTPS; session tokens are hashed in SQLite.
- Admin token never goes to logs, conversation/run events, or agent context.
- Workspace roots are absolute and inside configured allowed roots; file API paths are workspace-relative and reject traversal/symlink escapes.
- Do not expose arbitrary host paths.
- Agent `read_file` blocks secrets by default: `.env`, `.env.*`, `*.pem`, `*.key`, `id_rsa`, `id_ed25519`, `.npmrc`, `.pypirc`, `.netrc`.
- Users may manually open files, but secret contents must not enter agent context.
- Repo instruction files and skill files enter agent context only after workspace-root, symlink, size, binary, and secret-path checks.
- Skill discovery reads `~/.patchpilot/skills` before `~/.agent/skills`; duplicate keys use only the `~/.patchpilot/skills` copy for effective skills and agent context.
- User commands run only after direct submission.
- Agent commands auto-run only if allowed by `docs/product-spec.md` and free of shell control operators.
- Patch tools always require approval. Non-allowlisted agent commands require approval.
- Commands run at workspace root, without elevation, with output capped to latest 1 MiB.
- MCP HTTP servers require explicit config; no public discovery, marketplace sync, or automatic network scanning.
- MCP env placeholder values must not be persisted, logged, emitted in events, or sent to agent context.
- Unknown or mutating MCP tools require approval; read-only MCP tools may auto-run only when server/tool metadata and PatchPilot policy both mark them safe.

## Frontend

- Default route: mobile/iPad Vibe Mode, desktop Workspace Mode; users can switch modes.
- Vibe shows summary before detail; diff review must work on mobile.
- Vibe owns the agent cockpit for effective instructions, enabled skills, MCP servers/tools, approvals, and run details.
- Primary actions are at least 44px; tool buttons use `lucide-react`.
- App shell locks to full viewport width/height; scroll belongs to explicit inner regions, not document body.
- Text must not overflow; fixed-format UI has stable dimensions.
- No default landing page, nested cards, gradient blobs/orbs/bokeh, one-hue dominant palette, or marketing copy in workflow screens.
- Tailwind is the only component styling system; global CSS uses `@import "tailwindcss";`.
- Background/color/spacing/radius/shadow/focus tokens live in global CSS and Tailwind theme variables.
- Components use CSS variables through Tailwind theme tokens; no hardcoded hex/rgb/hsl, raw palette utilities, or ad hoc spacing/radius values.
- Shared UI primitives live in `web/src/shared/ui`; components use Tailwind utilities and may wrap Radix primitives.
- Repeated classes become shared primitives.
- No feature global CSS, CSS Modules, Tailwind CDN, or component `@apply`, except shared Markdown renderer styles under `web/src/shared/styles/global.css`.
- Inline styles/arbitrary Tailwind values only for dynamic values, locked tokens, measured constraints, or third-party needs.
- Third-party CSS overrides live in `web/src/shared/styles`.

## State, Git, Testing

- Server state: TanStack Query. Local UI: React state or Zustand for shared client-only UI state. Cross-route context: `web/src/app`. URL state: nuqs.
- Install React Router 7 nuqs adapter at route root from `nuqs/adapters/react-router/v7`.
- Deep-linkable workspace/mode/conversation/run/file/tool/port/tab selections live in URL state.
- Shared query parsers live in `web/src/shared/url`; features do not define ad hoc parsers.
- Do not duplicate server state into Zustand or keep command output in React state/Zustand beyond visible buffer.
- Current approved Git scope: status, diff, stage, unstage, discard, commit.
- Commits require non-empty selected paths, never push or auto-stage unrelated files, show untracked files, and use the exact user message.
- Push/pull/branch/merge/rebase are outside active scope.
- Local hooks live in `.githooks`; `make setup` configures `core.hooksPath` and executable bits. Pre-commit calls `make pre-commit` for Go formatting/tests and frontend `lint-staged`.

Coverage when area exists:

- Go unit: domain logic.
- Go integration: manual migrations, repositories, Git adapter, runner, API handlers.
- Frontend unit: pure utilities/reducers.
- Frontend component: Vibe lifecycle, tool approval, command output, Git status.
- Playwright: critical mobile AI loop once frontend shell exists.

Canonical commands:

```txt
go test ./...
go run ./cmd/patchpilot
pnpm --dir web install
pnpm --dir web dev
pnpm --dir web build
pnpm --dir web test
pnpm --dir web lint
pnpm --dir web exec playwright test
```

Verify by change: backend `go test ./...`; frontend `pnpm --dir web test` + `pnpm --dir web build`; UI browser/Playwright; API handler tests; data schema/repository tests; runner stdout/stderr/exit/cancel/truncation tests; port proxy route/workspace tests. If scaffolding is absent, say so.

## Docs, Dependencies, Agent Rules

- Behavior -> `docs/product-spec.md`.
- Rules/stack/structure/workflow -> this file.
- Product direction -> `docs/concept.md`.
- API/data changes update spec before implementation.
- Product docs use concrete decisions; no unresolved current-scope questions or out-of-scope acceptance criteria.
- New runtime dependencies need approval and must reduce real risk/complexity.
- Project-standard dependency changes update locked stack.
- Backend deps preserve single-binary build.
- Frontend deps must not add competing framework/UI/router/store/styling/build system.
- Test-only deps are dev/test deps.

Agents must:

1. Read `AGENTS.md`, this file, `docs/product-spec.md`, and related files.
2. Identify relevant rule/spec before editing.
3. Update docs first for behavior/API/data/stack/scope changes.
4. Make the smallest complete change.
5. Run narrowest verification.
6. Report files and results.

Agents must not add disallowed deps, broaden active product scope, implement out-of-scope features, leave scratch/generated files, run destructive Git commands, or revert user changes unless instructed.

## Out Of Scope

Full IDE, terminal emulator, WebSocket, unbounded plugin execution, Docker-required runtime, public tunnels, multi-user/team/RBAC, hosted SaaS, billing, push/pull/branch/merge/rebase, LSP, inline diagnostics, multi-tab editor, marketplace integrations, skill marketplace/install from remote sources, MCP public discovery.
