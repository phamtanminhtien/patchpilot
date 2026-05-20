import { describe, expect, it } from "vitest";

import { applyThemePreference, parseThemePreference } from "./theme";

describe("theme preference", () => {
  it("parses unknown stored values as system", () => {
    expect(parseThemePreference("dark")).toBe("dark");
    expect(parseThemePreference("light")).toBe("light");
    expect(parseThemePreference("system")).toBe("system");
    expect(parseThemePreference("unexpected")).toBe("system");
    expect(parseThemePreference(null)).toBe("system");
  });

  it("only applies a root data-theme attribute for explicit overrides", () => {
    const root = document.createElement("html");

    applyThemePreference(root, "dark");
    expect(root).toHaveAttribute("data-theme", "dark");

    applyThemePreference(root, "light");
    expect(root).toHaveAttribute("data-theme", "light");

    applyThemePreference(root, "system");
    expect(root).not.toHaveAttribute("data-theme");
  });
});
