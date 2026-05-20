import {
  createContext,
  type ReactNode,
  useContext,
  useEffect,
  useMemo,
  useState,
} from "react";

export type ThemePreference = "system" | "light" | "dark";

const storageKey = "patchpilot.theme";
const themePreferences: ReadonlyArray<ThemePreference> = [
  "system",
  "light",
  "dark",
];

interface ThemeContextValue {
  preference: ThemePreference;
  setPreference: (preference: ThemePreference) => void;
}

const ThemeContext = createContext<ThemeContextValue | null>(null);

export function parseThemePreference(value: string | null): ThemePreference {
  return themePreferences.includes(value as ThemePreference)
    ? (value as ThemePreference)
    : "system";
}

export function applyThemePreference(
  root: HTMLElement,
  preference: ThemePreference,
) {
  if (preference === "system") {
    root.removeAttribute("data-theme");
    return;
  }

  root.setAttribute("data-theme", preference);
}

export function ThemeProvider({ children }: { children: ReactNode }) {
  const [preference, setPreference] = useState(readStoredThemePreference);

  useEffect(() => {
    applyThemePreference(document.documentElement, preference);
    writeStoredThemePreference(preference);
  }, [preference]);

  const value = useMemo(() => ({ preference, setPreference }), [preference]);

  return (
    <ThemeContext.Provider value={value}>{children}</ThemeContext.Provider>
  );
}

export function useThemePreference() {
  const value = useContext(ThemeContext);

  if (!value) {
    throw new Error("useThemePreference must be used within ThemeProvider");
  }

  return value;
}

function readStoredThemePreference(): ThemePreference {
  try {
    return parseThemePreference(globalThis.localStorage?.getItem(storageKey));
  } catch {
    return "system";
  }
}

function writeStoredThemePreference(preference: ThemePreference) {
  try {
    globalThis.localStorage?.setItem(storageKey, preference);
  } catch {
    // Theme persistence is progressive enhancement; keep the in-memory choice.
  }
}
