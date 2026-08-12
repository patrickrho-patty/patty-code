#!/usr/bin/env node

import { existsSync, readFileSync } from "node:fs";
import { execFileSync } from "node:child_process";

const files = execFileSync("git", ["ls-files", "README.md", ":(glob)docs/**/*.md", "site/src/pages/*", "site/src/pages/**/*"], {
  encoding: "utf8",
}).trim().split("\n").filter(Boolean);
const migrationReferences = new Set([
  "docs/operations/RELEASING.md",
  "docs/operations/SIGNPATH_WINDOWS_ADMIN_SOP.md",
  "docs/guides/CLI.md",
]);
const rules = [
  [/release-channel-switch/, "public release channel selector"],
  [/patcode upgrade (?:preview|stable)\b/i, "deprecated channel install command"],
  [/[?&]channel=(?:preview|canary)\b/i, "public channel deep link"],
  [/npm (?:i|install).*(?:@canary|@next)\b/i, "public prerelease npm install"],
  [/\bStable 1\.x\b|· stable\b|current 1\.x stable\b/i, "public Stable channel label"],
];
const failures = [];
for (const file of files) {
  if (migrationReferences.has(file) || file.includes("/changelog/")) continue;
  if (!existsSync(file)) continue; // tracked-but-deleted or sparse-checkout
  const source = readFileSync(file, "utf8");
  for (const [pattern, description] of rules) {
    if (pattern.test(source)) failures.push(`${file}: ${description}`);
  }
}
if (failures.length) {
  console.error(`single-release public contract failed:\n${failures.join("\n")}`);
  process.exit(1);
}
console.log(`single-release public contract OK (${files.length} files checked)`);
