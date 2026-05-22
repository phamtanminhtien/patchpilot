# PatchPilot Architecture

This document summarizes the current architecture. `docs/project-rules.md` and `docs/product-spec.md` remain the source of truth for locked rules, scope, APIs, and data contracts.

## Overview

```mermaid
flowchart TB
  User["Single local user"]
  FE["Frontend<br/>React + Vite"]
  BE["Backend<br/>Go net/http"]
  DB[("SQLite<br/>app metadata")]
  Repo[("Workspace repo<br/>source files + Git history")]
  AppData[("~/.patchpilot<br/>PatchPilot-owned state")]

  User --> FE
  FE -->|"REST JSON"| BE
  FE -->|"SSE events"| BE
  BE --> DB
  BE --> Repo
  DB --> AppData
```

PatchPilot is a single-user, self-hosted app. The browser UI talks to the Go backend through REST and SSE. SQLite stores PatchPilot metadata. Workspace files stay in their original Git repository.

## Backend

```mermaid
flowchart TB
  API["HTTP API"]
  Auth["Auth/session"]
  Workspace["Workspace"]
  Agent["Agent runs"]
  Patch["Patch review/apply"]
  Runner["Command runner"]
  Git["Git adapter"]
  Files["File service"]
  Ports["Port proxy"]
  Events["SSE"]
  DB[("SQLite")]
  Repo[("Workspace repo")]

  API --> Auth
  API --> Workspace
  API --> Agent
  API --> Patch
  API --> Runner
  API --> Git
  API --> Files
  API --> Ports
  API --> Events

  Auth --> DB
  Workspace --> DB
  Agent --> DB
  Patch --> DB
  Runner --> DB
  Events --> DB

  Workspace --> Repo
  Agent --> Files
  Agent --> Git
  Agent --> Runner
  Patch --> Repo
  Runner --> Repo
  Git --> Repo
  Files --> Repo
  Ports --> Runner
```

Backend modules:

- `cmd/patchpilot`: application entrypoint.
- `internal/api`: HTTP routes, handlers, SSE, and preview proxy.
- `internal/config`: runtime configuration.
- `internal/database`: SQLite connection and manual migrations.
- `internal/workspace`: allowed workspace validation and metadata.
- `internal/filestore`: safe workspace file access.
- `internal/gitrepo`: Git status, diff, and commit operations.
- `internal/runner`: workspace-root command execution.
- `internal/events`: SSE fan-out for realtime command lifecycle and output.

The command runner creates durable command records before process start, runs
commands without a shell from the workspace root, appends stdout/stderr chunks
to SQLite, and publishes `process.started`, `command.output`, and
`process.exited` events. SSE clients receive live events plus durable command
replay for the latest output.

## Frontend

```mermaid
flowchart TB
  App["App shell"]
  Routes["Routes"]
  Query["TanStack Query"]
  API["Shared API client"]
  Launcher["Launcher"]
  Vibe["Vibe Mode"]
  Workspace["Workspace Mode"]
  UI["Shared UI"]
  BE["Go backend"]

  App --> Routes
  App --> UI
  Routes --> Launcher
  Routes --> Vibe
  Routes --> Workspace
  Launcher --> Query
  Vibe --> Query
  Workspace --> Query
  Query --> API
  API --> BE
```

Frontend modules:

- `web/src/app`: shell, routes, theme, default route behavior.
- `web/src/features/vibe`: conversation chat, agent run activity, and tool approval.
- `web/src/features/workspace`: files, Git, commands, and preview tools.
- `web/src/shared/api`: typed API functions over the shared Axios client.
- `web/src/shared/ui`: reusable UI primitives.
- `web/src/shared/styles`: global Tailwind theme and CSS.

## Storage

```mermaid
flowchart LR
  Env["Env / .env config"]
  Server["PatchPilot server"]
  SQLite[("patchpilot.db")]
  Repo[("Workspace repo")]
  Git[("Git history")]

  Env --> Server
  Server --> SQLite
  Server --> Repo
  Repo --> Git
```

SQLite stores conversations, messages, agent runs, events, tool calls, commands, command output, ports, and Git snapshots. Source files remain on disk in the workspace repo.

## Agent Tool Flow

```mermaid
sequenceDiagram
  actor User
  participant FE as Frontend
  participant BE as Backend
  participant Agent as Agent run
  participant Repo as Workspace repo
  participant DB as SQLite

  User->>FE: Send chat message
  FE->>BE: Create message and agent run
  BE->>DB: Store conversation message and run
  BE->>Agent: Run agent loop
  Agent->>Repo: Read/search approved files
  Agent->>DB: Store events and tool calls
  BE-->>FE: Stream progress via SSE
  FE-->>User: Show tool output or approval request
  User->>FE: Approve or reject approval-required tools
  FE->>BE: Record tool decision
  BE->>Repo: Execute approved mutating tools
  BE->>DB: Update status
  FE-->>User: Show Git status
```

Agents inspect approved context and request tools. File mutations happen only through approved tool execution.
