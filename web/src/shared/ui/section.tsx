import type { ReactNode } from "react";

interface SectionProps {
  children: ReactNode;
  eyebrow?: string;
  title: string;
}

export function Section({ children, eyebrow, title }: SectionProps) {
  return (
    <section className="border-line bg-panel border-b">
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
