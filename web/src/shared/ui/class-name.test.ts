import { describe, expect, it } from "vitest";

import { cn } from "./class-name";

describe("cn", () => {
  it("joins truthy class names", () => {
    expect(cn("base", false, "active", undefined, null)).toBe("base active");
  });

  it("merges conflicting Tailwind classes", () => {
    expect(cn("px-2", "px-4")).toBe("px-4");
  });
});
