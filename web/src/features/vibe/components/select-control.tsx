export function SelectControl({
  label,
  onChange,
  options,
  value,
}: {
  label: string;
  onChange: (value: string) => void;
  options: string[];
  value: string;
}) {
  return (
    <label className="text-muted flex min-h-10 min-w-0 items-center gap-2 rounded-md px-3 text-xs font-medium">
      <span>{label}</span>
      <select
        className="bg-hover text-ink rounded-sm px-2 py-1 text-xs"
        onChange={(event) => onChange(event.target.value)}
        value={value}
      >
        {options.map((option) => (
          <option key={option} value={option}>
            {option}
          </option>
        ))}
      </select>
    </label>
  );
}
