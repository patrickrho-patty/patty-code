import {
  createDefaultTypographyPreferences,
  normalizeTypographyPreferences,
  TYPOGRAPHY_REGIONS,
} from "../lib/typographyPreferences";

let failed = 0;
function assert(cond: boolean, msg: string) {
  if (!cond) { console.log("  FAIL " + msg); failed += 1; } else { console.log("  PASS " + msg); }
}

// Legacy stored object predates the terminal region; normalize must add it.
const legacy = normalizeTypographyPreferences({ conversation: { followGlobal: false, fontSize: 20 } });
assert(TYPOGRAPHY_REGIONS.includes("terminal"), "terminal is a registered region");
assert(legacy.terminal.followGlobal === true, "legacy prefs get a terminal default");
assert(legacy.terminal.fontSize === 13, "legacy prefs terminal base size is 13");

// Round-trip: terminal region survives a save/load cycle through normalize.
const withTerminal = createDefaultTypographyPreferences();
withTerminal.terminal = { followGlobal: false, fontFamily: "cascadia", customFontName: "", fontSize: 17 };
const round = normalizeTypographyPreferences(withTerminal);
assert(round.terminal.fontFamily === "cascadia", "terminal font round-trips");
assert(round.terminal.fontSize === 17, "terminal size round-trips");

console.log(failed === 0 ? "ALL PASS" : `${failed} FAILED`);
process.exit(failed === 0 ? 0 : 1);
