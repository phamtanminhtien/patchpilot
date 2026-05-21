import * as RadixHoverCard from "@radix-ui/react-hover-card";
import type { ReactNode } from "react";

import { cn } from "./class-name";

interface HoverCardProps {
  children: ReactNode;
  className?: string;
  content: ReactNode;
  contentClassName?: string;
  openDelay?: number;
  side?: RadixHoverCard.HoverCardContentProps["side"];
}

export function HoverCard({
  children,
  className,
  content,
  contentClassName,
  openDelay = 400,
  side = "right",
}: HoverCardProps) {
  return (
    <RadixHoverCard.Root closeDelay={120} openDelay={openDelay}>
      <RadixHoverCard.Trigger asChild>
        <div className={className} role="none">
          {children}
        </div>
      </RadixHoverCard.Trigger>
      <RadixHoverCard.Portal>
        <RadixHoverCard.Content
          align="start"
          avoidCollisions
          className={cn(
            "bg-panel border-line z-50 rounded-md border p-2 text-xs shadow-lg",
            contentClassName,
          )}
          collisionPadding={8}
          side={side}
          sideOffset={8}
        >
          {content}
        </RadixHoverCard.Content>
      </RadixHoverCard.Portal>
    </RadixHoverCard.Root>
  );
}
