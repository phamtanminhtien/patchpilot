# PatchPilot Agent Instructions

Before editing, read in order:

1. `docs/project-rules.md` - locked rules, highest authority.
2. `docs/product-spec.md` - active product scope, API, data, acceptance.
3. `docs/design-language.md` - required for UI changes.
4. Task-related files.

If the task conflicts with `docs/project-rules.md`, stop and ask for explicit approval.

For every change: stay within active scope, update docs first for behavior/API/data/stack/structure/scope changes, edit patch-first, run the narrowest verification, and report files plus results.
