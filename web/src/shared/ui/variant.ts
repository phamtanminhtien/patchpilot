import { cn } from "./class-name";

type VariantSchema = Record<string, Record<string, string>>;
type VariantKey<T> = Extract<keyof T, string>;
type VariantClassName = false | null | string | undefined;

export type VariantSelection<Variants extends VariantSchema> = {
  [Name in VariantKey<Variants>]?: VariantKey<Variants[Name]> | null;
};

export type VariantProps<Variants extends VariantSchema> =
  VariantSelection<Variants> & {
    className?: VariantClassName;
  };

export type CompoundVariant<Variants extends VariantSchema> =
  VariantSelection<Variants> & {
    className: string;
  };

export interface VariantConfig<Variants extends VariantSchema> {
  base?: VariantClassName;
  compoundVariants?: ReadonlyArray<CompoundVariant<Variants>>;
  defaultVariants?: VariantSelection<Variants>;
  variants: Variants;
}

export type VariantPropsOf<VariantFn> = VariantFn extends (
  props?: infer Props,
) => string
  ? NonNullable<Props>
  : never;

export function createVariant<const Variants extends VariantSchema>({
  base,
  compoundVariants = [],
  defaultVariants,
  variants,
}: VariantConfig<Variants>): (selection?: VariantProps<Variants>) => string {
  return (selection: VariantProps<Variants> = {}) => {
    const resolvedSelection = resolveVariantSelection(
      variants,
      defaultVariants,
      selection,
    );
    const variantClasses = Object.entries(variants).map(([name, values]) => {
      const selected = resolvedSelection[name as VariantKey<Variants>];

      if (selected == null) {
        return undefined;
      }

      return values[selected];
    });
    const compoundClasses = compoundVariants.map((compoundVariant) => {
      const { className, ...conditions } = compoundVariant;
      const matches = (
        Object.entries(conditions) as Array<
          [VariantKey<Variants>, string | null | undefined]
        >
      ).every(
        ([name, selected]) =>
          selected == null || resolvedSelection[name] === selected,
      );

      return matches ? className : undefined;
    });

    return cn(base, ...variantClasses, ...compoundClasses, selection.className);
  };
}

function resolveVariantSelection<Variants extends VariantSchema>(
  variants: Variants,
  defaultVariants: VariantSelection<Variants> | undefined,
  selection: VariantProps<Variants>,
): VariantSelection<Variants> {
  const resolvedSelection = {
    ...defaultVariants,
  } as VariantSelection<Variants>;

  for (const name of Object.keys(variants) as Array<VariantKey<Variants>>) {
    const selected = selection[name];

    if (selected === undefined) {
      continue;
    }

    resolvedSelection[name] = selected;
  }

  return resolvedSelection;
}
