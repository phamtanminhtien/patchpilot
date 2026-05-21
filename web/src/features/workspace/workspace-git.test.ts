import { describe, expect, it } from "vitest";

import {
  isGitChangeStageable,
  isIgnoredGitChange,
  isStagedGitChange,
  isUnstagedGitChange,
  parseGitPorcelain,
  stagedGitPaths,
  unstagedGitPaths,
  visibleGitChanges,
} from "./git/workspace-git";

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

  it("maps ignored paths", () => {
    expect(parseGitPorcelain("!! dist/")[0]).toMatchObject({
      code: "!!",
      displayPath: "dist/",
      path: "dist/",
      status: "Ignored",
    });
  });

  it("derives visible, staged, and unstaged path sets", () => {
    const changes = parseGitPorcelain(
      "M  staged.txt\n M changed.txt\n?? scratch.md\n!! dist/",
    );

    expect(stagedGitPaths(changes)).toEqual(["staged.txt"]);
    expect(unstagedGitPaths(changes)).toEqual(["changed.txt", "scratch.md"]);
    expect(visibleGitChanges(changes).map((change) => change.path)).toEqual([
      "staged.txt",
      "changed.txt",
      "scratch.md",
    ]);
  });

  it("classifies individual change states", () => {
    const changes = parseGitPorcelain(
      "M  staged.txt\n M changed.txt\n?? scratch.md\n!! dist/",
    );
    const staged = changes[0]!;
    const changed = changes[1]!;
    const untracked = changes[2]!;
    const ignored = changes[3]!;

    expect(isStagedGitChange(staged)).toBe(true);
    expect(isUnstagedGitChange(staged)).toBe(false);
    expect(isStagedGitChange(changed)).toBe(false);
    expect(isUnstagedGitChange(changed)).toBe(true);
    expect(isStagedGitChange(untracked)).toBe(false);
    expect(isUnstagedGitChange(untracked)).toBe(true);
    expect(isIgnoredGitChange(ignored)).toBe(true);
    expect(isGitChangeStageable(ignored)).toBe(false);
  });
});
