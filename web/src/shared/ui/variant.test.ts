import { describe, expect, it } from "vitest";

import { createVariant } from "./variant";

describe("createVariant", () => {
  it("returns the class name for a variant", () => {
    const tone = createVariant({
      variants: {
        tone: {
          neutral: "text-ink",
          warning: "text-warning",
        },
      },
    });

    expect(tone({ tone: "warning" })).toBe("text-warning");
  });

  it("combines base, default variants, selected variants, and caller classes", () => {
    const control = createVariant({
      base: "inline-flex rounded-md",
      variants: {
        size: {
          sm: "min-h-9 px-3",
          md: "min-h-11 px-4",
        },
        tone: {
          primary: "bg-accent text-accent-ink",
          secondary: "border border-line text-ink",
        },
      },
      defaultVariants: {
        size: "md",
        tone: "primary",
      },
    });

    expect(control({ tone: "secondary", className: "w-full" })).toBe(
      "inline-flex rounded-md min-h-11 px-4 border border-line text-ink w-full",
    );
  });

  it("adds compound variant classes when all matching selections are active", () => {
    const control = createVariant({
      base: "inline-flex",
      variants: {
        density: {
          compact: "gap-1",
          roomy: "gap-3",
        },
        tone: {
          primary: "bg-accent",
          secondary: "bg-panel",
        },
      },
      compoundVariants: [
        {
          className: "shadow-sm",
          density: "roomy",
          tone: "primary",
        },
      ],
      defaultVariants: {
        density: "compact",
        tone: "primary",
      },
    });

    expect(control({ density: "roomy" })).toBe(
      "inline-flex gap-3 bg-accent shadow-sm",
    );
  });

  it("lets selections clear default variants with null", () => {
    const badge = createVariant({
      base: "inline-flex",
      variants: {
        tone: {
          accent: "text-accent",
          muted: "text-muted",
        },
      },
      defaultVariants: {
        tone: "muted",
      },
    });

    expect(badge({ tone: null })).toBe("inline-flex");
  });

  it("keeps default variants when selections pass undefined", () => {
    const control = createVariant({
      base: "inline-flex",
      variants: {
        size: {
          default: "min-h-11 px-4",
          sm: "min-h-9 px-3",
        },
        tone: {
          primary: "bg-accent",
          secondary: "bg-panel",
        },
      },
      defaultVariants: {
        size: "default",
        tone: "primary",
      },
    });

    expect(control({ size: undefined, tone: "secondary" })).toBe(
      "inline-flex min-h-11 px-4 bg-panel",
    );
  });
});
