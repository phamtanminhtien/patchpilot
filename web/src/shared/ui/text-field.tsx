import type { InputHTMLAttributes } from "react";

import { classNames } from "./class-name";
import { createVariant, type VariantPropsOf } from "./variant";

const inputVariant = createVariant({
  base: "w-full rounded-md border text-base transition placeholder:text-muted",
  variants: {
    size: {
      default: "min-h-11 px-3 py-2",
    },
    state: {
      default: "border-line bg-panel text-ink focus:border-accent",
      invalid: "border-warning bg-panel text-ink focus:border-warning",
    },
  },
  defaultVariants: {
    size: "default",
    state: "default",
  },
});

type TextFieldVariantProps = VariantPropsOf<typeof inputVariant>;

interface TextFieldProps extends InputHTMLAttributes<HTMLInputElement> {
  label: string;
  state?: TextFieldVariantProps["state"];
}

export function TextField({
  className,
  id,
  label,
  state,
  ...props
}: TextFieldProps) {
  const inputId =
    id ?? props.name ?? label.toLowerCase().replaceAll(/\s+/g, "-");

  return (
    <label
      className="text-ink grid gap-2 text-sm font-medium"
      htmlFor={inputId}
    >
      {label}
      <input
        className={classNames(inputVariant({ state }), className)}
        id={inputId}
        {...props}
      />
    </label>
  );
}
