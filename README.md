# PatchPilot 🚀

PatchPilot is a self-hosted, single-user coding assistant for running chat-driven AI coding loops against local Git repositories. It lets you open an allowed workspace, continue multiple conversations, review approval-required tool calls, run project commands, inspect Git status, and commit selected paths from a mobile-friendly web UI.

## v0.3 Product Baseline ✅

- Open and index local Git workspaces under configured allowed roots.
- Use Vibe Mode to create and continue multiple workspace conversations.
- Send chat messages that trigger agent runs with model and reasoning-effort choices.
- Stream conversation, agent run, tool, command, and workspace activity through SSE.
- Inspect effective repo instructions, enabled local skills, MCP server/tool
  metadata, context warnings, approvals, and run details in the Vibe cockpit.
- Use local skill discovery and `use_skill` context retrieval without remote
  marketplaces.
- Configure MCP servers locally and execute MCP tools only through the backend
  bridge and approval policy.
- Approve or reject mutating agent tools before they touch the workspace.
- Browse files, read small text files, search workspace contents, and inspect diffs.
- Run classified workspace commands without a shell and replay the latest command output.
- Stage, unstage, discard, and commit explicit selected paths.
- Serve the frontend through Vite in development or through the Go server in a built deployment.

The v0.3 focus is managed agent context/runtime: repo instructions, local
skills, MCP metadata, approval-gated tools, visible agent plans, reviewable
patches, and narrow verification after changes.

## Tech Stack 🧱

- Backend: Go, `net/http`, SQLite through GORM, Zap logging.
- Frontend: React, React Router, Vite, TanStack Query, Zustand, nuqs, Tailwind CSS, Radix primitives, CodeMirror, lucide-react.
- Realtime: Server-Sent Events.
- Package manager: pnpm for `web/`.

## Requirements 🛠️

- Go 1.26 or newer.
- Node.js 24 or newer.
- pnpm 10.
- Git.
- `air` if you want to use `make dev` or `make dev-api`; otherwise use `go run ./cmd/patchpilot`.

## Configuration ⚙️

PatchPilot reads OS environment variables first, then a local `.env` file in the process working directory.

Create a local `.env` file:

```dotenv
PATCHPILOT_ADDR=127.0.0.1:8080
PATCHPILOT_ALLOWED_ROOTS=/absolute/path/to/repos
PATCHPILOT_ADMIN_TOKEN=choose-a-local-admin-token
PATCHPILOT_OPENAI_API_KEY=your-openai-or-compatible-key
# PATCHPILOT_OPENAI_BASE_URL=https://api.openai.com/v1
# PATCHPILOT_STATIC_DIR=/absolute/path/to/web/dist
# PATCHPILOT_LOG_FORMAT=json
```

Important variables:

- `PATCHPILOT_ALLOWED_ROOTS`: OS path-list of directories that may be opened as workspaces.
- `PATCHPILOT_ADMIN_TOKEN`: required single-user login secret.
- `PATCHPILOT_OPENAI_API_KEY`: backend-only provider secret used by agent runs.
- `PATCHPILOT_OPENAI_BASE_URL`: optional OpenAI-compatible base URL. Defaults to `https://api.openai.com/v1`.
- `PATCHPILOT_ADDR`: backend listen address. Defaults to `127.0.0.1:8080`.
- `PATCHPILOT_STATIC_DIR`: built frontend directory served by the Go server. Defaults to `web/dist`.

PatchPilot-owned state always lives under `~/.patchpilot`; SQLite is always `~/.patchpilot/patchpilot.db`.

Do not commit `.env` files or provider keys.

## Development 💻

Install frontend dependencies:

```sh
pnpm --dir web install
```

Run the backend:

```sh
go run ./cmd/patchpilot
```

In another terminal, run the frontend:

```sh
pnpm --dir web dev
```

Open the Vite app at:

```txt
http://127.0.0.1:5173
```

The Vite dev server proxies `/api` and `/workspaces` to `http://127.0.0.1:8080`.

If `air` is installed, you can run both backend and frontend through:

```sh
make dev
```

## Production Build 📦

Build backend and frontend:

```sh
make build
```

Serve the built frontend from the Go server:

```sh
go run ./cmd/patchpilot
```

Then open:

```txt
http://127.0.0.1:8080
```

## Docker 🐳

The repository includes a Dockerfile and Compose service that build the frontend, build the Go binary, store PatchPilot data in a named volume, and mount this repository under `/workspace/patchpilot`.

```sh
docker compose up --build
```

If `docker compose pull` reports that no `linux/arm64` manifest exists, build
the service locally instead:

```sh
docker compose build patchpilot
docker compose up patchpilot
```

The container listens on:

```txt
http://127.0.0.1:8080
```

Set `PATCHPILOT_OPENAI_API_KEY` in the Compose service environment or an override file before using agent runs.

Released Docker images are published to GitHub Container Registry:

```sh
docker run --rm \
  -p 8080:8080 \
  -e PATCHPILOT_ADMIN_TOKEN=choose-a-local-admin-token \
  -e PATCHPILOT_OPENAI_API_KEY=your-openai-or-compatible-key \
  -v patchpilot-data:/root/.patchpilot \
  -v /absolute/path/to/repos:/workspace \
  ghcr.io/phamtanminhtien/patchpilot:latest
```

Use a version tag such as `0.2.0` instead of `latest` for reproducible runs.

## Releases 🚢

PatchPilot uses Release Please to create release pull requests from Conventional
Commits. Merging a Release Please PR creates the GitHub Release and tag, then
publishes multi-architecture Docker image tags to GHCR:

```txt
ghcr.io/phamtanminhtien/patchpilot:patchpilot-v<version>
ghcr.io/phamtanminhtien/patchpilot:<version>
ghcr.io/phamtanminhtien/patchpilot:latest
```

See [docs/release.md](docs/release.md) for the release checklist.

## Common Commands ⌨️

```sh
make setup          # configure local Git hooks
make test           # run Go and frontend tests
make test-go        # run Go tests
make test-web       # run Vitest
make lint           # run frontend lint
make format         # format Go and frontend files
make format-check   # check frontend formatting
make build          # build backend and frontend
```

## Project Layout 🗂️

```txt
cmd/patchpilot       Go application entrypoint
internal/api         HTTP routes, handlers, SSE, and static serving
internal/agent       Conversation agent run orchestration and OpenAI-compatible provider
internal/config      Runtime configuration
internal/database    SQLite connection and manual migrations
internal/events      SSE event fan-out
internal/filestore   Safe workspace file access
internal/gitrepo     Git status, diff, staging, discard, and commit helpers
internal/runner      Workspace command classification and execution
internal/workspace   Workspace validation, metadata, and file indexing
web/src/app          Frontend shell, routing, theme, and mode defaults
web/src/features     Vibe and Workspace feature UI
web/src/shared       API client, UI primitives, URL helpers, and styles
docs                 Product rules, product spec, architecture, and design language
```

## Safety Model 🔒

PatchPilot keeps workspace source files in their original repositories. App metadata lives in SQLite. Agent changes run through the tool loop: the agent can inspect approved workspace context and request tools, but mutating tools run only after user approval.

Commands run from the workspace root without a shell. Shell control operators, workspace escape attempts, and destructive patterns are blocked or require confirmation according to the command classifier.

## Documentation 📚

- `docs/project-rules.md`: locked implementation rules.
- `docs/product-spec.md`: v0.3 scope, flows, API, and data contracts.
- `docs/app-architecture.md`: architecture overview.
- `docs/design-language.md`: frontend design system and UI rules.
- `docs/release.md`: release checklist and post-release verification.

## License

PatchPilot is licensed under the [Apache License 2.0](LICENSE).
