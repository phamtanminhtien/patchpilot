# PatchPilot

Lightweight AI coding workspace

## Vision
A self-hosted AI coding workspace that runs on a server and is optimized for mobile, iPad, and browser-based development.

It is not a VS Code clone.
It focuses on:
- Chatting with AI
- Reviewing patches
- Running commands
- Web terminal support
- Exposing preview URLs
- Quick commits
- Vibe coding from any device

> The fastest way to collaborate with coding agents from any device.

---

# Product Direction
## PatchPilot
Main flow:
1. Open a repository
2. Chat with an agent
3. Agent generates a patch
4. Review/apply diff
5. Run tests/commands
6. Commit/push

The product should not try to become a full IDE.

---

# Core Idea
The server owns:
- Source code
- Git
- Agent runtime
- Terminal sessions
- Runtime processes
- Preview/logs
- Port forwarding

The web client has two main interfaces:

## 1. Vibe Mode
A pure vibe-coding interface, similar to the current Codex experience.

Focus areas:
- Chat with AI
- Task/session list
- Agent progress
- Patch summary
- Test/result summary
- One-tap approve/reject
- Quick commit/push

Users do not need to constantly look at the file tree.

## 2. Workspace Mode
A lightweight IDE-like workspace interface with file explorer and editor.

Focus areas:
- Browse files
- Open/edit files
- Search files
- Review detailed diffs
- Terminal
- Logs
- Preview
- Git status

Users can switch between the two modes depending on their needs.

---

# Target Users
- Indie hackers
- Developers using iPad/mobile devices
- Small self-hosted teams
- AI-first developers

---

# Key Differentiators
## vs code-server / VS Code Web
- Lighter
- Mobile-first
- Agent-first
- Simpler UI
- Does not try to clone a desktop IDE

## vs Cursor/Windsurf
- Self-hosted OSS
- Browser-first
- No desktop app required

## vs Codespaces
- Cheaper
- More minimal
- Friendly for individuals and small teams

---

# Client UX Modes
## Vibe Mode
Vibe Mode is the default mode for mobile and iPad.

Goals:
- Minimal UI
- Chat-first experience
- No requirement for users to manually manage files
- Suitable for fixing bugs or implementing features with AI

Main screens:
- Sessions
- Chat
- Plan
- Changes
- Run result
- Commit

Typical flow:
```txt
Open workspace
→ Ask AI
→ Watch agent progress
→ Review summary/diff
→ Approve changes
→ Run tests
→ Commit/push
```

## Workspace Mode
Workspace Mode is the more complete mode for direct code intervention.

Goals:
- File explorer
- Editor
- Terminal
- Diff viewer
- Logs/preview

Main panels:
- File explorer
- Editor
- Chat
- Diff
- Terminal
- Preview
- Git panel

Typical flow:
```txt
Open workspace
→ Browse/search file
→ Edit or ask AI
→ Review diff
→ Run terminal/test
→ Commit/push
```

## Mode Switching
Users can switch modes within the same workspace.

Rules:
- Vibe Mode prioritizes simplicity
- Workspace Mode prioritizes control
- Both modes share the same backend, session, and state

---

# MVP Scope
## In MVP
### Workspace
- Open local repository
- Basic workspace indexing
- Basic file viewer
- File search

### Vibe Mode
- Chat-first UI
- Task/session list
- Agent progress timeline
- Patch summary
- Run result summary
- One-tap approve/reject

### AI Workflow
- Agent can read/search files
- Agent proposes a patch instead of directly mutating files
- User reviews diff before applying
- Apply/revert patch
- Session history

### Runtime
- Command runner
- Realtime command output streaming
- Basic process lifecycle
- Port proxy for preview

### Git
- Status
- Diff
- Commit

## Post-MVP
- Full Workspace Mode editor
- Full web terminal
- Public tunnel network
- Plugin system
- Docker isolation
- Push/pull/branch management
- Multi-tab editor
- LSP support
- Inline diagnostics
- Team collaboration

## Important Scope Rule
MVP prioritizes Vibe Mode first.

Workspace Mode in MVP only needs enough functionality to:
- View files
- View diffs
- Make small manual interventions when needed

Do not build a full IDE in MVP.

---

# MVP Acceptance Criteria
- User can open a local repository as a workspace.
- User can ask AI to modify code.
- Agent can inspect workspace files through approved tools.
- Agent returns a patch instead of directly mutating files.
- User can review diff before applying changes.
- User can approve or reject a patch.
- User can run a command and see streamed output.
- User can see command success/failure result.
- User can view Git status and diff.
- User can commit applied changes.
- User can preview a running app through the server port proxy.
- Mobile/iPad user can complete the AI coding flow from Vibe Mode.

---

# User Flows
## Workspace Flow
```txt
Open app
→ Select repo/local workspace
→ Workspace indexing
→ Restore previous session
→ Workspace ready
```

## AI Coding Flow
```txt
User asks AI
→ Agent reads/searches code
→ Agent proposes patch
→ Diff review UI
→ User approve/reject
→ Apply patch
→ Run test/command
→ Commit changes
```

## Terminal Flow
```txt
Open terminal
→ Create PTY session
→ Stream realtime output
→ Reconnect on refresh
→ Optional background session
```

## Port Sharing Flow
```txt
Dev server starts
→ System detects opened port
→ User clicks expose
→ HTTPS preview URL is generated under the same host
→ User opens or shares preview link if allowed
```

## Git Flow
```txt
Modified files detected
→ Review diff
→ Stage changes
→ Commit
→ Push/pull
```

---

# Mobile-first UX
## Vibe Mode UX
- Chat-first
- Timeline/session-first
- Large action buttons
- Minimal panels
- Summary before details
- One-tap approve/reject

## Workspace Mode UX
- File explorer + editor + chat
- Bottom navigation on mobile
- Split panes on iPad/Desktop
- Large touch targets
- Mobile tabs:
  - Chat
  - Files
  - Diff
  - Terminal
  - Preview
  - Git

## Responsive Strategy
- Mobile default: Vibe Mode
- iPad default: Vibe Mode + optional split view
- Desktop default: Workspace Mode
- User can manually switch anytime

---

# Architecture
## Backend
- Go
- Single lightweight binary
- REST + WebSocket/SSE
- Workspace manager
- Git integration
- Agent runtime
- PTY terminal manager
- Sandbox per workspace
- SQLite embedded DB

## Frontend
- React + Vite
- CodeMirror 6
- xterm.js
- Shared state between Vibe Mode and Workspace Mode
- Route-based mode switching

## Agent Providers
- OpenAI-compatible
- Anthropic-compatible
- Ollama/local model

---

# Agent Execution Model
## Agent Task
Each AI request creates one agent task inside a workspace.

A task includes:
- User prompt
- Plan
- Tool calls
- Progress logs
- Generated patch
- Command results
- Final summary

## Task States
```txt
queued → running → waiting_approval → applying → testing → done
                         ↓
                      rejected
                         ↓
                       failed
```

## Agent Rules
- Agent must not directly modify files.
- Agent must generate a patch first.
- Server applies the patch only after user approval.
- Agent must not expose ports automatically.
- Agent must not read secrets directly.
- Command execution requires user approval if the command is dangerous or not in the allowlist.

## Tool Calling
Supported tools:
- read_file
- list_files
- search_files
- git_diff
- git_status
- propose_patch
- run_command

## Streaming
Client receives realtime events for:
- agent text delta
- tool call started/finished
- command output
- patch created
- approval required
- task status changed

---

# Workspace & Runtime Model
## Workspace Definition
A workspace is an isolated development environment containing:
- Source code
- Git repository
- Terminal sessions
- Runtime processes
- AI session history
- Plugin state

## Workspace Isolation
MVP:
- One workspace = one local directory
- Optimized for single-user usage
- Logical isolation only
- Commands run as the same OS user as the server process
- Working directory is restricted to the workspace root
- No strong sandbox guarantee unless Docker mode is enabled

Future:
- Docker isolation
- Multi-tenant runtime
- Remote workspace scheduling

## Runtime Model
The server manages:
- Long-running processes
- Dev servers
- PTY terminal sessions
- Port forwarding
- Background tasks

Supported runtimes:
- Node.js
- Go
- Python
- Docker Compose

## Process Management
- Start/stop/restart process
- Process logs
- Port detection
- Auto reconnect
- Crash detection
- Background persistence

## Storage
MVP:
- Local filesystem
- SQLite embedded DB
- Default app space: `~/.patchpilot`

Stored data:
- Workspace metadata
- AI sessions
- Terminal history
- Plugin state
- Runtime state

---

# Security Model
## Security Boundary
MVP targets single-user self-hosting.

In single-user self-host mode:
- Terminal/commands run with the server OS user permission.
- The security boundary is the host OS user, not the app itself.
- App-level checks prevent accidental unsafe actions, not strong sandbox escape protection.

In cloud/team mode:
- Every workspace must run inside an isolated container.
- Strong isolation is required before supporting untrusted users.

## Security Principles
- Default deny
- Explicit permissions
- Workspace isolation
- Human approval for destructive actions

## AI Restrictions
AI must not:
- Auto-apply patches without approval
- Access files outside the workspace
- Read secrets directly
- Execute privileged commands
- Expose ports automatically

## Command Execution
Allowed:
- Project-scoped commands
- User-approved runtime commands
- Full terminal access in single-user self-host mode

Restricted by default:
- AI-initiated dangerous commands
- Commands outside the workspace root
- Commands requiring elevated privileges

Note:
- In MVP self-host mode, restrictions are guardrails, not a hard sandbox.
- In cloud/team mode, command execution must happen inside containers.

## Port Exposure Security
MVP uses Option A: server-side port proxy.

Example:
```txt
https://host.devspace.com/workspaces/:workspaceId/ports/:port
```

MVP rules:
- No separate public tunnel network
- Proxy only through the same host/domain
- Manual expose action required
- Optional auth protection
- Port lifecycle tied to workspace/process

Post-MVP:
- Public tunnel network
- Temporary public URLs
- Rate limiting
- Share links

## Plugin Security
- Must declare permissions
- Sandboxed through ctx APIs
- No direct system access

## Secrets Handling
- Stored encrypted locally
- Never exposed directly to AI
- Injected into runtime environment only
- Scoped per workspace

---

# Post-MVP Extension System
## Goal
Allow the workspace to be extended without turning the app into a VS Code clone.

Plugins are used for:
- Agent tools
- Command templates
- Small UI panels
- External integrations
- Runtime/framework support

## Plugin Types
### Tool Plugin
Expose additional tools to the AI agent.

### Command Plugin
Add reusable commands.

### UI Plugin
Add small panels/tabs.

### Runtime Plugin
Support a framework/runtime.

## Plugin Manifest
Each plugin has a `plugin.json` file.

```json
{
  "id": "nextjs-runtime",
  "name": "Next.js Runtime",
  "version": "0.1.0"
}
```

## Permission Model
Suggested permissions:
- `fs:read`
- `fs:write`
- `process:run`
- `network:outbound`
- `git:read`
- `git:write`
- `ports:read`
- `ports:expose`
- `ui:panel`

Rule:
> Plugins must not directly access the filesystem or processes.

## Plugin Install
```txt
.plugins/
  nextjs-runtime/
    plugin.json
    dist/index.js
```

## Plugin Lifecycle
```txt
install → load → check permissions → activate → register → deactivate
```

## MVP Decision
Plugin system is not part of MVP.

MVP should use built-in adapters instead:
- Git adapter
- Node.js runtime adapter
- Go runtime adapter
- Docker Compose adapter
- Terminal adapter
- Port proxy adapter

Post-MVP plugin scope:
1. Command Plugin
2. Agent Tool Plugin
3. Runtime Plugin

---

# API & Event Model
## REST API
All workspace resources are scoped by workspace ID.

### Workspace
- `POST /workspaces`
- `GET /workspaces/:workspaceId`
- `DELETE /workspaces/:workspaceId`

### Agent Tasks
- `POST /workspaces/:workspaceId/agent/tasks`
- `GET /workspaces/:workspaceId/agent/tasks`
- `GET /workspaces/:workspaceId/agent/tasks/:taskId`
- `POST /workspaces/:workspaceId/agent/tasks/:taskId/cancel`

### Files
- `GET /workspaces/:workspaceId/files?path=`
- `GET /workspaces/:workspaceId/file?path=`
- `PUT /workspaces/:workspaceId/file`
- `GET /workspaces/:workspaceId/search?q=`

### Patches
- `GET /workspaces/:workspaceId/patches/:patchId`
- `POST /workspaces/:workspaceId/patches/:patchId/apply`
- `POST /workspaces/:workspaceId/patches/:patchId/reject`
- `POST /workspaces/:workspaceId/patches/:patchId/revert`

### Git
- `GET /workspaces/:workspaceId/git/status`
- `GET /workspaces/:workspaceId/git/diff`
- `POST /workspaces/:workspaceId/git/commit`

### Runtime
- `POST /workspaces/:workspaceId/commands`
- `POST /workspaces/:workspaceId/processes/:processId/stop`
- `GET /workspaces/:workspaceId/ports`
- `POST /workspaces/:workspaceId/ports/:port/expose`

## WebSocket/SSE Events
### Client → Server
- `agent.message`
- `agent.cancel`
- `terminal.input`
- `workspace.open`
- `patch.apply`
- `patch.reject`
- `command.run`
- `port.expose`

### Server → Client
- `agent.delta`
- `agent.tool.started`
- `agent.tool.finished`
- `agent.approval_required`
- `agent.task.status_changed`
- `terminal.output`
- `process.started`
- `process.exited`
- `port.opened`
- `port.exposed`
- `git.changed`
- `patch.created`

---

# Suggested Stack
## Go + React
- Backend: Go
- Frontend: React + Vite
- Editor: CodeMirror 6
- Terminal: xterm.js + Go PTY bridge
- DB: SQLite
- Realtime: WebSocket/SSE
- Packaging: single binary + embedded web assets
- Docker optional

Why Go:
- Lightweight
- Single binary deployment
- Good self-host experience
- Easier process/runtime management
- Lower memory footprint

---

# Repo Structure
```txt
apps/
  web/
    src/
      modes/
        vibe/
        workspace/
      shared/

cmd/
  devspace/

internal/
  agent/
  git/
  workspace/
  runner/
  terminal/
  ports/
  plugins/
  sessions/
  db/

web/
  dist/
```

---

# Runtime Ideas
## Web Terminal
Post-MVP full terminal.

Features:
- Browser terminal with xterm.js
- Mobile-friendly gestures
- Multiple sessions
- Session reconnect

MVP can start with command runner first.

## Port Proxy First
MVP exposes local dev servers through the same host/domain.

Features:
- Auto-detect opened ports
- One-click expose
- Preview URL under the same host
- Share preview link if auth allows

## Public Tunnel Later
Post-MVP public tunnel can support:
- Temporary public URL
- Webhook testing
- External sharing
- Rate limiting

---

# Killer Features
## One-tap fix
Select an error log → AI fixes it.

## Patch-first UX
Review/apply patches instead of manually editing raw files most of the time.

## AI Timeline
Rewind all AI changes.

## Workspace Snapshot
Create a snapshot before AI modifies files for quick rollback.

---

# Non-functional Requirements
## Performance Targets
- Startup < 2s
- Workspace open < 5s
- Memory target < 300MB idle
- Mobile-friendly latency

## Browser Support
- Chrome
- Safari
- Edge
- iPad Safari

## Deployment
Supported deployment:
- Single binary
- Docker container
- Self-hosted VPS
- Local server

Recommended:
- Reverse proxy via Nginx/Caddy
- HTTPS enabled

---

# Roadmap
## Phase 1 — Core Infra
- Workspace manager
- File API
- Command runner
- Git wrapper
- SSE/WebSocket streaming

## Phase 2 — Vibe Mode Agent MVP
- Chat UI
- Agent task model
- Patch generation
- Diff review
- Apply/revert
- Run command and stream output
- Git status/commit

## Phase 3 — Mobile UX
- PWA
- Responsive layout
- Vibe Mode polish
- Gesture-friendly diff
- iPad shortcuts

## Phase 4 — Runtime Preview
- Preview iframe
- Background process
- Port proxy under the same host
- Basic process manager

## Phase 5 — Workspace Mode
- Full editor experience
- Full terminal
- Multi-tab editor
- Search/replace

## Phase 6 — Advanced Dev
- Docker workspace optional
- LSP support
- Inline diagnostics
- Plugin system

## Phase 7 — Team/SaaS
- Auth
- Shared workspace
- Hosted runtime
- Billing

---

# Monetization
## OSS Core
- Self-hosted
- Single-user
- Basic AI workflow

## Pro/Cloud
- Hosted workspace
- Team collaboration
- Persistent runtime
- Better sync/history

## Enterprise
- RBAC
- Audit logs
- Private model gateway
- On-prem deployment

---

# Main Rule
> Every feature must improve the mobile vibe-coding workflow.

If it does not, reject it or postpone it.
