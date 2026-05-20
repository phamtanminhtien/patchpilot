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
});
