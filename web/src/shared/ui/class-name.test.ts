import { describe, expect, it } from "vitest";

import { classNames } from "./class-name";

describe("classNames", () => {
  it("joins truthy class names", () => {
    expect(classNames("base", false, "active", undefined, null)).toBe(
      "base active",
    );
  });
});
