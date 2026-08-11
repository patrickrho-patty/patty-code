import {
  COMPACT_RATIO_MAX,
  COMPACT_RATIO_MIN,
  LEGACY_COMPACT_RATIO_MAX,
  PATTY_MEDIUM_COMPACT_RATIO,
  PATTY_MEDIUM_COMPACT_TOKENS,
  PATTY_MEDIUM_CONTEXT_WINDOW,
  compactRatioDraftChanged,
  compactRatioEditableMax,
  compactRatioIsEditable,
  formatCompactRatioPercent,
} from "../lib/compactRatio";

let passed = 0;
let failed = 0;

function ok(value: boolean, label: string) {
  if (value) {
    process.stdout.write(`  PASS  ${label}\n`);
    passed += 1;
  } else {
    process.stdout.write(`  FAIL  ${label}\n`);
    failed += 1;
  }
}

console.log("\ncompact ratio contract");

ok(COMPACT_RATIO_MIN === 0.65 && COMPACT_RATIO_MAX === 0.97, "frontend bounds match the backend editor contract");
ok(LEGACY_COMPACT_RATIO_MAX === 0.85, "older backends retain their historical editor ceiling");
ok(PATTY_MEDIUM_CONTEXT_WINDOW === 248124 && PATTY_MEDIUM_COMPACT_TOKENS === 238123, "stock token thresholds remain exact");
ok(PATTY_MEDIUM_COMPACT_RATIO === 238123 / 248124, "stock ratio derives from the token contract");
ok(compactRatioIsEditable(238123 / 248124), "the exact Patty stock ratio is editable");
ok(!compactRatioIsEditable(0.98), "the force boundary is not editable");
ok(compactRatioEditableMax(0.9) === 0.8999, "legacy force thresholds constrain the representable editable maximum");
ok(compactRatioEditableMax(0.90005) === 0.9, "non-millistep force thresholds retain valid representable values");
ok(!compactRatioIsEditable(0.8, { min: 0.65, max: 0.97, snip: 0.8, force: 0.98 }), "the backend snip boundary is exclusive");
ok(!compactRatioIsEditable(0.9, { min: 0.65, max: 0.97, snip: 0.6, force: 0.9 }), "the backend force boundary is exclusive");
ok(formatCompactRatioPercent(238123 / 248124) === "95.97", "stock ratio display preserves CLI precision");
ok(!compactRatioDraftChanged(96, 238123 / 248124, false), "an untouched rounded draft preserves the exact stock threshold");
ok(compactRatioDraftChanged(96, 238123 / 248124, true), "an explicit edit to 96 percent is treated as a change");

console.log(`\n${passed} passed, ${failed} failed`);
if (failed > 0) process.exit(1);
