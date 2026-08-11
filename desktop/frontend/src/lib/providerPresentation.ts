import type { DictKey } from "./i18n";

type ProviderLabelTranslator = (key: DictKey) => string;

export function builtInProviderLabel(provider: string, t: ProviderLabelTranslator): string {
  switch (provider.trim().toLowerCase()) {
    case "patty":
      return t("settings.providerLabel.patty");
    case "deepseek":
    case "deepseek-flash":
    case "deepseek-pro":
      return t("settings.providerLabel.deepseek");
    default:
      return provider;
  }
}
