import * as RadixAlertDialog from "@radix-ui/react-alert-dialog";
import type { ComponentPropsWithoutRef, ElementRef } from "react";
import { forwardRef } from "react";

import { Button } from "./button";
import { cn } from "./class-name";

export const AlertDialogRoot = RadixAlertDialog.Root;
export const AlertDialogTrigger = RadixAlertDialog.Trigger;

export const AlertDialogPortal = RadixAlertDialog.Portal;

export const AlertDialogOverlay = forwardRef<
  ElementRef<typeof RadixAlertDialog.Overlay>,
  ComponentPropsWithoutRef<typeof RadixAlertDialog.Overlay>
>(function AlertDialogOverlay({ className, ...props }, ref) {
  return (
    <RadixAlertDialog.Overlay
      className={cn(
        "bg-canvas/70 fixed inset-0 z-50 backdrop-blur-sm data-[state=closed]:hidden",
        className,
      )}
      ref={ref}
      {...props}
    />
  );
});

export const AlertDialogContent = forwardRef<
  ElementRef<typeof RadixAlertDialog.Content>,
  ComponentPropsWithoutRef<typeof RadixAlertDialog.Content>
>(function AlertDialogContent({ className, ...props }, ref) {
  return (
    <AlertDialogPortal>
      <AlertDialogOverlay />
      <RadixAlertDialog.Content
        className={cn(
          "bg-panel text-ink fixed top-1/2 left-1/2 z-50 grid max-h-[min(32rem,calc(100vh-2rem))] w-[calc(100vw-2rem)] max-w-md -translate-x-1/2 -translate-y-1/2 gap-4 overflow-auto rounded-md p-4 shadow-md data-[state=closed]:hidden",
          className,
        )}
        ref={ref}
        {...props}
      />
    </AlertDialogPortal>
  );
});

export function AlertDialogHeader({
  className,
  ...props
}: ComponentPropsWithoutRef<"div">) {
  return <div className={cn("grid gap-1", className)} {...props} />;
}

export function AlertDialogFooter({
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

export const AlertDialogTitle = forwardRef<
  ElementRef<typeof RadixAlertDialog.Title>,
  ComponentPropsWithoutRef<typeof RadixAlertDialog.Title>
>(function AlertDialogTitle({ className, ...props }, ref) {
  return (
    <RadixAlertDialog.Title
      className={cn("text-ink text-sm font-semibold", className)}
      ref={ref}
      {...props}
    />
  );
});

export const AlertDialogDescription = forwardRef<
  ElementRef<typeof RadixAlertDialog.Description>,
  ComponentPropsWithoutRef<typeof RadixAlertDialog.Description>
>(function AlertDialogDescription({ className, ...props }, ref) {
  return (
    <RadixAlertDialog.Description
      className={cn("text-muted text-sm leading-6", className)}
      ref={ref}
      {...props}
    />
  );
});

export function AlertDialogCancel({
  className,
  ...props
}: ComponentPropsWithoutRef<typeof RadixAlertDialog.Cancel>) {
  return (
    <RadixAlertDialog.Cancel asChild>
      <Button
        className={className}
        size="small"
        type="button"
        variant="secondary"
        {...props}
      />
    </RadixAlertDialog.Cancel>
  );
}

export function AlertDialogAction({
  className,
  ...props
}: ComponentPropsWithoutRef<typeof RadixAlertDialog.Action>) {
  return (
    <RadixAlertDialog.Action asChild>
      <Button className={className} size="small" type="button" {...props} />
    </RadixAlertDialog.Action>
  );
}
