// i18n is the desktop's localization seam. It mirrors theme.ts's "persist a choice
// and apply it" shape, but UI text must re-render on a switch, so the active locale
// lives in React state behind a context — flipping it re-renders the whole tree
// (App is a child of the provider). A module-level mirror (`currentLocale`) lets
// non-React code (lib/tools.ts) translate too; it stays fresh because the provider
// updates it on every render.
//
// Desktop UI language is intentionally separate from the CLI/kernel `language`
// config for prompts and terminal text. The desktop preference is persisted in
// the user-level [desktop] config; localStorage is only read once for legacy
// migration from older desktop builds.

import { createContext, useCallback, useContext, useEffect, useState } from "react";
import type { ReactNode } from "react";
import { en, type DictKey } from "../locales/en";

export type Locale = "en";
export type { DictKey };
// LangPref is the stored preference; only English is shipped.
export type LangPref = "" | "en";

type Dict = Record<DictKey, string>;

const DICTS: Partial<Record<Locale, Dict>> = { en };
const STORAGE_KEY = "patty-lang";

// currentLocale mirrors the active locale for callers outside React (lib/tools.ts).
let currentLocale: Locale = "en";

// Whimsical present-participles cycled in the status line while a turn runs. Kept
// out of the dict (it's an array, and purely decorative) but localized all the same.
export const SPINNER_WORDS: Record<Locale, string[]> = {
  en: [
    "Frolicking", "Pondering", "Noodling", "Brewing", "Conjuring", "Cogitating",
    "Percolating", "Ruminating", "Simmering", "Synthesizing", "Tinkering",
    "Marinating", "Crunching", "Hatching", "Mulling", "Whirring", "Forging",
    "Spelunking", "Puttering", "Vibing",
  ],
};

export function detectLocale(_pref: LangPref): Locale {
  return "en";
}

export function preloadLocale(_locale: Locale): Promise<void> {
  return Promise.resolve();
}

export function preloadDetectedLocale(pref: LangPref = ""): Promise<void> {
  return preloadLocale(detectLocale(pref));
}

function readPref(): LangPref {
  return "";
}

export function normalizeLangPref(v: unknown): LangPref {
  return v === "en" ? v : "";
}

export function readLegacyLangPref(): LangPref {
  const v = typeof localStorage !== "undefined" ? localStorage.getItem(STORAGE_KEY) : null;
  return normalizeLangPref(v);
}

export function clearLegacyLangPref(): void {
  try {
    localStorage.removeItem(STORAGE_KEY);
  } catch {
    /* private mode / no storage */
  }
}

// translate resolves a key for a locale and fills {placeholders}. Missing keys fall
// back to English, then to the raw key, so the UI never renders blank.
function translate(locale: Locale, key: DictKey, vars?: Record<string, string | number>): string {
  const s = DICTS[locale]?.[key] ?? en[key] ?? key;
  if (!vars) return s;
  return s.replace(/\{(\w+)\}/g, (_, k) => (vars[k] !== undefined ? String(vars[k]) : `{${k}}`));
}

// t is the non-reactive translator for code outside React (e.g. lib/tools.ts). It
// reads the module mirror, which the provider keeps in sync.
export function t(key: DictKey, vars?: Record<string, string | number>): string {
  return translate(currentLocale, key, vars);
}

export function getLocale(): Locale {
  return currentLocale;
}

export type Translator = (key: DictKey, vars?: Record<string, string | number>) => string;

interface I18nValue {
  locale: Locale;
  pref: LangPref;
  setPref: (pref: LangPref) => void;
  t: Translator;
}

const I18nContext = createContext<I18nValue | null>(null);

export function LocaleProvider({ children }: { children: ReactNode }) {
  const [pref, setPrefState] = useState<LangPref>(() => readPref());
  const [dictionaryVersion, setDictionaryVersion] = useState(0);
  const locale = detectLocale(pref);
  currentLocale = locale; // keep the mirror fresh for non-React callers

  useEffect(() => {
    if (typeof document === "undefined") return;
    document.documentElement.lang = "en";
  }, [locale]);

  useEffect(() => {
    if (DICTS[locale]) return;
    let cancelled = false;
    void preloadLocale(locale).then(() => {
      if (!cancelled) setDictionaryVersion((version) => version + 1);
    });
    return () => {
      cancelled = true;
    };
  }, [locale]);

  // setPref updates only the live UI; persistence is handled by desktop config.
  const setPref = useCallback((next: LangPref) => {
    setPrefState(normalizeLangPref(next));
  }, []);

  const tt = useCallback<Translator>(
    (key, vars) => translate(detectLocale(pref), key, vars),
    [dictionaryVersion, pref],
  );

  return <I18nContext.Provider value={{ locale, pref, setPref, t: tt }}>{children}</I18nContext.Provider>;
}

export function useI18n(): I18nValue {
  const ctx = useContext(I18nContext);
  if (!ctx) throw new Error("useI18n must be used within a LocaleProvider");
  return ctx;
}

// useT is the common shorthand: just the translator.
export function useT(): Translator {
  return useI18n().t;
}
