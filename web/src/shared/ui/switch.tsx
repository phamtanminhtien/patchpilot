import * as RadixSwitch from "@radix-ui/react-switch";
import type { ComponentPropsWithoutRef, ElementRef } from "react";
import { forwardRef } from "react";

import { cn } from "./class-name";

export const Switch = forwardRef<
  ElementRef<typeof RadixSwitch.Root>,
  ComponentPropsWithoutRef<typeof RadixSwitch.Root>
>(function Switch({ className, ...props }, ref) {
  return (
    <RadixSwitch.Root
      className={cn(
        "bg-hover data-[state=checked]:bg-accent relative inline-flex h-5.5 w-9 shrink-0 cursor-pointer rounded-full p-0.5 transition disabled:cursor-not-allowed disabled:opacity-55",
        className,
      )}
      ref={ref}
      {...props}
    >
      <RadixSwitch.Thumb className="bg-panel block size-4.5 rounded-full transition-transform data-[state=checked]:translate-x-3.5" />
    </RadixSwitch.Root>
  );
});
