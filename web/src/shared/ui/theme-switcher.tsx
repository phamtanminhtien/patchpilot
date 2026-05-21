import { Monitor, Moon, Sun } from "lucide-react";

import { cn } from "./class-name";

type ThemePreference = "system" | "light" | "dark";

const options = [
  { icon: Monitor, label: "System", value: "system" },
  { icon: Sun, label: "Light", value: "light" },
  { icon: Moon, label: "Dark", value: "dark" },
] as const;

interface ThemeSwitcherProps {
  className?: string;
  onChange: (preference: ThemePreference) => void;
  value: ThemePreference;
}

export function ThemeSwitcher({
  className,
  onChange,
  value,
}: ThemeSwitcherProps) {
  return (
    <div
      aria-label="Theme"
      className={cn(
        "bg-panel inline-grid grid-cols-3 rounded-md p-0.5 shadow-sm",
        className,
      )}
      role="group"
    >
      {options.map((option) => (
        <button
          aria-pressed={value === option.value}
          className={cn(
            "text-muted hover:bg-hover hover:text-ink inline-flex min-h-7 min-w-7 cursor-pointer items-center justify-center rounded-sm px-1.5 transition",
            value === option.value
              ? "bg-accent-soft text-accent shadow-sm"
              : undefined,
          )}
          key={option.value}
          onClick={() => onChange(option.value)}
          type="button"
          title={option.label}
        >
          <option.icon aria-hidden="true" className="size-4 shrink-0" />
          <span className="sr-only">{option.label}</span>
        </button>
      ))}
    </div>
  );
}
