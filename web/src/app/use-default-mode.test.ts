import { describe, expect, it } from "vitest";

import { createDefaultMode } from "./use-default-mode";

describe("createDefaultMode", () => {
  it("uses Workspace Mode on desktop viewports", () => {
    const mode = createDefaultMode(() => ({ matches: true }));

    expect(mode).toBe("workspace");
  });

  it("uses Vibe Mode when media matching is unavailable", () => {
    const mode = createDefaultMode(undefined);

    expect(mode).toBe("vibe");
  });
});
