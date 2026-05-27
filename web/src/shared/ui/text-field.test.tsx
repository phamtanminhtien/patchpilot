import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { TextField } from "./text-field";

describe("TextField", () => {
  it("keeps default text sizing attached to the size variant", () => {
    render(<TextField label="Workspace root" />);

    const input = screen.getByLabelText("Workspace root");

    expect(input).toHaveClass("min-h-10", "px-3", "py-2", "text-sm");
    expect(input).not.toHaveClass("border");
    expect(input).not.toHaveClass("shadow-sm");
    expect(input.closest("label")).toHaveClass("gap-2", "text-sm");
  });

  it("supports smaller text field size variants", () => {
    render(
      <>
        <TextField label="Patch" size="small" />
        <TextField label="Command" size="compact" />
      </>,
    );

    expect(screen.getByLabelText("Patch")).toHaveClass(
      "min-h-8",
      "px-2.5",
      "py-1",
      "text-xs",
    );
    expect(screen.getByLabelText("Patch").closest("label")).toHaveClass(
      "gap-1",
      "text-xs",
    );
    expect(screen.getByLabelText("Command")).toHaveClass(
      "min-h-9",
      "px-3",
      "py-1.5",
      "text-sm",
    );
  });
});
