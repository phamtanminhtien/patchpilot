import * as RadixDialog from "@radix-ui/react-dialog";
import { X } from "lucide-react";
import type { ComponentPropsWithoutRef, ElementRef } from "react";
import { forwardRef } from "react";

import { Button } from "./button";
import { cn } from "./class-name";

export const DialogRoot = RadixDialog.Root;
export const DialogTrigger = RadixDialog.Trigger;
export const DialogClose = RadixDialog.Close;
export const DialogPortal = RadixDialog.Portal;

export const DialogOverlay = forwardRef<
  ElementRef<typeof RadixDialog.Overlay>,
  ComponentPropsWithoutRef<typeof RadixDialog.Overlay>
>(function DialogOverlay({ className, ...props }, ref) {
  return (
    <RadixDialog.Overlay
      className={cn(
        "bg-canvas/35 fixed inset-0 z-50 backdrop-blur-[2px] data-[state=closed]:hidden",
        className,
      )}
      ref={ref}
      {...props}
    />
  );
});

export const DialogContent = forwardRef<
  ElementRef<typeof RadixDialog.Content>,
  ComponentPropsWithoutRef<typeof RadixDialog.Content> & {
    showClose?: boolean;
  }
>(function DialogContent(
  { children, className, showClose = true, ...props },
  ref,
) {
  return (
    <DialogPortal>
      <DialogOverlay />
      <RadixDialog.Content
        className={cn(
          "bg-panel text-ink fixed top-1/2 left-1/2 z-50 grid max-h-[min(36rem,calc(100vh-2rem))] w-[calc(100vw-2rem)] max-w-lg -translate-x-1/2 -translate-y-1/2 gap-4 overflow-auto rounded-xl p-4 data-[state=closed]:hidden",
          className,
        )}
        ref={ref}
        {...props}
      >
        {children}
        {showClose ? (
          <RadixDialog.Close asChild>
            <Button
              aria-label="Close dialog"
              className="absolute top-3 right-3"
              icon={<X />}
              size="icon"
              type="button"
              variant="action"
            />
          </RadixDialog.Close>
        ) : null}
      </RadixDialog.Content>
    </DialogPortal>
  );
});

export function DialogHeader({
  className,
  ...props
}: ComponentPropsWithoutRef<"div">) {
  return <div className={cn("grid gap-1 pr-8", className)} {...props} />;
}

export function DialogFooter({
  className,
  ...props
}: ComponentPropsWithoutRef<"div">) {
  return (
    <div
      className={cn(
        "flex flex-col-reverse gap-2 sm:flex-row sm:justify-end",
        className,
      )}
      {...props}
    />
  );
}

export const DialogTitle = forwardRef<
  ElementRef<typeof RadixDialog.Title>,
  ComponentPropsWithoutRef<typeof RadixDialog.Title>
>(function DialogTitle({ className, ...props }, ref) {
  return (
    <RadixDialog.Title
      className={cn("text-ink text-sm font-semibold", className)}
      ref={ref}
      {...props}
    />
  );
});

export const DialogDescription = forwardRef<
  ElementRef<typeof RadixDialog.Description>,
  ComponentPropsWithoutRef<typeof RadixDialog.Description>
>(function DialogDescription({ className, ...props }, ref) {
  return (
    <RadixDialog.Description
      className={cn("text-muted text-sm leading-6", className)}
      ref={ref}
      {...props}
    />
  );
});
