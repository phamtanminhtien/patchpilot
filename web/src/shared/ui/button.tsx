import { Slot } from "@radix-ui/react-slot";
import type { ButtonHTMLAttributes, ReactNode } from "react";

import { classNames } from "./class-name";
import { createVariant } from "./variant";

const buttonVariant = createVariant({
  ghost: "bg-transparent text-ink hover:bg-black/5",
  primary: "bg-accent text-accent-ink shadow-sm hover:bg-accent/90",
  secondary: "border border-line bg-panel text-ink hover:bg-black/5",
});

type ButtonVariant = Parameters<typeof buttonVariant>[0];

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  asChild?: boolean;
  icon?: ReactNode;
  variant?: ButtonVariant;
}

export function Button({
  asChild = false,
  children,
  className,
  icon,
  variant = "primary",
  ...props
}: ButtonProps) {
  const Component = asChild ? Slot : "button";
  const content = asChild ? (
    children
  ) : (
    <>
      {icon ? (
        <span className="grid size-4 place-items-center">{icon}</span>
      ) : null}
      {children}
    </>
  );

  return (
    <Component
      className={classNames(
        "inline-flex min-h-11 items-center justify-center gap-2 rounded-md px-4 py-2 text-sm font-medium transition disabled:cursor-not-allowed disabled:opacity-55",
        buttonVariant(variant),
        className,
      )}
      {...props}
    >
      {content}
    </Component>
  );
}
