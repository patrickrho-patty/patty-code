import { getLocale, type Locale } from "./i18n";

// formatUsageTokens follows the desktop language instead of leaking
// East-Asian compact units into English UI. The explicit locale argument keeps the helper easy to
// verify while the default remains reactive through LocaleProvider rerenders.
export function formatUsageTokens(n: number, locale: Locale = getLocale()): string {
  return new Intl.NumberFormat(locale, {
    notation: "compact",
    maximumFractionDigits: 1,
  }).format(n);
}
