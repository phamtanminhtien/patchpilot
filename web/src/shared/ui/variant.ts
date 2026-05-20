export function createVariant<const Variants extends Record<string, string>>(
  variants: Variants,
) {
  return (variant: keyof Variants) => variants[variant];
}
