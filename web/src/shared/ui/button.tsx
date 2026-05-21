import { Slot } from "@radix-ui/react-slot";
import type { ButtonHTMLAttributes, ReactNode } from "react";

import { cn } from "./class-name";
import { createVariant, type VariantPropsOf } from "./variant";

const buttonVariant = createVariant({
  base: "inline-flex items-center justify-center gap-2 rounded-md font-medium transition disabled:cursor-not-allowed disabled:opacity-55",
  variants: {
    size: {
      icon: "size-6 p-0 text-xs",
      small: "min-h-9 px-2.5 py-1.5 text-xs",
      compact: "min-h-10 px-3 py-2 text-sm",
      default: "min-h-11 px-4 py-2 text-base",
    },
    variant: {
      action: "bg-transparent text-muted hover:bg-hover hover:text-ink",
      ghost: "bg-transparent text-ink hover:bg-hover",
      primary: "bg-accent text-accent-ink shadow-sm hover:bg-accent-hover",
      secondary: "bg-panel text-ink shadow-sm hover:bg-hover",
    },
    width: {
      auto: "",
      full: "w-full",
    },
  },
  defaultVariants: {
    size: "default",
    variant: "primary",
    width: "auto",
  },
});

type ButtonVariantProps = VariantPropsOf<typeof buttonVariant>;

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  asChild?: boolean;
  icon?: ReactNode;
  size?: ButtonVariantProps["size"];
  variant?: ButtonVariantProps["variant"];
  width?: ButtonVariantProps["width"];
}

export function Button({
  asChild = false,
  children,
  className,
  icon,
  size,
  variant = "primary",
  width,
  ...props
}: ButtonProps) {
  const Component = asChild ? Slot : "button";
  const content = asChild ? (
    children
  ) : (
    <>
      {icon ? (
        <span
          aria-hidden="true"
          className={cn(
            "grid shrink-0 cursor-pointer place-items-center",
            size === "icon" ? "size-4" : "size-5",
          )}
        >
          {icon}
        </span>
      ) : null}
      {children}
    </>
  );

  return (
    <Component
      className={cn(buttonVariant({ size, variant, width }), className)}
      {...props}
    >
      {content}
    </Component>
  );
}
