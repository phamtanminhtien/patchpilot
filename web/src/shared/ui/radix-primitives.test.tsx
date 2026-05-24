import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { describe, expect, it, vi } from "vitest";

import {
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogRoot,
  AlertDialogTitle,
  AlertDialogTrigger,
  Button,
  Checkbox,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogRoot,
  DialogTitle,
  DialogTrigger,
  PopoverContent,
  PopoverRoot,
  PopoverTrigger,
  Switch,
} from ".";

describe("Radix-backed shared primitives", () => {
  it("confirms and cancels alert dialogs", async () => {
    const user = userEvent.setup({ pointerEventsCheck: 0 });
    const onConfirm = vi.fn();

    render(
      <AlertDialogRoot>
        <AlertDialogTrigger asChild>
          <Button>Discard</Button>
        </AlertDialogTrigger>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Discard paths?</AlertDialogTitle>
            <AlertDialogDescription>
              This will discard one selected path.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction onClick={onConfirm}>Discard</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialogRoot>,
    );

    await user.click(screen.getByRole("button", { name: "Discard" }));
    expect(screen.getByRole("alertdialog")).toHaveAccessibleName(
      "Discard paths?",
    );

    await user.click(screen.getByRole("button", { name: "Cancel" }));
    await waitFor(() => {
      expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
    });

    await user.click(screen.getByRole("button", { name: "Discard" }));
    await user.click(screen.getByRole("button", { name: "Discard" }));

    expect(onConfirm).toHaveBeenCalledOnce();
  });

  it("opens and closes dialogs", async () => {
    const user = userEvent.setup({ pointerEventsCheck: 0 });

    render(
      <DialogRoot>
        <DialogTrigger asChild>
          <Button>Review commit</Button>
        </DialogTrigger>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Review commit</DialogTitle>
            <DialogDescription>Review exact staged paths.</DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button>Commit</Button>
          </DialogFooter>
        </DialogContent>
      </DialogRoot>,
    );

    await user.click(screen.getByRole("button", { name: "Review commit" }));
    expect(screen.getByRole("dialog")).toHaveAccessibleName("Review commit");

    await user.click(screen.getByRole("button", { name: "Close dialog" }));
    await waitFor(() => {
      expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    });
  });

  it("opens popovers and runs actions", async () => {
    const user = userEvent.setup({ pointerEventsCheck: 0 });
    const onAction = vi.fn();

    render(
      <PopoverRoot>
        <PopoverTrigger asChild>
          <Button>More actions</Button>
        </PopoverTrigger>
        <PopoverContent>
          <Button onClick={onAction} size="small" variant="ghost">
            Stage selected
          </Button>
        </PopoverContent>
      </PopoverRoot>,
    );

    await user.click(screen.getByRole("button", { name: "More actions" }));
    await user.click(screen.getByRole("button", { name: "Stage selected" }));

    expect(onAction).toHaveBeenCalledOnce();
  });

  it("changes switch checked state and preserves disabled state", async () => {
    const user = userEvent.setup({ pointerEventsCheck: 0 });

    function SwitchExample() {
      const [checked, setChecked] = useState(false);

      return (
        <>
          <Switch
            aria-label="Selected only"
            checked={checked}
            onCheckedChange={setChecked}
          />
          <Switch aria-label="Disabled selected only" disabled />
        </>
      );
    }

    render(<SwitchExample />);

    const selectedOnly = screen.getByRole("switch", { name: "Selected only" });
    expect(selectedOnly).not.toBeChecked();

    await user.click(selectedOnly);

    expect(selectedOnly).toBeChecked();
    expect(
      screen.getByRole("switch", { name: "Disabled selected only" }),
    ).toBeDisabled();
  });

  it("changes checkbox checked state and supports indeterminate/disabled", async () => {
    const user = userEvent.setup({ pointerEventsCheck: 0 });

    function CheckboxExample() {
      const [checked, setChecked] = useState(false);

      return (
        <>
          <Checkbox
            aria-label="Select path"
            checked={checked}
            onCheckedChange={(value) => setChecked(value === true)}
          />
          <Checkbox aria-label="Select some paths" checked="indeterminate" />
          <Checkbox aria-label="Disabled path" disabled />
        </>
      );
    }

    render(<CheckboxExample />);

    const checkbox = screen.getByRole("checkbox", { name: "Select path" });
    expect(checkbox).not.toBeChecked();

    await user.click(checkbox);

    expect(checkbox).toBeChecked();
    expect(
      screen.getByRole("checkbox", { name: "Select some paths" }),
    ).toBePartiallyChecked();
    expect(
      screen.getByRole("checkbox", { name: "Disabled path" }),
    ).toBeDisabled();
  });
});
