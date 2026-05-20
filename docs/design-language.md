# PatchPilot Design Language

Use this file before creating or changing UI. It defines the stable design language for PatchPilot MVP screens.

## North Star

PatchPilot feels like a compact mobile control surface for AI-assisted coding, not a marketing site and not a browser IDE clone.

The interface should be:

- Calm: quiet surfaces, restrained color, no visual noise.
- Dense but readable: useful information is close together without feeling cramped.
- Touch-first: primary actions are easy to hit on mobile and iPad.
- Patch-first: summaries, approvals, command results, and Git state are easier to scan than raw editor chrome.
- Operational: UI copy names concrete actions and states instead of selling the product.

## Visual Direction

- Prefer flat, work-focused layouts over decorative presentation.
- Use full-width sections and constrained inner content for screen structure.
- Use cards only for repeated items, side panels, modals, or framed tools.
- Do not nest cards inside cards.
- Do not use landing-page heroes, gradient blobs, bokeh, decorative orbs, or one-hue dominant palettes in workflow screens.
- Avoid oversized typography in panels and tools; reserve large type for real page-level headings only.
- Keep borders subtle and use spacing to group information before adding more chrome.

## Design Tokens

Tokens live in `web/src/shared/styles/global.css` as CSS variables and are exposed to components through Tailwind theme variables. Components must use semantic Tailwind tokens, not hardcoded hex/rgb/hsl values, raw palette utilities, or ad hoc spacing/radius values.

Token families:

- Background: `--pp-bg-*`.
- Text, border, and state color: `--pp-color-*`.
- Spacing: `--pp-space-*`.
- Radius: `--pp-radius-*`.
- Shadow: `--pp-shadow-*`.
- Focus: `--pp-focus-*`.

Background roles:

- `canvas`: app background.
- `panel`: primary surface.
- `hover`: neutral hover and pressed surface.
- `accent-soft`: subtle accent surface.

Color roles:

- `ink`: primary text.
- `muted`: secondary text.
- `line`: borders and dividers.
- `accent`: primary action and selected state.
- `accent-ink`: text/icons on accent surfaces.
- `warning`: destructive, failed, or risky state.

Spacing and radius roles:

- `space-unit`: base spacing unit used by Tailwind spacing utilities.
- `space-touch`: minimum primary action size, currently 44px.
- `radius-sm`, `radius-md`, `radius-lg`: normal control and panel radii.

Token rules:

- Use `accent` sparingly for primary actions, active navigation, focus points, and meaningful highlights.
- Use `accent-soft` for low-emphasis selected, icon, or badge backgrounds.
- Use `hover` for neutral hover states instead of raw black/white opacity utilities.
- Use `warning` only for failed/risky/destructive states.
- Prefer `line` and spacing for separation before adding tinted backgrounds.
- Keep status colors semantic; do not introduce status-specific palettes without a real state model.
- Light and dark values must stay paired under the same semantic background/color variable names.
- Spacing and radius changes must update token values, not one-off component utilities.

## Layout

- Mobile first: every primary workflow must fit and work at 320px width.
- iPad should support split or two-column layouts only when both columns remain useful.
- Desktop can show denser workspace controls, but must not become VS Code parity.
- Default max content width is compact and readable; use `max-w-6xl` unless the screen has a stronger reason.
- Primary actions must be at least 44px tall.
- Fixed-format controls must have stable dimensions so icons, labels, counters, loading states, and hover states do not shift layout.
- Text must wrap or truncate intentionally. No uncontrolled overflow, especially for paths, command output labels, branch names, and patch filenames.

## Typography

- Use the global sans font stack from the theme.
- Do not scale font size with viewport width.
- Letter spacing stays normal unless the existing component pattern already uses uppercase labels.
- Use compact headings inside panels: `text-lg` or `text-xl` is usually enough.
- Body text should be direct and stateful: describe what happened, what is needed, or what action is available.

## Components

- Shared primitives live in `web/src/shared/ui`.
- Repeated Tailwind class patterns should become shared primitives.
- Use Radix primitives when a component needs accessible menu, dialog, tab, toggle, tooltip, or similar behavior.
- Use `lucide-react` icons inside tool buttons whenever an icon exists.
- Icon-only controls need accessible labels and, when not obvious, a tooltip.
- Buttons use the shared `Button` primitive unless there is a concrete reason to extend it.
- Form fields use shared input primitives once a pattern repeats.

## Interaction

- Vibe Mode shows summary before detail.
- Patch review prioritizes approve/reject clarity, changed-file scanability, and mobile diff readability.
- Workspace Mode prioritizes control but remains lightweight: files, search, diffs, small edits, command output, preview, Git status.
- Loading, empty, error, pending approval, running command, and applied/rejected states must be explicit.
- Dangerous or irreversible actions need a clear confirmation or disabled state based on available data.
- Do not use visible instructional text for obvious UI mechanics; show state and available actions instead.

## Copy

- Use concrete verbs: `Open repo`, `Run command`, `Apply patch`, `Reject patch`, `Commit`.
- Avoid marketing copy in workflow screens.
- Avoid vague labels like `Continue`, `Submit`, or `Next` when a specific action is known.
- Error copy should be short, actionable, and must not expose stack traces, secrets, raw env, or host paths outside the workspace root.

## UI Change Checklist

Before finishing a UI change:

- It works at mobile width and with touch targets of at least 44px for primary actions.
- Text does not overflow its container.
- The layout does not shift when content becomes loading, empty, errored, selected, or disabled.
- It uses existing design tokens, shared primitives, and approved libraries.
- Background, color, spacing, radius, shadow, and focus values resolve through CSS variables/Tailwind theme tokens.
- It keeps MVP scope and does not add full IDE behavior.
- It avoids nested cards, decorative blobs/orbs/bokeh, one-hue dominance, and marketing copy.
- It includes the narrowest relevant verification, or clearly states why verification was not run.
