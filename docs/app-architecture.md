# PatchPilot Architecture

This document summarizes the MVP architecture. `docs/project-rules.md` and `docs/mvp-spec.md` remain the source of truth for locked rules, scope, APIs, and data contracts.

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
  Agent["Agent tasks"]
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
- `internal/database`: SQLite connection and schema setup.
- `internal/workspace`: allowed workspace validation and metadata.
- `internal/filestore`: safe workspace file access.
- `internal/gitrepo`: Git status, diff, and commit operations.
- `internal/runner`: workspace-root command execution.

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
- `web/src/features/vibe`: AI task flow and patch review.
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

SQLite stores sessions, workspaces, agent tasks, events, patches, commands, command output, ports, and Git snapshots. Source files remain on disk in the workspace repo.

## Patch Flow

```mermaid
sequenceDiagram
  actor User
  participant FE as Frontend
  participant BE as Backend
  participant Agent as Agent task
  participant Repo as Workspace repo
  participant DB as SQLite

  User->>FE: Ask AI
  FE->>BE: Create task
  BE->>DB: Store task
  BE->>Agent: Run task
  Agent->>Repo: Read/search approved files
  Agent->>DB: Store events and proposed patch
  BE-->>FE: Stream progress via SSE
  FE-->>User: Show diff
  User->>FE: Approve or reject
  FE->>BE: Apply or reject patch
  BE->>Repo: Apply approved patch
  BE->>DB: Update status
  FE-->>User: Show Git status
```

Agents inspect approved context and propose patches. File mutations happen only after explicit user approval.
