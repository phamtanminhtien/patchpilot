import * as RadixSelect from "@radix-ui/react-select";
import { Check, ChevronDown } from "lucide-react";
import { type PointerEvent, useId } from "react";

import { cn } from "./class-name";
import { createVariant, type VariantPropsOf } from "./variant";

export interface SelectOption {
  label?: string;
  value: string;
}

const triggerVariant = createVariant({
  base: "bg-panel text-ink hover:bg-hover data-[placeholder]:text-muted focus-visible:outline-focus focus-visible:outline-offset-focus focus-visible:outline-width-focus inline-flex min-w-0 cursor-pointer items-center justify-between gap-2 rounded-xl font-medium transition disabled:cursor-not-allowed disabled:opacity-55",
  variants: {
    size: {
      tiny: "min-h-7 px-2 text-xs [&_[data-slot=select-icon]>svg]:size-3",
      small: "min-h-8 px-2.5 text-xs [&_[data-slot=select-icon]>svg]:size-3.5",
      compact: "min-h-9 px-3 text-sm [&_[data-slot=select-icon]>svg]:size-4",
    },
  },
  defaultVariants: {
    size: "compact",
  },
});

const itemVariant = createVariant({
  base: "text-ink data-[highlighted]:bg-hover data-[state=checked]:bg-accent-soft relative flex cursor-pointer items-center rounded-xl transition outline-none select-none data-[disabled]:pointer-events-none data-[disabled]:opacity-55",
  variants: {
    size: {
      tiny: "min-h-7 py-1 pr-2 pl-7 text-xs",
      small: "min-h-8 py-1.5 pr-2.5 pl-7 text-xs",
      compact: "min-h-9 py-2 pr-3 pl-8 text-sm",
    },
  },
  defaultVariants: {
    size: "compact",
  },
});

type SelectVariantProps = VariantPropsOf<typeof triggerVariant>;

function dismissOpenSelectBeforeOpening(
  event: PointerEvent<HTMLButtonElement>,
) {
  if (event.currentTarget.getAttribute("aria-expanded") === "true") {
    return;
  }

  if (!document.querySelector('[role="listbox"][data-state="open"]')) {
    return;
  }

  document.dispatchEvent(
    new KeyboardEvent("keydown", {
      bubbles: true,
      key: "Escape",
    }),
  );
}

interface SelectProps {
  className?: string;
  contentClassName?: string;
  disabled?: boolean;
  label: string;
  onValueChange: (value: string) => void;
  options: SelectOption[];
  size?: SelectVariantProps["size"];
  triggerClassName?: string;
  value: string;
}

export function Select({
  className,
  contentClassName,
  disabled = false,
  label,
  onValueChange,
  options,
  size,
  triggerClassName,
  value,
}: SelectProps) {
  const labelId = useId();

  return (
    <RadixSelect.Root
      disabled={disabled}
      onValueChange={onValueChange}
      value={value}
    >
      <div className={cn("min-w-0", className)}>
        <RadixSelect.Trigger
          aria-labelledby={labelId}
          className={cn(triggerVariant({ size }), triggerClassName)}
          onPointerDownCapture={dismissOpenSelectBeforeOpening}
        >
          <span className="sr-only" id={labelId}>
            {label}
          </span>
          <RadixSelect.Value />
          <RadixSelect.Icon asChild>
            <span
              aria-hidden="true"
              className="text-muted grid shrink-0 place-items-center"
              data-slot="select-icon"
            >
              <ChevronDown />
            </span>
          </RadixSelect.Icon>
        </RadixSelect.Trigger>
      </div>
      <RadixSelect.Portal>
        <RadixSelect.Content
          align="start"
          className={cn(
            "border-line/45 bg-panel text-ink z-50 min-w-(--radix-select-trigger-width) overflow-hidden rounded-xl border data-[state=closed]:hidden!",
            contentClassName,
          )}
          collisionPadding={8}
          position="popper"
          sideOffset={6}
        >
          <RadixSelect.Viewport className="grid gap-1 p-1">
            {options.map((option) => (
              <RadixSelect.Item
                className={itemVariant({ size })}
                key={option.value}
                value={option.value}
              >
                <RadixSelect.ItemIndicator className="absolute left-2 grid size-4 place-items-center">
                  <Check aria-hidden="true" className="size-3.5" />
                </RadixSelect.ItemIndicator>
                <RadixSelect.ItemText>
                  <span className="block truncate">
                    {option.label ?? option.value}
                  </span>
                </RadixSelect.ItemText>
              </RadixSelect.Item>
            ))}
          </RadixSelect.Viewport>
        </RadixSelect.Content>
      </RadixSelect.Portal>
    </RadixSelect.Root>
  );
}
