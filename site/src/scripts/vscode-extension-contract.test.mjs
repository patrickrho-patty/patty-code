import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const source = (path) => readFile(new URL(path, import.meta.url), "utf8");

test("homepage presents one VS Code extension through both registries", async () => {
  const page = await source("../pages/index.astro");

  assert.match(page, /data-pane="vscode"/);
  assert.match(page, /data-pane="desktop"[\s\S]*data-pane="npm"[\s\S]*data-pane="brew"[\s\S]*data-pane="vscode"/);
  assert.match(page, /Editor extension/);
  assert.match(page, /More ways to use Patty Code:/);
  assert.match(page, /Local Web UI:[\s\S]*patcode serve/);
  assert.match(page, /ACP editor integration/);
  assert.match(page, /SivanLiu\.patty-agent/);
  assert.match(page, /marketplace\.visualstudio\.com\/items\?itemName=SivanLiu\.patty-agent/);
  assert.match(page, /open-vsx\.org\/extension\/SivanLiu\/patty-code-agent/);
  assert.match(page, /does not bundle the CLI/);
  assert.match(page, /data-goto="vscode"/);
});

test("mobile download channels use a two-column tab grid", async () => {
  const css = await source("../styles/global.css");
  const mobile = css.slice(css.indexOf("@media (max-width: 640px)"), css.indexOf("@media (max-width: 360px)"));

  assert.match(mobile, /\.dl-tabs \{\s*display: grid; grid-template-columns: repeat\(2, minmax\(0, 1fr\)\)/);
});
