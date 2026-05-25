import { describe, expect, it } from "vitest";

import { humanizeSkillName } from "./skills";

describe("humanizeSkillName", () => {
  it("formats slug skill names for display", () => {
    expect(humanizeSkillName("incremental-implementation")).toBe(
      "Incremental Implementation",
    );
  });

  it("keeps common technical abbreviations readable", () => {
    expect(humanizeSkillName("github-app-commit-pr")).toBe(
      "GitHub App Commit PR",
    );
  });
});
