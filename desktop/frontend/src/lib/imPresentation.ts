export const IM_PLATFORM = "im";
export const IM_FALLBACK_LABEL = "IM";

export function imDisplayLabel(label?: string, provider?: string, domain?: string): string {
  const explicitLabel = label?.trim();
  if (explicitLabel) return explicitLabel;
  const connectionLabel = [provider?.trim(), domain?.trim()].filter(Boolean).join(" / ");
  return connectionLabel || IM_FALLBACK_LABEL;
}
