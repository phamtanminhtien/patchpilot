import type { InputHTMLAttributes } from "react";

import { cn } from "./class-name";
import { createVariant, type VariantPropsOf } from "./variant";

const inputVariant = createVariant({
  base: "w-full rounded-md shadow-sm transition placeholder:text-muted",
  variants: {
    size: {
      small: "min-h-9 px-2.5 py-1.5 text-xs",
      compact: "min-h-10 px-3 py-2 text-sm",
      default: "min-h-11 px-3 py-2 text-base",
    },
    state: {
      default: "bg-panel text-ink focus-visible:outline-accent",
      invalid: "bg-panel text-ink focus-visible:outline-warning",
    },
  },
  defaultVariants: {
    size: "default",
    state: "default",
  },
});

type TextFieldVariantProps = VariantPropsOf<typeof inputVariant>;

const labelVariant = createVariant({
  base: "text-ink grid font-medium",
  variants: {
    size: {
      small: "gap-1 text-xs",
      compact: "gap-2 text-sm",
      default: "gap-2 text-sm",
    },
  },
  defaultVariants: {
    size: "default",
  },
});

interface TextFieldProps extends Omit<
  InputHTMLAttributes<HTMLInputElement>,
  "size"
> {
  label: string;
  size?: TextFieldVariantProps["size"];
  state?: TextFieldVariantProps["state"];
}

export function TextField({
  className,
  id,
  label,
  size,
  state,
  ...props
}: TextFieldProps) {
  const inputId =
    id ?? props.name ?? label.toLowerCase().replaceAll(/\s+/g, "-");

  return (
    <label className={labelVariant({ size })} htmlFor={inputId}>
      {label}
      <input
        className={cn(inputVariant({ size, state }), className)}
        id={inputId}
        {...props}
      />
    </label>
  );
}
