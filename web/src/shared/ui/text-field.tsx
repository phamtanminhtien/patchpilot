import type { InputHTMLAttributes } from "react";

import { cn } from "./class-name";
import { createVariant, type VariantPropsOf } from "./variant";

const inputVariant = createVariant({
  base: "w-full rounded-md shadow-sm transition placeholder:text-muted focus-visible:!outline-none",
  variants: {
    size: {
      small: "min-h-9 px-2.5 py-1.5 text-xs",
      compact: "min-h-10 px-3 py-2 text-sm",
      default: "min-h-11 px-3 py-2 text-base",
    },
    state: {
      default:
        "bg-panel text-ink focus-visible:shadow-[inset_0_0_0_1px_var(--pp-color-focus)]",
      invalid:
        "bg-panel text-ink focus-visible:shadow-[inset_0_0_0_1px_var(--pp-color-warning)]",
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
  labelClassName?: string;
  labelHidden?: boolean;
  size?: TextFieldVariantProps["size"];
  state?: TextFieldVariantProps["state"];
}

export function TextField({
  className,
  id,
  label,
  labelClassName,
  labelHidden = false,
  size,
  state,
  ...props
}: TextFieldProps) {
  const inputId =
    id ?? props.name ?? label.toLowerCase().replaceAll(/\s+/g, "-");

  return (
    <label
      className={cn(labelVariant({ size }), labelClassName)}
      htmlFor={inputId}
    >
      <span className={labelHidden ? "sr-only" : undefined}>{label}</span>
      <input
        className={cn(inputVariant({ size, state }), className)}
        id={inputId}
        {...props}
      />
    </label>
  );
}
