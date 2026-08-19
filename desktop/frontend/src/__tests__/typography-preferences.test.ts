// Run: tsx src/__tests__/typography-preferences.test.ts

import {
  TYPOGRAPHY_STORAGE_KEY,
  TYPOGRAPHY_REGION_META,
  applyTypographyPreferences,
  createDefaultTypographyPreferences,
  fontStackForPreference,
  isSafeCustomFontNameInput,
  normalizeTypographyPreferences,
  sanitizeCustomFontName,
  terminalFontSizeFor,
  terminalFontStackFor,
} from "../lib/typographyPreferences";

let passed = 0;
let failed = 0;

function eq(actual: unknown, expected: unknown, label: string) {
  if (JSON.stringify(actual) === JSON.stringify(expected)) {
    process.stdout.write(`  PASS  ${label}\n`);
    passed += 1;
  } else {
    process.stdout.write(`  FAIL  ${label}: expected ${JSON.stringify(expected)}, got ${JSON.stringify(actual)}\n`);
    failed += 1;
  }
}

console.log("\nregional typography preferences");

const defaults = createDefaultTypographyPreferences();
eq(defaults.conversation.followGlobal, true, "regions follow global by default");
eq(defaults.code.fontSize, TYPOGRAPHY_REGION_META.code.baseSize, "code uses its semantic base size");
eq(defaults.terminal.fontSize, TYPOGRAPHY_REGION_META.terminal.baseSize, "terminal uses its 13px base size");
eq(defaults.metadata.fontSize, 12, "metadata defaults to the existing 12px supporting-text size");

const normalized = normalizeTypographyPreferences({
  conversation: { followGlobal: false, fontFamily: "applesdgothic", fontSize: 99 },
  metadata: { followGlobal: false, fontFamily: "custom", customFontName: "  IBM   Plex Sans  ", fontSize: 8 },
});
eq(normalized.conversation.fontSize, TYPOGRAPHY_REGION_META.conversation.max, "oversized values clamp to the region maximum");
eq(normalized.metadata.fontSize, TYPOGRAPHY_REGION_META.metadata.min, "undersized values clamp to the region minimum");
eq(normalized.metadata.customFontName, "IBM Plex Sans", "custom names are normalized");
eq(normalized.interface.followGlobal, true, "missing regions retain backward-compatible defaults");

eq(sanitizeCustomFontName("Bad; font"), "", "unsafe CSS delimiters are rejected");
eq(isSafeCustomFontNameInput("IBM Plex Sans"), true, "safe custom font input remains editable");
eq(isSafeCustomFontNameInput("Bad; font"), false, "unsafe custom font input is blocked before state changes");
eq(fontStackForPreference(normalized.conversation).includes("Apple SD Gothic Neo"), true, "preset resolves to a usable font stack");

const applied = new Map<string, string>([["--typography-interface-size", "20px"]]);
const stored = new Map<string, string>();
Object.defineProperty(globalThis, "document", {
  configurable: true,
  value: {
    documentElement: {
      style: {
        removeProperty: (name: string) => applied.delete(name),
        setProperty: (name: string, value: string) => applied.set(name, value),
      },
    },
  },
});
Object.defineProperty(globalThis, "localStorage", {
  configurable: true,
  value: { setItem: (key: string, value: string) => stored.set(key, value) },
});

const custom = createDefaultTypographyPreferences();
custom.conversation = { followGlobal: false, fontFamily: "applesdgothic", customFontName: "", fontSize: 24 };
custom.code = { followGlobal: false, fontFamily: "jetbrains", customFontName: "", fontSize: 15 };
custom.metadata = { followGlobal: false, fontFamily: "inherit", customFontName: "", fontSize: defaults.metadata.fontSize };
applyTypographyPreferences(custom);
eq(applied.get("--typography-conversation-size"), "24px", "custom regions expose an exact CSS size");
eq(applied.get("--typography-code-size"), "15px", "code exposes an exact CSS size independent of root tokens");
eq(applied.get("--typography-metadata-size"), "12px", "disabling metadata follow-global preserves its rendered size");
eq(applied.get("--typography-metadata-scale"), "1", "metadata uses the same base as the global supporting-text token");
eq(applied.has("--typography-interface-size"), false, "follow-global regions clear stale exact sizes");
eq(stored.has(TYPOGRAPHY_STORAGE_KEY), true, "applied preferences remain persisted");

// Terminal font resolution: default follow-global falls back to the mono
// stack at the base size; mono picks resolve to their stack; a proportional
// family (which can't be chosen from the terminal UI) still degrades to mono;
// custom uses the sanitized name.
eq(terminalFontStackFor(createDefaultTypographyPreferences().terminal).startsWith("ui-monospace"), true, "terminal default resolves to the mono stack");
eq(terminalFontSizeFor(createDefaultTypographyPreferences().terminal), 13, "terminal follow-global uses the base size");
const monoPref: import("../lib/typographyPreferences").RegionTypography = { followGlobal: false, fontFamily: "jetbrains", customFontName: "", fontSize: 16 };
eq(terminalFontStackFor(monoPref).includes("JetBrains Mono"), true, "selected mono family resolves to its stack");
eq(terminalFontSizeFor(monoPref), 16, "terminal custom size wins when not following global");
const proportional: import("../lib/typographyPreferences").RegionTypography = { followGlobal: false, fontFamily: "malgun", customFontName: "", fontSize: 99 };
eq(terminalFontStackFor(proportional), terminalFontStackFor({ followGlobal: true, fontFamily: "inherit", customFontName: "", fontSize: 13 }), "proportional pick degrades to mono fallback");
eq(terminalFontSizeFor(proportional), TYPOGRAPHY_REGION_META.terminal.max, "terminal size clamps to its region max");
const customPref: import("../lib/typographyPreferences").RegionTypography = { followGlobal: false, fontFamily: "custom", customFontName: "  Cascadia Code  ", fontSize: 14 };
eq(terminalFontStackFor(customPref), "Cascadia Code", "terminal custom font uses the sanitized name");

console.log(`\n${passed} passed, ${failed} failed, ${passed + failed} total`);
if (failed > 0) process.exit(1);
