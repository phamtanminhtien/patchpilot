import type { ElementType, HTMLAttributes } from "react";

import { classNames } from "./class-name";
import { createVariant, type VariantPropsOf } from "./variant";

const surfaceVariant = createVariant({
  base: "bg-panel shadow-sm",
  variants: {
    layout: {
      block: "",
      grid: "grid",
    },
    padding: {
      compact: "p-2",
      default: "p-3",
      none: "p-0",
    },
    radius: {
      default: "rounded-lg",
      md: "rounded-md",
    },
  },
  defaultVariants: {
    layout: "block",
    padding: "default",
    radius: "default",
  },
});

type SurfaceVariantProps = VariantPropsOf<typeof surfaceVariant>;

interface SurfaceProps extends HTMLAttributes<HTMLElement> {
  as?: Extract<ElementType, "aside" | "div" | "section">;
  layout?: SurfaceVariantProps["layout"];
  padding?: SurfaceVariantProps["padding"];
  radius?: SurfaceVariantProps["radius"];
}

export function Surface({
  as: Component = "div",
  className,
  layout,
  padding,
  radius,
  ...props
}: SurfaceProps) {
  return (
    <Component
      className={classNames(
        surfaceVariant({ layout, padding, radius }),
        className,
      )}
      {...props}
    />
  );
}
