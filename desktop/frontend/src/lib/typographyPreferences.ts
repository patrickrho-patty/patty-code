export const TYPOGRAPHY_REGIONS = ["interface", "conversation", "composer", "code", "terminal", "metadata"] as const;

export type TypographyRegion = (typeof TYPOGRAPHY_REGIONS)[number];

export const REGION_FONT_FAMILIES = [
  "inherit",
  "system",
  "malgun",
  "applesdgothic",
  "notokr",
  "cascadia",
  "jetbrains",
  "sfmono",
  "custom",
] as const;

export type RegionFontFamily = (typeof REGION_FONT_FAMILIES)[number];

export type RegionTypography = {
  followGlobal: boolean;
  fontFamily: RegionFontFamily;
  customFontName: string;
  fontSize: number;
};

export type TypographyPreferences = Record<TypographyRegion, RegionTypography>;

export const TYPOGRAPHY_STORAGE_KEY = "patty-region-typography-v1";

export const TYPOGRAPHY_REGION_META: Record<TypographyRegion, { baseSize: number; min: number; max: number }> = {
  interface: { baseSize: 14, min: 11, max: 20 },
  conversation: { baseSize: 14, min: 12, max: 24 },
  composer: { baseSize: 14, min: 12, max: 24 },
  code: { baseSize: 12, min: 10, max: 22 },
  terminal: { baseSize: 13, min: 9, max: 24 },
  metadata: { baseSize: 12, min: 9, max: 18 },
};

// TERMINAL_MONO_STACK is the fallback default terminal font when no mono family
// is configured — the stack TerminalView previously hardcoded at 13px. Keeping
// it here means the terminal always resolves to a real mono face, never to the
// proportional UI font.
export const TERMINAL_MONO_STACK =
  "ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace";

function terminalStackForFamily(family: RegionFontFamily): string {
  switch (family) {
    case "cascadia":
      return FONT_STACKS.cascadia;
    case "jetbrains":
      return FONT_STACKS.jetbrains;
    case "sfmono":
      return FONT_STACKS.sfmono;
    default:
      return TERMINAL_MONO_STACK;
  }
}

// terminalFontStackFor resolves the xterm font-family stack from a terminal
// region preference. Inherit/system (and any proportional pick) fall back to
// the mono default so a terminal never loses its monospace guarantee; custom
// uses the sanitized custom name.
export function terminalFontStackFor(preference: RegionTypography): string {
  if (preference.fontFamily === "custom") {
    return sanitizeCustomFontName(preference.customFontName) || TERMINAL_MONO_STACK;
  }
  return terminalStackForFamily(preference.fontFamily);
}

// terminalFontSizeFor resolves the terminal font size. Following global
// (inherit) uses the terminal region's base size; otherwise the configured px.
// The value is bounded by the region's declared min/max via normalize.
export function terminalFontSizeFor(preference: RegionTypography): number {
  if (preference.followGlobal) {
    return TYPOGRAPHY_REGION_META.terminal.baseSize;
  }
  const meta = TYPOGRAPHY_REGION_META.terminal;
  return Math.round(Math.min(meta.max, Math.max(meta.min, preference.fontSize)));
}


const FONT_STACKS: Record<RegionFontFamily, string> = {
  inherit: "",
  system: 'var(--font-ui)',
  malgun: '"Malgun Gothic", "Apple SD Gothic Neo", "Noto Sans KR", sans-serif',
  applesdgothic: '"Apple SD Gothic Neo", "Malgun Gothic", "Noto Sans KR", sans-serif',
  notokr: '"Noto Sans KR", "Apple SD Gothic Neo", "Malgun Gothic", sans-serif',
  cascadia: '"Cascadia Code", "Cascadia Mono", Consolas, ui-monospace, monospace',
  jetbrains: '"JetBrains Mono", "Cascadia Code", "SF Mono", Consolas, ui-monospace, monospace',
  sfmono: '"SF Mono", SFMono-Regular, ui-monospace, Menlo, Monaco, monospace',
  custom: "",
};

function defaultRegion(region: TypographyRegion): RegionTypography {
  return {
    followGlobal: true,
    fontFamily: "inherit",
    customFontName: "",
    fontSize: TYPOGRAPHY_REGION_META[region].baseSize,
  };
}

export function createDefaultTypographyPreferences(): TypographyPreferences {
  return Object.fromEntries(TYPOGRAPHY_REGIONS.map((region) => [region, defaultRegion(region)])) as TypographyPreferences;
}

export function sanitizeCustomFontName(value: unknown): string {
  if (typeof value !== "string") return "";
  const compact = value.trim().replace(/\s+/g, " ").slice(0, 200);
  return isSafeCustomFontNameInput(compact) ? compact : "";
}

export function isSafeCustomFontNameInput(value: unknown): value is string {
  return typeof value === "string" && !/[;{}<>]/.test(value);
}

export function normalizeTypographyPreferences(value: unknown): TypographyPreferences {
  const defaults = createDefaultTypographyPreferences();
  const source = value && typeof value === "object" ? (value as Record<string, unknown>) : {};

  for (const region of TYPOGRAPHY_REGIONS) {
    const raw = source[region];
    if (!raw || typeof raw !== "object") continue;
    const candidate = raw as Record<string, unknown>;
    const meta = TYPOGRAPHY_REGION_META[region];
    const numericSize = typeof candidate.fontSize === "number" && Number.isFinite(candidate.fontSize)
      ? candidate.fontSize
      : meta.baseSize;
    defaults[region] = {
      followGlobal: candidate.followGlobal !== false,
      fontFamily: typeof candidate.fontFamily === "string" && (REGION_FONT_FAMILIES as readonly string[]).includes(candidate.fontFamily)
        ? candidate.fontFamily as RegionFontFamily
        : "inherit",
      customFontName: sanitizeCustomFontName(candidate.customFontName),
      fontSize: Math.round(Math.min(meta.max, Math.max(meta.min, numericSize))),
    };
  }
  return defaults;
}

export function getTypographyPreferences(): TypographyPreferences {
  if (typeof localStorage === "undefined") return createDefaultTypographyPreferences();
  try {
    const stored = localStorage.getItem(TYPOGRAPHY_STORAGE_KEY);
    return stored ? normalizeTypographyPreferences(JSON.parse(stored)) : createDefaultTypographyPreferences();
  } catch {
    return createDefaultTypographyPreferences();
  }
}

// Typography change notification: the terminal (and any other live surface)
// subscribes so a preference edit re-applies without a reload. applyTypography
// broadcasts after each write.
const typographyListeners = new Set<() => void>();

export function onTypographyPreferencesChange(listener: () => void): () => void {
  typographyListeners.add(listener);
  return () => typographyListeners.delete(listener);
}

function notifyTypographyPreferencesChange(): void {
  for (const listener of typographyListeners) listener();
}

export function fontStackForPreference(preference: RegionTypography): string {
  if (preference.fontFamily === "custom") return sanitizeCustomFontName(preference.customFontName);
  return FONT_STACKS[preference.fontFamily];
}

export function applyTypographyPreferences(preferences: TypographyPreferences): void {
  if (typeof document === "undefined") return;
  const normalized = normalizeTypographyPreferences(preferences);
  const root = document.documentElement;

  for (const region of TYPOGRAPHY_REGIONS) {
    const preference = normalized[region];
    const scaleProperty = `--typography-${region}-scale`;
    const sizeProperty = `--typography-${region}-size`;
    const fontProperty = `--typography-${region}-font`;
    if (preference.followGlobal) {
      root.style.removeProperty(scaleProperty);
      root.style.removeProperty(sizeProperty);
      root.style.removeProperty(fontProperty);
      continue;
    }
    root.style.setProperty(scaleProperty, String(preference.fontSize / TYPOGRAPHY_REGION_META[region].baseSize));
    root.style.setProperty(sizeProperty, `${preference.fontSize}px`);
    const fontStack = fontStackForPreference(preference);
    if (fontStack) root.style.setProperty(fontProperty, fontStack);
    else root.style.removeProperty(fontProperty);
  }

  try {
    localStorage.setItem(TYPOGRAPHY_STORAGE_KEY, JSON.stringify(normalized));
  } catch {
    /* private mode / no storage */
  }
  notifyTypographyPreferencesChange();
}

export function initTypographyPreferences(): void {
  applyTypographyPreferences(getTypographyPreferences());
}
