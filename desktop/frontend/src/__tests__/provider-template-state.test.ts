import { officialProviderTemplateState } from "../components/SettingsPanel";
import type { ProviderView } from "../lib/types";

const patty = {
  name: "patty",
  apiKeyEnv: "AGENTS_PATTY_API_KEY",
  added: true,
  keySet: true,
} as ProviderView;

const installed = officialProviderTemplateState("patty", "AGENTS_PATTY_API_KEY", [patty]);
if (!installed.added || !installed.keySet) {
  throw new Error(`installed Patty template state was lost: ${JSON.stringify(installed)}`);
}

const available = officialProviderTemplateState("deepseek", "DEEPSEEK_API_KEY", [patty]);
if (available.added || available.keySet) {
  throw new Error(`uninstalled DeepSeek template state was fabricated: ${JSON.stringify(available)}`);
}

console.log("provider template state: 2 passed");
