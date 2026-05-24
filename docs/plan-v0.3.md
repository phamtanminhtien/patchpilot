# PatchPilot v0.3 Remaining Implementation Plan

This plan only covers v0.3 work that is not already present in the current
codebase. `docs/product-spec.md` remains the source of truth for behavior; this
file is a task breakdown for the remaining implementation slices.

## Already Implemented Baseline

Do not re-plan or rebuild these areas unless a later task needs a focused change:

- Manual SQLite migrations and conversation/run schema baseline exist in
  `internal/database`.
- Auth/session, workspace lifecycle, file access, Git operations, command runner,
  port proxy, conversations, agent runs, approval-gated built-in tools, SSE
  replay, Vibe chat, and Workspace tools have implemented packages and tests.
- Existing Vibe tool review supports grouped tool calls, approval actions,
  transient output, and smart scroll behavior.
- Existing Workspace Mode supports the Files, Git, Commands, and Preview surface.

## Task 1: Add AGENTS.md Instruction Discovery

**Description:** Implement safe discovery of root and task-relevant descendant
`AGENTS.md` files during agent-context refresh and run creation. Effective
instructions must preserve source order, precedence, and safe skipped-file
warnings.

**Acceptance criteria:**

- [ ] Discovery reads `AGENTS.md` files from the workspace filesystem, not a DB
      registry.
- [ ] Root instructions are included before task-relevant descendant
      instructions.
- [ ] Files outside the workspace root, symlink escapes, secret-like paths,
      binaries, and oversized files are rejected with safe warnings.
- [ ] Warning text never exposes host paths outside the workspace root, secrets,
      or raw env values.
- [ ] Agent runs receive effective repo instructions separately from
      conversation messages.

**Verification:**

- [ ] `go test ./internal/agent ./internal/filestore ./internal/api`
- [ ] Manual check: create nested `AGENTS.md` files in a test workspace and
      confirm effective source order and warnings.

**Dependencies:** Existing workspace, filestore, and agent context baseline.

**Likely touched areas:** `internal/agent`, `internal/filestore`,
`internal/api`.

**Estimated scope:** Medium

## Task 2: Add Agent Context Refresh API

**Description:** Add the v0.3 context refresh/read endpoint that reports effective
instructions, selected skills, MCP server/tool summaries, context-budget warnings,
and refresh time. This endpoint should be the backend contract used by Vibe
context/cockpit UI.

**Acceptance criteria:**

- [ ] Refresh rereads applicable `AGENTS.md` files and current local config.
- [ ] Response includes effective instruction sources, skipped-source warnings,
      selected skill summaries, MCP summaries, context-budget warnings, and
      refreshed timestamp.
- [ ] Failures use the standard error envelope and do not leak host paths outside
      the workspace root.
- [ ] The endpoint requires a valid session cookie and is scoped by
      `workspaceId`.

**Verification:**

- [ ] `go test ./internal/api ./internal/agent`

**Dependencies:** Task 1

**Likely touched areas:** `internal/api`, `internal/agent`,
`web/src/shared/api`.

**Estimated scope:** Medium

## Task 3: Implement Local Skills Registry and Context Injection

**Description:** Add local skill discovery from `~/.patchpilot/skills` first and
`~/.agent/skills` second, merge enablement from `~/.patchpilot/config.json`, and
inject bounded selected skill context into future agent runs.

**Acceptance criteria:**

- [ ] Skills are discovered from `~/.patchpilot/skills` and `~/.agent/skills`.
- [ ] Duplicate skill keys use only the `~/.patchpilot/skills` copy for the
      effective skill list and injected context.
- [ ] Missing `config.skills` entries default enabled.
- [ ] Invalid skills remain visible with safe warnings and are not injected.
- [ ] Enable/disable API updates config and affects future runs.
- [ ] Runs inject concise selected skill instructions plus directly needed
      supporting context, not whole directories.
- [ ] No remote skill installation, marketplace sync, or public discovery is
      introduced.

**Verification:**

- [ ] `go test ./internal/skills ./internal/agent ./internal/api`
- [ ] Manual check: duplicate local skills prefer `~/.patchpilot/skills`.

**Dependencies:** Task 2

**Likely touched areas:** `internal/skills`, `internal/config`,
`internal/agent`, `internal/api`, `web/src/shared/api`.

**Estimated scope:** Large

## Task 4: Implement MCP Registry, Discovery, and Backend Bridge

**Description:** Add configured MCP server support from
`~/.patchpilot/config.json`, including stdio/HTTP discovery, cached health/tool
metadata, approval policy, and backend-only tool execution.

**Acceptance criteria:**

- [ ] Configured stdio servers are managed as backend child processes.
- [ ] Configured HTTP servers use explicit URLs only; no network scanning or
      public discovery is added.
- [ ] Disabled servers do not start.
- [ ] Unresolved `${ENV_NAME}` placeholders produce safe warnings without
      exposing secret values.
- [ ] Tool/resource list shows server, transport, metadata, health, disabled
      state, last error, read-only hints, and effective approval policy.
- [ ] Unknown or mutating MCP tools require approval.
- [ ] Read-only MCP auto-run requires both server/tool metadata and PatchPilot
      policy to mark the tool safe.
- [ ] Frontend and agents never call MCP servers directly; all calls go through
      the backend bridge and durable tool-call flow.

**Verification:**

- [ ] `go test ./internal/mcp ./internal/agent ./internal/api`
- [ ] Manual check: fake stdio and HTTP MCP servers can be refreshed, listed, and
      called through approval flow.

**Dependencies:** Task 2

**Likely touched areas:** `internal/mcp`, `internal/config`, `internal/agent`,
`internal/api`, `web/src/shared/api`.

**Estimated scope:** Large

## Task 5: Extend Agent Tool Calls for Skills and MCP Sources

**Description:** Extend the existing durable tool-call and approval flow so tool
calls can represent built-in, skill, and MCP sources with source references and
policy reasons.

**Acceptance criteria:**

- [ ] Tool-call records expose `source` as `builtin`, `skill`, or `mcp` and
      include `sourceRef` when applicable.
- [ ] Approval review can show server/tool name, source, input summary, and
      policy reason.
- [ ] Provider tool-call order remains preserved across mixed built-in and MCP
      batches.
- [ ] If any tool in a batch requires approval, approval-required calls in that
      batch wait for decisions before execution.
- [ ] Rejected tools do not run and later approval-required tools in the same
      blocked batch do not execute prematurely.

**Verification:**

- [ ] `go test ./internal/agent ./internal/api`
- [ ] `pnpm --dir web test`

**Dependencies:** Task 3, Task 4

**Likely touched areas:** `internal/database`, `internal/agent`,
`internal/api`, `web/src/features/vibe`, `web/src/shared/api`.

**Estimated scope:** Medium

## Task 6: Build Vibe Context, Skills, and MCP Cockpit

**Description:** Add the missing Vibe cockpit surfaces for effective
instructions, enabled skills, MCP servers/tools, context warnings, and run
details while keeping chat/run progress primary on mobile.

**Acceptance criteria:**

- [ ] Users can inspect effective instruction sources, precedence, and skipped
      warnings.
- [ ] Users can inspect discovered skills, invalid-skill warnings, enablement,
      and selected skills for future runs.
- [ ] Users can inspect MCP server status, tool/resource metadata, disabled
      state, last error, read-only hints, and effective approval policy.
- [ ] Approval cards show source metadata and policy reason for MCP tools.
- [ ] Long paths, skill names, server names, tool names, and JSON summaries wrap
      or truncate without layout shifts.
- [ ] Mobile uses tabs/sheets or equivalent compact surfaces so the primary chat
      flow remains usable at 320px width.

**Verification:**

- [ ] `pnpm --dir web test`
- [ ] `pnpm --dir web build`
- [ ] Playwright smoke: inspect cockpit context, skills, MCP, approvals, and run
      details at 320px and desktop widths.

**Dependencies:** Task 2, Task 3, Task 4, Task 5

**Likely touched areas:** `web/src/features/vibe`, `web/src/shared/api`,
`web/src/shared/ui`.

**Estimated scope:** Large

## Task 7: Harden Remaining v0.3 Safety Cases

**Description:** Add focused tests for the remaining high-risk v0.3 acceptance
criteria that are not covered by the current baseline, especially context,
skills, MCP, and cockpit layout behavior.

**Acceptance criteria:**

- [ ] Instruction context rejects workspace escapes, external symlinks,
      secret-like paths, binaries, and oversized files.
- [ ] Skill parser preserves invalid-skill warnings, respects config enablement,
      applies duplicate precedence, and injects bounded selected context only.
- [ ] MCP fake servers cover add/refresh/list/call behavior, disabled servers,
      unresolved env placeholders, mutating approval, and read-only auto-run
      policy.
- [ ] Vibe cockpit tests cover long paths/tool names/server names/JSON summaries
      without layout shifts.
- [ ] Any remaining acceptance criterion in `docs/product-spec.md` without
      automated coverage is documented with a reason.

**Verification:**

- [ ] `go test ./...`
- [ ] `pnpm --dir web test`
- [ ] `pnpm --dir web build`
- [ ] Playwright smoke for Vibe cockpit on mobile and desktop widths.

**Dependencies:** Tasks 1-6

**Likely touched areas:** Backend tests, frontend tests, Playwright specs.

**Estimated scope:** Large

## Task 8: Align Documentation After Remaining Work Lands

**Description:** Update documentation after the remaining v0.3 work is
implemented so README, architecture notes, and product spec terminology match the
actual shipped behavior.

**Acceptance criteria:**

- [ ] README product direction and documentation links refer to v0.3 behavior.
- [ ] Architecture notes mention implemented Skills and MCP modules only after
      they exist.
- [ ] Product spec remains authoritative and is updated before behavior/API/data
      changes.
- [ ] Release readiness remains in `docs/release.md`; this plan does not
      duplicate the release checklist.

**Verification:**

- [ ] Manual docs review against `docs/product-spec.md`.
- [ ] `go test ./...`
- [ ] `pnpm --dir web test`
- [ ] `pnpm --dir web build`

**Dependencies:** Tasks 1-7

**Likely touched areas:** `README.md`, `docs/app-architecture.md`,
`docs/product-spec.md`, `docs/release.md`.

**Estimated scope:** Medium

## Checkpoints

- [ ] After Tasks 1-2: context refresh exposes effective AGENTS.md instructions
      and safe warnings.
- [ ] After Task 3: local skills can be discovered, enabled/disabled, and
      injected into future runs.
- [ ] After Task 4: configured MCP servers can be discovered and called only
      through backend policy.
- [ ] After Tasks 5-6: Vibe cockpit shows context, skills, MCP, approvals, and
      run details on mobile and desktop.
- [ ] After Tasks 7-8: full verification passes and docs match shipped v0.3
      behavior.

## Parallelization Notes

- Tasks 3 and 4 can proceed in parallel after Task 2 defines the shared context
  refresh contract.
- Task 6 can start with mocked API data after Task 2, but final wiring depends on
  Tasks 3-5.
- Task 7 should start as each slice lands, then finish after Task 6.

## Risks and Mitigations

| Risk | Impact | Mitigation |
| ---- | ------ | ---------- |
| Context leaks unsafe file content | High | Reuse central workspace-root, symlink, size, binary, and secret-path checks before provider injection. |
| MCP execution bypasses approval policy | High | Keep all MCP calls behind backend durable tool calls and policy checks. |
| Skill injection becomes too broad | Medium | Inject only concise selected skill instructions and directly needed supporting context. |
| Cockpit overwhelms mobile Vibe flow | Medium | Keep chat/run progress primary and move dense context into compact tabs or sheets. |
| Docs drift from implementation | Medium | Update docs after each behavior/API/data change and verify against `docs/product-spec.md`. |

