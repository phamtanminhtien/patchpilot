import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { Button } from "./button";

describe("Button", () => {
  it("keeps the default size classes when optional variant props are omitted", () => {
    render(<Button>Open repo</Button>);

    expect(screen.getByRole("button", { name: "Open repo" })).toHaveClass(
      "min-h-11",
      "px-4",
      "py-2",
    );
  });

  it("keeps leading icons from overflowing into the label", () => {
    render(<Button icon={<svg data-testid="button-icon" />}>Open repo</Button>);

    const iconFrame = screen.getByTestId("button-icon").parentElement;

    expect(iconFrame).toHaveAttribute("aria-hidden", "true");
    expect(iconFrame).toHaveClass("size-5", "shrink-0", "[&>svg]:size-5");
  });
});
