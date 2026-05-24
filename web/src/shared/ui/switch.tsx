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
        "bg-hover data-[state=checked]:bg-accent relative inline-flex h-6 w-10 shrink-0 cursor-pointer rounded-full p-0.5 shadow-sm transition disabled:cursor-not-allowed disabled:opacity-55",
        className,
      )}
      ref={ref}
      {...props}
    >
      <RadixSwitch.Thumb className="bg-panel block size-5 rounded-full shadow-sm transition-transform data-[state=checked]:translate-x-4" />
    </RadixSwitch.Root>
  );
});
