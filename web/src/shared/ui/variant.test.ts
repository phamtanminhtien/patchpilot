import { describe, expect, it } from "vitest";

import { createVariant } from "./variant";

describe("createVariant", () => {
  it("returns the class name for a variant", () => {
    const tone = createVariant({
      neutral: "text-ink",
      warning: "text-warning",
    });

    expect(tone("warning")).toBe("text-warning");
  });
});
