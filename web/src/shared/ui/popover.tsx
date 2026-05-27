import * as RadixPopover from "@radix-ui/react-popover";
import type { ComponentPropsWithoutRef, ElementRef } from "react";
import { forwardRef } from "react";

import { cn } from "./class-name";

export const PopoverRoot = RadixPopover.Root;
export const PopoverTrigger = RadixPopover.Trigger;
export const PopoverClose = RadixPopover.Close;
export const PopoverAnchor = RadixPopover.Anchor;
export const PopoverPortal = RadixPopover.Portal;

export const PopoverContent = forwardRef<
  ElementRef<typeof RadixPopover.Content>,
  ComponentPropsWithoutRef<typeof RadixPopover.Content>
>(function PopoverContent(
  { align = "end", className, collisionPadding = 8, sideOffset = 6, ...props },
  ref,
) {
  return (
    <PopoverPortal>
      <RadixPopover.Content
        align={align}
        className={cn(
          "bg-panel text-ink z-50 grid min-w-48 gap-1 rounded-xl p-1 data-[state=closed]:hidden",
          className,
        )}
        collisionPadding={collisionPadding}
        ref={ref}
        sideOffset={sideOffset}
        {...props}
      />
    </PopoverPortal>
  );
});
