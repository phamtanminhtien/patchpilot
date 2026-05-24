import * as RadixCheckbox from "@radix-ui/react-checkbox";
import { Check, Minus } from "lucide-react";
import type { ComponentPropsWithoutRef, ElementRef } from "react";
import { forwardRef } from "react";

import { cn } from "./class-name";

export const Checkbox = forwardRef<
  ElementRef<typeof RadixCheckbox.Root>,
  ComponentPropsWithoutRef<typeof RadixCheckbox.Root>
>(function Checkbox({ className, ...props }, ref) {
  return (
    <RadixCheckbox.Root
      className={cn(
        "bg-panel text-accent data-[state=checked]:bg-accent data-[state=checked]:text-accent-ink data-[state=indeterminate]:bg-accent data-[state=indeterminate]:text-accent-ink grid size-5 shrink-0 cursor-pointer place-items-center rounded-sm shadow-sm transition disabled:cursor-not-allowed disabled:opacity-55",
        className,
      )}
      ref={ref}
      {...props}
    >
      <RadixCheckbox.Indicator className="grid place-items-center">
        {props.checked === "indeterminate" ? (
          <Minus aria-hidden="true" className="size-3.5" />
        ) : (
          <Check aria-hidden="true" className="size-3.5" />
        )}
      </RadixCheckbox.Indicator>
    </RadixCheckbox.Root>
  );
});
