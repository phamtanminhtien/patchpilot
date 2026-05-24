# PatchPilot Project Rules

Locked engineering contract. Precedence: this file > `docs/product-spec.md` > `docs/concept.md` > implementation.

## Change Control

- Rule changes need explicit human approval before implementation.
- Same change must update this file, affected specs, and task/PR summary.
- If a task conflicts with this file, stop and ask.

## Product

- Self-hosted, single-user, mobile-first AI coding workspace.
- Active product direction = Vibe Mode conversations + agent run tool-loop AI coding flow.
- Workspace Mode only supports files, search, diffs, small edits, command output, preview, Git status.
- Do not build VS Code parity.
- Agents return assistant text and tool calls; tools that mutate files or run risky commands require explicit user approval.
- Server executes approved mutating tools only after all approval-required tools in the current batch have a user decision.

## Locked Stack

Backend:

- Go 1.26.x, `net/http`, `http.ServeMux`, REST mutations, SSE realtime.
- SQLite via GORM 1.x with `github.com/glebarez/sqlite` (pure-Go `modernc.org/sqlite` driver); schema changes require explicit, manually authored migrations.
- Logging via `go.uber.org/zap`; default console logs colorize level, time, and caller for local dev, and `PATCHPILOT_LOG_FORMAT=json` enables JSON logs.
- Git only via `internal/gitrepo`; process execution only via `internal/runner`.
- One Go binary serves API and embedded frontend.
- No Go web framework, GraphQL, gRPC, WebSocket for the active product direction, ORM other than GORM, or CGO-only default dependency.

Frontend:

- Node.js 24.x LTS, pnpm 10.x, TypeScript 5.x, React 19.x, Vite 8.x.
- React Router 7.x, TanStack Query 5.x, Axios 1.x, nuqs 2.x, Zustand 5.x, Radix UI primitives, CodeMirror 6, `lucide-react`, `react-markdown`, `react-syntax-highlighter`, `remark-gfm`.
- Tailwind CSS 4.x via `@tailwindcss/vite`; Vitest, React Testing Library, Playwright.
- No competing app framework, state store other than Zustand, non-Radix UI framework, CSS Modules, xterm.js, WebSocket, direct `fetch` for frontend API calls, or generated SVG primary UI.

Runtime:

- Runs as host OS user on local/private/self-hosted VPS.
- Default app space is `~/.patchpilot`; app-owned state may live there.
- Local `.env` is supported for PatchPilot config; OS environment variables override `.env` values.
- Logical isolation under configured workspace roots.
- Same-host preview proxy only.
- Docker isolation, public tunnels, and multi-user cloud are outside the active product direction.

## Structure

```txt
cmd/patchpilot/        startup and wiring only
internal/api/          HTTP handlers, middleware, routes, request/response encoding
internal/auth/         admin-token login, session cookies, validation
internal/database/     SQLite setup, manual migrations, transactions
internal/events/       SSE fan-out and replay
internal/workspace/    workspace lifecycle and path validation
internal/filestore/    listing, reads, search, small manual writes
internal/agent/        conversation agent runs and tool execution
internal/gitrepo/      only package that may execute Git
internal/runner/       only package that may execute workspace processes
internal/ports/        port detection and same-host proxy
web/src/app/           app shell, providers, cross-route context
web/src/features/vibe/ Vibe Mode screens and flows
web/src/features/workspace/ lightweight workspace support screens
web/src/shared/        shared api, events, ui, styles, url, utils
```

- Frontend imports may use the `@/*` alias for `web/src/*`.
- Feature modules must not import each other directly; shared code belongs in `web/src/shared`.
- Domain packages must not import `internal/api` or write HTTP responses.
- Handlers call service functions and contain no business logic.
- Long-running state lives in managers, not handler locals.
- Normalize and validate all filesystem paths.

## API

- Routes use `/api`; workspace resources use `/api/workspaces/:workspaceId`.
- JSON bodies except SSE and preview proxy traffic.
- Reads `GET`; mutations `POST`/`PUT`/`DELETE`.
- Lists newest-first unless naturally tree-ordered; if >100 records, support `limit`/`cursor`, max `limit=100`.
- REST error: `{ "error": { "code": "snake_case", "message": "...", "details": {} } }`.
- UI errors must not expose stack traces, secrets, raw env, or host paths outside workspace root.
- ID prefixes: `ws_`, `auth_`, `sess_`, `conv_`, `msg_`, `run_`, `evt_`, `cmd_`, `port_`.
- API timestamps: UTC RFC3339. SQLite timestamps: UTC.

Frontend API:

- Axios instance: `web/src/shared/api/client.ts`, `baseURL: "/api"`, `withCredentials: true`.
- Features call typed APIs from `web/src/shared/api`; no direct Axios outside the shared API layer.
- DTOs/errors live under `web/src/shared/api`; preserve backend error fields.
- Do not use raw `fetch` for PatchPilot APIs; use the shared Axios client and typed API functions.

SSE:

- Endpoint: `GET /api/workspaces/:workspaceId/events`.
- Server-to-client only; no mutations over SSE.
- Every event has ID and `docs/product-spec.md` envelope.
- Replay `Last-Event-ID` for stored conversation/run events and latest 1 MiB command output.
- Non-replayable transient events need durable follow-up state events.

## Data

- SQLite is the only app DB in the active product direction; source files stay on disk; Git is repo history source.
- App-owned runtime/state files may live under `~/.patchpilot`; workspace source files must not be copied there.
- Manual migrations run before API traffic and enable foreign keys. Automatic GORM `AutoMigrate` is not used for product schema changes.
- Multi-table writes use transactions.
- JSON columns only for event payloads, snapshots, unindexed metadata; query-critical fields are columns.
- No plaintext secrets.
- Agent prompts, events, command lines/output are user data.

## Security

- Single-user auth: admin token -> `POST /api/auth/login` -> HTTP-only session cookie.
- All other APIs require valid cookie.
- Cookies: `HttpOnly`, `SameSite=Lax`, `Secure` over HTTPS; session tokens hashed in SQLite.
- Admin token never goes to logs, conversation/run events, or agent context.
- Workspace roots are absolute and inside configured allowed roots.
- File API paths are workspace-relative; reject traversal and symlinks outside root.
- Do not expose arbitrary host paths.
- Agent `read_file` blocks secrets by default: `.env`, `.env.*`, `*.pem`, `*.key`, `id_rsa`, `id_ed25519`, `.npmrc`, `.pypirc`, `.netrc`.
- Users may manually open files; secret contents must not enter agent context.
- User commands run only after direct submission.
- Agent commands auto-run only if allowed by `docs/product-spec.md` and no shell control operators.
- Patch tools always require approval. Non-allowlisted agent commands require approval.
- Commands run at workspace root, without elevation, output capped to latest 1 MiB.

## Frontend

- Default route: mobile/iPad Vibe Mode, desktop Workspace Mode; users can switch modes.
- Vibe shows summary before detail; diff review must work on mobile.
- Primary actions >=44px; tool buttons use `lucide-react`.
- Web app shell locks to full viewport width and height; scroll belongs to explicit inner regions that need it, not the document body.
- Text must not overflow; fixed-format UI has stable dimensions.
- No default landing page, nested cards, gradient blobs/orbs/bokeh, one-hue dominant palette, or marketing copy in workflow screens.
- Tailwind is the only component styling system; global CSS uses `@import "tailwindcss";`.
- Design tokens for background, color, spacing, radius, shadow, and focus live in global CSS and Tailwind theme variables.
- Components must use CSS variables through Tailwind theme tokens; no hardcoded hex/rgb/hsl values, raw palette utilities, or ad hoc spacing/radius values in components.
- Shared UI primitives in `web/src/shared/ui`; components use Tailwind utilities and may wrap Radix primitives.
- Repeated classes become shared primitives.
- No feature global CSS, CSS Modules, Tailwind CDN, or `@apply` for component styling, except shared Markdown renderer styles under `web/src/shared/styles/global.css`.
- Inline styles/arbitrary Tailwind values only for dynamic values, locked tokens, measured constraints, or third-party needs.
- Third-party CSS overrides live in `web/src/shared/styles`.

## State And Git

- Server state: TanStack Query. Local UI: React state or Zustand for shared client-only UI state. Cross-route context: `web/src/app`. URL state: nuqs.
- Install React Router 7 nuqs adapter at route root from `nuqs/adapters/react-router/v7`.
- Deep-linkable workspace/mode/conversation/run/file/tool/port/tab selections live in URL state.
- Shared query parsers live in `web/src/shared/url`; features do not define ad hoc parsers.
- Do not duplicate server state into Zustand or keep command output in React state/Zustand beyond visible buffer.
- Current Git scope: status, diff, stage, unstage, discard, and commit. This
  broader Git scope is explicitly approved for v0.2.
- Commits require non-empty selected paths, do not push, do not auto-stage unrelated files, show untracked files, and use exact user message.
- Push/pull/branch/merge/rebase are outside the active product direction.
- Local Git hooks live in `.githooks`; `make setup` configures `core.hooksPath` and executable bits. Pre-commit calls `make pre-commit`, which runs Go formatting/tests and frontend `lint-staged` checks before allowing a commit.

## Testing

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

## Docs And Dependencies

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

## Agent Rules

1. Read `AGENTS.md`, this file, `docs/product-spec.md`, and related files.
2. Identify relevant rule/spec before editing.
3. Update docs first for behavior/API/data/stack/scope changes.
4. Make the smallest complete change.
5. Run narrowest verification.
6. Report files and results.

Agents must not add disallowed deps, broaden active product scope, implement out-of-scope features, leave scratch/generated files, run destructive Git commands, or revert user changes unless instructed.

## Current Out-Of-Scope Areas

Full IDE, terminal emulator, WebSocket, plugins, Docker-required runtime, public tunnels, multi-user/team/RBAC, hosted SaaS, billing, push/pull/branch/merge/rebase, LSP, inline diagnostics, multi-tab editor, marketplace integrations.
