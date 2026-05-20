import { describe, expect, it } from "vitest";

import { parseGitPorcelain } from "./workspace-git";

describe("parseGitPorcelain", () => {
  it("maps porcelain rows into readable change records", () => {
    expect(
      parseGitPorcelain(" M web/src/app.tsx\nA  docs/spec.md\n?? scratch.txt"),
    ).toEqual([
      {
        code: " M",
        displayPath: "web/src/app.tsx",
        id: "0- M-web/src/app.tsx",
        path: "web/src/app.tsx",
        raw: " M web/src/app.tsx",
        status: "Modified",
      },
      {
        code: "A ",
        displayPath: "docs/spec.md",
        id: "1-A -docs/spec.md",
        path: "docs/spec.md",
        raw: "A  docs/spec.md",
        status: "Added",
      },
      {
        code: "??",
        displayPath: "scratch.txt",
        id: "2-??-scratch.txt",
        path: "scratch.txt",
        raw: "?? scratch.txt",
        status: "Untracked",
      },
    ]);
  });

  it("uses the destination path for renamed files", () => {
    expect(parseGitPorcelain("R  old-name.ts -> new-name.ts")[0]).toMatchObject(
      {
        displayPath: "old-name.ts -> new-name.ts",
        path: "new-name.ts",
        status: "Renamed",
      },
    );
  });
});
