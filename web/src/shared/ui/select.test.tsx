import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { Select } from "./select";

describe("Select", () => {
  it("renders the selected value and emits option changes", async () => {
    const user = userEvent.setup({ pointerEventsCheck: 0 });
    const onValueChange = vi.fn();

    render(
      <Select
        label="Model"
        onValueChange={onValueChange}
        options={[{ value: "gpt-5.5" }, { value: "gpt-5.4-mini" }]}
        value="gpt-5.5"
      />,
    );

    const trigger = screen.getByRole("combobox", { name: "Model" });
    expect(trigger).toHaveTextContent("gpt-5.5");

    await user.click(trigger);
    await user.click(screen.getByRole("option", { name: "gpt-5.4-mini" }));

    expect(onValueChange).toHaveBeenCalledWith("gpt-5.4-mini");
  });

  it("dismisses stacked menus after opening another select and clicking outside", async () => {
    const user = userEvent.setup({ pointerEventsCheck: 0 });

    render(
      <div>
        <Select
          label="Model"
          onValueChange={vi.fn()}
          options={[{ value: "gpt-5.5" }, { value: "gpt-5.4-mini" }]}
          value="gpt-5.5"
        />
        <Select
          label="Reasoning"
          onValueChange={vi.fn()}
          options={[{ value: "medium" }, { value: "high" }]}
          value="medium"
        />
        <button type="button">Outside</button>
      </div>,
    );

    await user.click(screen.getByRole("combobox", { name: "Model" }));
    expect(screen.getByRole("option", { name: "gpt-5.4-mini" })).toBeVisible();

    await user.click(
      screen.getByRole("combobox", { hidden: true, name: "Reasoning" }),
    );
    expect(screen.getByRole("option", { name: "high" })).toBeVisible();

    await user.click(
      screen.getByRole("button", { hidden: true, name: "Outside" }),
    );

    await waitFor(() => {
      expect(screen.queryByRole("listbox")).not.toBeInTheDocument();
    });
  });
});
