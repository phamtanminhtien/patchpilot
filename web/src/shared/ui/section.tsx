import type { ReactNode } from "react";

import { classNames } from "./class-name";
import { createVariant, type VariantPropsOf } from "./variant";

const sectionVariant = createVariant({
  base: "shadow-sm",
  variants: {
    tone: {
      panel: "bg-panel",
      transparent: "bg-transparent",
    },
  },
  defaultVariants: {
    tone: "panel",
  },
});

type SectionVariantProps = VariantPropsOf<typeof sectionVariant>;

interface SectionProps {
  children: ReactNode;
  className?: string;
  eyebrow?: string;
  title: string;
  tone?: SectionVariantProps["tone"];
}

export function Section({
  children,
  className,
  eyebrow,
  title,
  tone,
}: SectionProps) {
  return (
    <section className={classNames(sectionVariant({ tone }), className)}>
      <div className="mx-auto grid w-full max-w-6xl gap-4 px-4 py-5 sm:px-6">
        <div className="grid gap-1">
          {eyebrow ? (
            <p className="text-muted text-xs font-semibold uppercase">
              {eyebrow}
            </p>
          ) : null}
          <h1 className="text-ink text-2xl font-semibold">{title}</h1>
        </div>
        {children}
      </div>
    </section>
  );
}
