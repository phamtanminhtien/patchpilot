import {
  createContext,
  type ReactNode,
  useContext,
  useEffect,
  useMemo,
  useState,
} from "react";

import type { SettingsPreferences } from "@/shared/api";

export const defaultAppFont = "Inter, ui-sans-serif, system-ui, sans-serif";
export const defaultMonoFont =
  "ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace";

interface AppearanceFonts {
  appFontFamily: string;
  codeFontFamily: string;
  terminalFontFamily: string;
}

interface AppearanceContextValue extends AppearanceFonts {
  applyPreferences: (preferences: Partial<SettingsPreferences>) => void;
  setFonts: (fonts: AppearanceFonts) => void;
}

const storageKey = "patchpilot.appearance.fonts";
const AppearanceContext = createContext<AppearanceContextValue | null>(null);

const fallbackAppearance: AppearanceContextValue = {
  appFontFamily: defaultAppFont,
  applyPreferences: () => {},
  codeFontFamily: defaultMonoFont,
  setFonts: () => {},
  terminalFontFamily: defaultMonoFont,
};

export function AppearanceProvider({ children }: { children: ReactNode }) {
  const [fonts, setFontsState] = useState<AppearanceFonts>(readStoredFonts);

  useEffect(() => {
    applyFontVariables(fonts);
    writeStoredFonts(fonts);
  }, [fonts]);

  const value = useMemo<AppearanceContextValue>(
    () => ({
      ...fonts,
      applyPreferences(preferences) {
        setFontsState((current) => {
          const next = {
            appFontFamily: preferences.appFontFamily ?? current.appFontFamily,
            codeFontFamily:
              preferences.codeFontFamily ?? current.codeFontFamily,
            terminalFontFamily:
              preferences.terminalFontFamily ?? current.terminalFontFamily,
          };
          return fontsEqual(current, next) ? current : next;
        });
      },
      setFonts(nextFonts) {
        setFontsState((current) =>
          fontsEqual(current, nextFonts) ? current : nextFonts,
        );
      },
    }),
    [fonts],
  );

  return (
    <AppearanceContext.Provider value={value}>
      {children}
    </AppearanceContext.Provider>
  );
}

export function useAppearance() {
  const value = useContext(AppearanceContext);
  if (!value) {
    return fallbackAppearance;
  }
  return value;
}

function readStoredFonts(): AppearanceFonts {
  try {
    const parsed = JSON.parse(
      globalThis.localStorage?.getItem(storageKey) ?? "{}",
    ) as Partial<AppearanceFonts>;
    return normalizeFonts(parsed);
  } catch {
    return normalizeFonts({});
  }
}

function writeStoredFonts(fonts: AppearanceFonts) {
  try {
    globalThis.localStorage?.setItem(storageKey, JSON.stringify(fonts));
  } catch {
    // Font persistence is progressive enhancement; keep the in-memory choice.
  }
}

function normalizeFonts(fonts: Partial<AppearanceFonts>): AppearanceFonts {
  return {
    appFontFamily: fonts.appFontFamily || defaultAppFont,
    codeFontFamily: fonts.codeFontFamily || defaultMonoFont,
    terminalFontFamily: fonts.terminalFontFamily || defaultMonoFont,
  };
}

function fontsEqual(first: AppearanceFonts, second: AppearanceFonts) {
  return (
    first.appFontFamily === second.appFontFamily &&
    first.codeFontFamily === second.codeFontFamily &&
    first.terminalFontFamily === second.terminalFontFamily
  );
}

function applyFontVariables(fonts: AppearanceFonts) {
  const root = document.documentElement;
  root.style.setProperty("--pp-font-app", fonts.appFontFamily);
  root.style.setProperty("--pp-font-code", fonts.codeFontFamily);
  root.style.setProperty("--pp-font-terminal", fonts.terminalFontFamily);
}
