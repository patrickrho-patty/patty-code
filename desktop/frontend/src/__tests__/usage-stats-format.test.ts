// Run: tsx src/__tests__/usage-stats-format.test.ts

import { formatUsageTokens } from "../lib/usageStatsFormat";
import { en } from "../locales/en";

let failed = 0;

function ok(value: boolean, label: string) {
  if (value) {
    process.stdout.write(`  PASS  ${label}\n`);
  } else {
    process.stdout.write(`  FAIL  ${label}\n`);
    failed += 1;
  }
}

const english = formatUsageTokens(10_000, "en");

ok(!english.includes("만") && !english.includes("억"), "English token totals do not use Chinese units");
ok(en["settings.modelTab.stats"] === "Usage stats", "English stats tab has a dedicated label");

if (failed > 0) process.exit(1);
