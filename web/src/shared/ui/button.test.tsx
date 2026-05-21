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
      "text-base",
    );
  });

  it("keeps text sizing attached to size variants", () => {
    render(
      <>
        <Button size="small">Cancel</Button>
        <Button size="compact">Run command</Button>
      </>,
    );

    expect(screen.getByRole("button", { name: "Cancel" })).toHaveClass(
      "min-h-9",
      "px-2.5",
      "py-1.5",
      "text-xs",
    );
    expect(screen.getByRole("button", { name: "Run command" })).toHaveClass(
      "min-h-10",
      "text-sm",
    );
  });

  it("keeps leading icons from overflowing into the label", () => {
    render(<Button icon={<svg data-testid="button-icon" />}>Open repo</Button>);

    const iconFrame = screen.getByTestId("button-icon").parentElement;

    expect(iconFrame).toHaveAttribute("aria-hidden", "true");
    expect(iconFrame).toHaveClass("size-5", "shrink-0", "cursor-pointer");
  });

  it("supports compact icon action buttons", () => {
    render(
      <Button
        aria-label="Stage change"
        icon={<svg />}
        size="icon"
        variant="action"
      />,
    );

    expect(screen.getByRole("button", { name: "Stage change" })).toHaveClass(
      "size-6",
      "p-0",
      "text-muted",
      "hover:bg-hover",
    );
  });

  it("keeps secondary buttons borderless and elevated", () => {
    render(<Button variant="secondary">Open workspace</Button>);

    expect(screen.getByRole("button", { name: "Open workspace" })).toHaveClass(
      "bg-panel",
      "shadow-sm",
    );
    expect(
      screen.getByRole("button", { name: "Open workspace" }),
    ).not.toHaveClass("border");
  });
});
