import type { InputHTMLAttributes } from "react";

import { classNames } from "./class-name";

interface TextFieldProps extends InputHTMLAttributes<HTMLInputElement> {
  label: string;
}

export function TextField({ className, id, label, ...props }: TextFieldProps) {
  const inputId =
    id ?? props.name ?? label.toLowerCase().replaceAll(/\s+/g, "-");

  return (
    <label
      className="text-ink grid gap-2 text-sm font-medium"
      htmlFor={inputId}
    >
      {label}
      <input
        className={classNames(
          "border-line bg-panel text-ink placeholder:text-muted focus:border-accent focus:ring-accent/20 min-h-11 w-full rounded-md border px-3 py-2 text-base transition outline-none focus:ring-2",
          className,
        )}
        id={inputId}
        {...props}
      />
    </label>
  );
}
