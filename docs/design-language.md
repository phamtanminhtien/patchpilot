# PatchPilot Design Language

Read before creating or changing UI. This file defines stable product-screen
rules; scope still comes from `docs/project-rules.md` and
`docs/product-spec.md`.

## North Star

PatchPilot has two related surfaces:

- Vibe Mode: guided AI coding centered on prompt, task progress, and approvals.
- Workspace Mode: compact repository control centered on files, Git, terminal,
  preview, and output.

Both are work-focused. Vibe may feel conversational and centered; Workspace must
feel denser, quieter, and tool-like.

Interface qualities:

- Calm: restrained color, quiet surfaces, no visual noise.
- Dense but readable: useful information close together without crowding.
- Touch-first: primary mobile/iPad actions are easy to hit.
- Patch-first: summaries, approvals, terminal state, and Git state beat editor
  chrome.
- Operational: copy names concrete actions and states, not marketing claims.

## Shared Rules

Visual direction:

- Prefer flat, work-focused layouts over decorative presentation.
- Cards only for repeated items, side panels, modals, or framed tools; never nest
  cards.
- No landing-page heroes, decorative blobs/orbs/bokeh, one-hue dominant palettes,
  oversized panel typography, or marketing copy in workflow screens.
- Default UI is soft-borderless: prefer spacing, opaque surface contrast, and
  shadow before outlines.

Tokens:

- Tokens live in `web/src/shared/styles/global.css` as CSS variables exposed
  through Tailwind theme variables.
- Components use semantic Tailwind tokens, not hardcoded hex/rgb/hsl values, raw
  palette utilities, or ad hoc spacing/radius values.
- Families: `--pp-bg-*`, `--pp-color-*`, `--pp-space-*`, `--pp-radius-*`,
  `--pp-shadow-*`, `--pp-focus-*`.
- Background roles: `canvas`, `panel`, `hover`, `composer`, `composer-bar`,
  `accent-soft`.
- Color roles: `ink`, `muted`, `line`, `accent`, `accent-ink`, `warning`.
- Use `accent` sparingly for primary actions, active navigation, focus points,
  and meaningful highlights.
- Use `accent-soft` for low-emphasis selected/icon/badge surfaces, `hover` for
  neutral hover, and `warning` only for failed/risky/destructive states.
- Prefer spacing, contrast, and shadow before `line`; use `line` only where it
  improves scanability or focus/error clarity.
- Light/dark values stay paired under the same semantic names.
- Spacing, radius, and elevation changes update token values instead of one-off
  component utilities.

Layout:

- Mobile first; every primary workflow must work at 320px width.
- iPad may split into columns only when both columns remain useful.
- Desktop can be denser but must not become VS Code parity.
- Workflow screens lock the app shell to full viewport width/height. Document
  scrolling and overscroll are disabled; overflow-prone panels scroll internally.
- Primary actions are at least 44px tall.
- Fixed-format controls have stable dimensions so icons, labels, counters,
  loading states, and hover states do not shift layout.
- Text wraps or truncates intentionally, especially paths, command labels, branch
  names, and patch filenames.

App shell:

- Workflow screens use a compact top bar with mode switch, workspace signal, and
  theme control.
- App-wide settings are reached from the compact top bar as a utility route; they
  do not become a third workflow mode.
- Theme control exposes `System`, `Light`, `Dark`; `System` leaves root theme to
  CSS media queries.
- No workspace selected: show centered starter launcher instead of workflow
  chrome, app header, or mode switch.
- Starter launcher includes compact `Open repo`, lightweight theme control, and
  recent workspaces when available.
- Selecting a recent workspace sets `workspaceId` without changing the selected
  mode.

Typography and copy:

- Use global sans theme font. Do not scale font size with viewport width.
- Appearance settings may change app, code, and terminal font roles. Font values
  may be built-in stacks, installed local font files, or custom OS-resolved
  font-family fallback stacks. They must flow through CSS variables/Tailwind
  theme roles or xterm options, not one-off component font-family strings.
  Installed fonts are local same-origin files only; no CSS CDN or remote font
  loading in workflow UI.
- Letter spacing stays normal unless matching an existing uppercase-label
  pattern.
- Panel headings should be compact; `text-lg` or `text-xl` is usually enough.
- Copy describes what happened, what is needed, or what action is available.
- Prefer concrete verbs: `Open repo`, `New terminal`, `Apply patch`,
  `Reject patch`, `Stage`, `Unstage`, `Discard`, `Stop`, `Expose`,
  `Open preview`, `Commit`.
- Avoid vague labels like `Continue`, `Submit`, or `Next` when the action is
  known.
- Error copy is short, actionable, and must not expose stack traces, secrets, raw
  env, or host paths outside the workspace root.

Components and interaction:

- Shared primitives live in `web/src/shared/ui`; repeated Tailwind class patterns
  should become shared primitives.
- Use Radix for accessible menu, dialog, tab, toggle, tooltip, or similar
  behavior.
- Use `lucide-react` icons inside tool buttons when an icon exists.
- Icon-only controls need accessible labels and, when not obvious, tooltips.
- Buttons use the shared `Button` primitive unless there is a clear reason to
  extend it.
- Loading, empty, error, pending approval, open terminal, applied/rejected, and
  disabled states must be explicit.
- Dangerous or irreversible actions need confirmation or disabled states based on
  available data.
- Do not explain obvious UI mechanics with visible instructional text; show state
  and available actions.

## Vibe Mode

Vibe Mode keeps attention on the prompt, active task, and approvals.

- Use a centered composer, conversation/task thread, agent activity, task history,
  and approval review.
- Composer is primary; it may use stronger elevation, `composer`/
  `composer-bar` backgrounds, and slightly larger page-level type than Workspace.
- Keep the primary column centered and readable. Desktop may add a task-history
  rail; mobile stacks task context in or below the main flow.
- Context panels explain task state, model/reasoning, approvals, and recent
  activity; they must not become file browsers or IDE sidebars.
- Vibe may use warmer spacing: larger composer padding, softer grouping, and
  more breathing room around the prompt.
- Use accent for send/start, active task, approvals, and meaningful agent-state
  highlights.
- Task cards and tool-review blocks may be card-like because they are repeated
  review objects; keep them compact and scannable.
- Show summary before detail. Users should understand task status before raw
  event output.
- Patch/tool review prioritizes approve/reject clarity, changed-file
  scanability, and mobile diff readability.
- Approval-required tools show what is being approved, why it is blocked, and
  whether earlier approvals are pending.
- Model, reasoning, permissions, and workspace selectors stay secondary to prompt
  and task progress.

## Workspace Mode

Workspace Mode is a compact operations console, not a conversation page or IDE
clone.

- Stable layout: activity navigation, contextual sidebar, primary work panel,
  and a persistent bottom terminal panel.
- Allowed primary panels: Files, Git, Preview. Terminal is always available in
  the bottom panel and does not replace the primary work panel.
- Activity navigation is stable and readable on mobile; desktop can use a narrow
  icon rail.
- Sidebar holds navigation, selections, commit controls, detected ports, and
  small contextual actions; keep it narrow and utilitarian.
- Main panel shows selected tool content: file readout, diff, or preview state.
- Bottom panel hosts the Workspace terminal emulator. Terminal session tabs live
  on the right side of that panel so the shell surface stays stable while users
  switch primary panels.
- Do not add multi-tab editors, LSP surfaces, branch management, marketplace
  panels, or other IDE parity features. Terminal emulator behavior is allowed
  only inside the Workspace bottom terminal panel.
- Prefer `text-xs`/`text-sm`, tight rows, fixed rails, compact buttons, and
  stable grids.
- Avoid large centered headings, wide composer controls, decorative starter
  language, or spacious chat rhythm inside an active workspace.
- Use `line` more readily than Vibe only for dense scanability: file rows, Git
  groups, bottom tabs, diff/output regions, and panel boundaries.
- Paths, branch-like labels, terminal titles, filenames, and status output truncate
  or wrap without resizing rails or controls.
- Workspace prioritizes direct control: inspect files, search/list, view diffs,
  stage/unstage/discard selected changes, open/close terminal sessions in the
  persistent bottom panel, expose/open previews, and commit selected work.
- Active panel, selected file/change, staged status, open terminal, exposed
  port, loading, and error states must be visually explicit.
- Terminal output and Git status are operational data: use monospace terminal
  surfaces, compact summaries, and internal scrolling.

## Settings

Settings is a compact app-wide utility screen for local/server configuration.

- Keep the screen dense and operational: category navigation, status rows,
  compact controls, and explicit save/loading/error states.
- Desktop uses a narrow category rail plus one scrollable content region. Mobile
  uses horizontally scrollable category tabs or segmented controls with 44px
  targets.
- Appearance includes theme, app font, code font, terminal font, custom
  font-family entry, installed font management, and live previews. Previews must
  be compact and must not shift the surrounding layout while fonts load.
- Server config shows safe status only. Never render tokens, raw env values,
  secret placeholders, or host paths outside safe workspace/config summaries.
- Settings may reuse Skills/MCP components from Vibe, but copy should stay about
  configuration state rather than agent task progress.

## UI Change Checklist

Before finishing a UI change:

- It works at mobile width with primary touch targets >=44px.
- Text does not overflow its container.
- Loading/empty/error/selected/disabled content does not shift layout.
- It uses design tokens, shared primitives, and approved libraries.
- Background, color, spacing, radius, shadow, and focus resolve through CSS
  variables/Tailwind theme tokens.
- It keeps active product scope and avoids full IDE behavior.
- It avoids nested cards, decorative blobs/orbs/bokeh, one-hue dominance, and
  marketing copy.
- It includes the narrowest relevant verification, or states why not.
