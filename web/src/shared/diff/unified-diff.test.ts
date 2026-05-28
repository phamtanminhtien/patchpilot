import { describe, expect, it } from "vitest";

import { parseUnifiedDiff } from "./unified-diff";

describe("parseUnifiedDiff", () => {
  it("groups files, hunks, and stats", () => {
    const diff = `diff --git a/src/app.ts b/src/app.ts
index 1111111..2222222 100644
--- a/src/app.ts
+++ b/src/app.ts
@@ -1,3 +1,4 @@
 const a = 1;
-const b = 2;
+const b = 3;
+const c = 4;
 export { a };
`;

    const summary = parseUnifiedDiff(diff);

    expect(summary).toMatchObject({ additions: 2, deletions: 1 });
    expect(summary.files[0]).toMatchObject({
      additions: 2,
      deletions: 1,
      path: "src/app.ts",
    });
    expect(summary.files[0]?.hunks[0]?.patch).toContain("@@ -1,3 +1,4 @@");
  });

  it("returns no files for empty diffs", () => {
    expect(parseUnifiedDiff("\n")).toEqual({
      additions: 0,
      deletions: 0,
      files: [],
    });
  });

  it("does not count apply_patch wrapper lines as files", () => {
    const diff = `*** Begin Patch
*** Update File: web/index.html
@@ -1,3 +1,3 @@
 <head>
-  <title>Tirtea</title>
+  <title>ACB</title>
 </head>
*** End Patch
`;

    const summary = parseUnifiedDiff(diff);

    expect(summary).toMatchObject({ additions: 1, deletions: 1 });
    expect(summary.files).toHaveLength(1);
    expect(summary.files[0]).toMatchObject({ path: "web/index.html" });
  });

  it("deduplicates mixed git and apply_patch headers for the same file", () => {
    const diff = `diff --git a/apps/webui/index.html b/apps/webui/index.html
*** Begin Patch
*** Update File: apps/webui/index.html
@@ -7,3 +7,3 @@
-  <title>Tirtea</title>
+  <title>ACB</title>
*** End Patch
`;

    const summary = parseUnifiedDiff(diff);

    expect(summary.files).toHaveLength(1);
    expect(summary.files[0]).toMatchObject({
      additions: 1,
      deletions: 1,
      path: "apps/webui/index.html",
    });
  });
});
