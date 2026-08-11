export const COMPACT_RATIO_MIN = 0.65;
export const COMPACT_RATIO_MAX = 0.97;
export const PATTY_MEDIUM_CONTEXT_WINDOW = 248124;
export const PATTY_MEDIUM_COMPACT_TOKENS = 238123;
export const PATTY_MEDIUM_COMPACT_RATIO = PATTY_MEDIUM_COMPACT_TOKENS / PATTY_MEDIUM_CONTEXT_WINDOW;
export const PATTY_MEDIUM_COMPACT_FORCE_RATIO = 0.98;

// Older desktop backends can omit these fields. Keep their historical values
// explicit instead of making them look like the current Patty stock contract.
export const LEGACY_COMPACT_RATIO = 0.8;
export const LEGACY_COMPACT_FORCE_RATIO = 0.9;
export const LEGACY_COMPACT_RATIO_MAX = 0.85;

export type CompactRatioBounds = {
  min: number;
  max: number;
  snip: number;
  force: number;
};

export function compactRatioEditableMax(forceRatio: number, absoluteMax = COMPACT_RATIO_MAX): number {
  const finiteForce = Number.isFinite(forceRatio) && forceRatio > 0 ? forceRatio : LEGACY_COMPACT_FORCE_RATIO;
  if (finiteForce > absoluteMax) return absoluteMax;
  return Math.min(absoluteMax, (Math.ceil(finiteForce * 10_000) - 1) / 10_000);
}

export function compactRatioIsEditable(ratio: number, bounds: CompactRatioBounds = {
  min: COMPACT_RATIO_MIN,
  max: COMPACT_RATIO_MAX,
  snip: 0,
  force: 0,
}): boolean {
  return Number.isFinite(ratio)
    && ratio >= bounds.min
    && ratio <= bounds.max
    && (bounds.snip <= 0 || ratio > bounds.snip)
    && (bounds.force <= 0 || ratio < bounds.force);
}

export function formatCompactRatioPercent(ratio: number): string {
  if (!Number.isFinite(ratio)) return "";
  return (ratio * 100).toFixed(2).replace(/\.00$/, "").replace(/(\.\d)0$/, "$1");
}

export function compactRatioDraftChanged(draftPercent: number, savedRatio: number, touched: boolean): boolean {
  return touched && Number.isFinite(draftPercent) && Math.abs(draftPercent / 100 - savedRatio) > 0.0001;
}
