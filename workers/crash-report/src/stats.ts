import { esc, page } from "./shell";
import { type User, userNav } from "./auth";

type Daily = { date: string; users: number; opens: number };
type MetricRow = { signal: string; bucket: string; total: number };
type BarRow = { label: string; users: number };
type BarListOptions = {
  limit?: number;
  className?: string;
  labelFormatter?: (label: string) => string;
};

type OverviewCounts = {
  latestAdoptionPct: number | null;
  openReports: number;
  newLatestReports: number;
  regressedReports: number;
  criticalOpenReports: number;
};

export type StatsModule = "diagnostics" | "usage" | "preferences" | "health";

function lastDays(rows: Daily[], count: 7 | 30): Daily[] {
  const byDate = new Map(rows.map((r) => [r.date, r]));
  const out: Daily[] = [];
  for (let i = count - 1; i >= 0; i--) {
    const date = new Date(Date.now() - i * 86400000).toISOString().slice(0, 10);
    out.push(byDate.get(date) ?? { date, users: 0, opens: 0 });
  }
  return out;
}

function chartTickStep(max: number, targetTicks = 4): number {
  if (max <= targetTicks) return 1;
  const raw = Math.max(1, max) / targetTicks;
  const pow = 10 ** Math.floor(Math.log10(raw));
  const fraction = raw / pow;
  if (fraction <= 1) return pow;
  if (fraction <= 2) return 2 * pow;
  if (fraction <= 5) return 5 * pow;
  return 10 * pow;
}

function chartTickLabel(n: number): string {
  if (n >= 1_000_000) return `${Number((n / 1_000_000).toFixed(n % 1_000_000 === 0 ? 0 : 1))}m`;
  if (n >= 1_000) return `${Number((n / 1_000).toFixed(n % 1_000 === 0 ? 0 : 1))}k`;
  return String(Math.round(n));
}

function i18n(en: string): string {
  return esc(en);
}

function i18nHTML(en: string): string {
  return en;
}

function dailyChart(days: Daily[]): string {
  const W = 960;
  const H = 220;
  const plotLeft = 50;
  const plotRight = 8;
  const plotTop = 16;
  const baseY = H - 26;
  const plotH = baseY - plotTop;
  const slot = (W - plotLeft - plotRight) / days.length;
  const max = Math.max(1, ...days.map((d) => d.opens));
  const step = chartTickStep(max);
  const chartMax = Math.max(step, Math.ceil(max / step) * step);
  const h = (v: number) => (v / chartMax) * plotH;
  const ticks: number[] = [];
  for (let v = 0; v <= chartMax; v += step) ticks.push(v);
  const grid = ticks
    .map((v) => {
      const y = baseY - h(v);
      return `<g><line x1="${plotLeft}" y1="${y}" x2="${W - plotRight}" y2="${y}" class="gridline"/><text x="${plotLeft - 8}" y="${y + 4}" text-anchor="end" class="ay">${chartTickLabel(v)}</text></g>`;
    })
    .join("");
  const bars = days
    .map((d, i) => {
      const x = plotLeft + i * slot;
      const label = i % 5 === 4 ? `<text x="${x + slot / 2}" y="${H - 8}" text-anchor="middle" class="ax">${d.date.slice(5)}</text>` : "";
      return `<g><title>${esc(`${d.date} — ${d.users} users · ${d.opens} opens`)}</title>
<rect x="${x}" y="${plotTop}" width="${slot}" height="${plotH}" fill="transparent" pointer-events="all"/>
<rect x="${x + slot * 0.18}" y="${baseY - h(d.opens)}" width="${slot * 0.64}" height="${h(d.opens)}" rx="3" fill="var(--accent)" opacity="0.22"/>
<rect x="${x + slot * 0.3}" y="${baseY - h(d.users)}" width="${slot * 0.4}" height="${h(d.users)}" rx="3" fill="var(--accent)"/>
${label}</g>`;
    })
    .join("");
  return `<svg class="chart" viewBox="0 0 ${W} ${H}" role="img" aria-label="Daily active installs chart"><style>.ax,.ay{font:11px var(--mono);fill:var(--ink-3)}.gridline{stroke:var(--line);stroke-width:1}</style>
${grid}${bars}</svg>`;
}

function bucketDisplayLabel(signal: string, bucket: string): string {
  if (signal.includes("_model") && bucket.startsWith("custom_")) {
    const model = bucket.slice("custom_".length).replace(/_/g, " ");
    return `<span class="bucket-prefix">custom</span><span class="bucket-main">${esc(model)}</span>`;
  }
  return esc(bucket);
}

function barRow(r: BarRow, max: number, labelFormatter?: (label: string) => string): string {
  const label = labelFormatter ? labelFormatter(r.label) : esc(r.label);
  return `<div class="row" title="${esc(r.label)}"><span class="row-label">${label}</span><div class="row-bar"><div class="bar" style="width:${Math.max(3, Math.round((r.users / max) * 100))}%"></div></div><span class="n">${r.users}</span></div>`;
}

function listBars(rows: BarRow[], options: BarListOptions = {}): string {
  if (!rows.length) return `<div class="empty">${i18n("No data in this window")}</div>`;
  const max = Math.max(1, ...rows.map((r) => r.users));
  const limit = options.limit ?? 5;
  const visible = limit > 0 ? rows.slice(0, limit) : rows;
  const hidden = limit > 0 ? rows.slice(limit) : [];
  const className = options.className ? ` ${esc(options.className)}` : "";
  const visibleRows = visible.map((r) => barRow(r, max, options.labelFormatter)).join("");
  if (!hidden.length) return `<div class="bars-list${className}">${visibleRows}</div>`;
  return `<div class="bars-list${className}">${visibleRows}<details class="bars-more"><summary><span class="more-closed">${i18nHTML(
    `Show ${hidden.length} more`,
  )}</span><span class="more-open">${i18nHTML(`Hide ${hidden.length}`)}</span></summary><div class="bars-more-list">${hidden
    .map((r) => barRow(r, max, options.labelFormatter))
    .join("")}</div></details></div>`;
}

function labelizeBucket(bucket: string): string {
  return bucket.replace(/^n_/, "").replace(/_/g, " ");
}

function sumMetric(rows: MetricRow[], signal: string): number {
  return rows.filter((r) => r.signal === signal).reduce((sum, r) => sum + r.total, 0);
}

function topMetricBucket(rows: MetricRow[], signal: string): string {
  const row = rows.filter((r) => r.signal === signal).sort((a, b) => b.total - a.total)[0];
  return row ? `${labelizeBucket(row.bucket)} · ${row.total}` : "none";
}

function cacheHitRate(rows: MetricRow[]): number | null {
  const cacheRows = rows.filter((r) => r.signal === "cache_hit");
  const total = cacheRows.reduce((sum, r) => sum + r.total, 0);
  if (!total) return null;
  const weighted = cacheRows.reduce((sum, r) => {
    const m = r.bucket.match(/^(\d+)_(\d+)$/);
    const midpoint = m ? (Number(m[1]) + Number(m[2])) / 2 : 0;
    return sum + midpoint * r.total;
  }, 0);
  return weighted / total;
}

function pct(n: number | null): string {
  if (n === null || !Number.isFinite(n)) return "n/a";
  return `${Math.round(n)}%`;
}

function ratioPer100(rows: MetricRow[], signal: string): number | null {
  const turns = sumMetric(rows, "turns");
  if (!turns) return null;
  return (sumMetric(rows, signal) / turns) * 100;
}

function deltaLabel(current: number | null, previous: number | null, suffix = ""): string {
  if (current === null || previous === null) return "new";
  const delta = current - previous;
  if (Math.abs(delta) < 0.05) return "flat";
  const sign = delta > 0 ? "+" : "";
  const rounded = Math.abs(delta) >= 10 ? Math.round(delta) : Number(delta.toFixed(1));
  return `${sign}${rounded}${suffix}`;
}

const METRIC_SIGNAL_LABELS: Record<string, string> = {
  finish_reason: "Finish reason",
  empty_final: "Empty final guard",
  provider_error: "Provider errors",
  cache_hit: "Cache hit rate",
  tool_error: "Tool errors",
  updater_error: "Updater errors",
  updater_event: "Updater events",
  compaction: "Compactions",
  turns: "Turns",
  desktop_hang: "Desktop hangs",
  desktop_hang_age: "Desktop hang age",
  desktop_exit: "Desktop exits",
  desktop_exit_phase: "Abnormal exit phase",
  desktop_uptime: "Uptime before exit",
  desktop_install: "Install profile",
  desktop_update_transition: "Update transition",
  desktop_restore: "Window restore",
  desktop_webview2_failure: "WebView2 failures",
  recovery_failure: "Recovery failures",
  recovery_rule_continue: "Rule recovery continues",
  recovery_review_continue: "Review recovery continues",
  recovery_human_prompt: "Recovery prompts",
  recovery_human_continue: "Human recovery continues",
  recovery_human_revise: "Human recovery revisions",
  recovery_review_error: "Recovery review errors",
  recovery_repeat_prompt: "Repeated recovery prompts",
  recovery_review_latency: "Recovery review latency",
  client_surface: "Client surface",
  client_version: "Client version",
  settings_language: "Settings: language",
  settings_desktop_layout: "Settings: desktop style",
  settings_theme: "Settings: light/dark",
  settings_theme_style: "Settings: theme style",
  settings_close_behavior: "Settings: close behavior",
  settings_display_mode: "Settings: transcript mode",
  settings_status_bar_style: "Settings: status bar style",
  settings_status_bar_items_count: "Settings: status bar items",
  settings_check_updates: "Settings: update checks",
  settings_default_model: "Settings: default model",
  settings_planner_model: "Settings: planner model",
  settings_subagent_model: "Settings: subagent model",
  settings_subagent_effort: "Settings: subagent effort",
  settings_patty_code_language: "Settings: patty code language",
  settings_provider_count: "Settings: provider count",
  settings_provider_access_count: "Settings: enabled providers",
  settings_provider_access: "Settings: provider access",
  settings_bot_enabled: "Bot: enabled",
  settings_bot_model: "Bot: default model",
  settings_bot_tool_approval: "Bot: tool approval",
  settings_bot_allowlist: "Bot: allowlist",
  settings_bot_allow_all: "Bot: allow all",
  settings_bot_connection_count: "Bot: connection count",
  settings_bot_connection_provider: "Bot: connection provider",
  settings_bot_connection_enabled: "Bot: connection enabled",
  settings_bot_connection_status: "Bot: connection status",
  settings_bot_connection_model: "Bot: connection model",
  settings_bot_connection_approval: "Bot: connection approval",
  cli_mode: "CLI mode",
  cli_profile: "CLI profile",
  cli_permission_mode: "CLI permission mode",
  cli_session_mode: "CLI session mode",
  cli_turn_latency: "CLI turn latency",
  cli_exit: "CLI turn outcome",
};

const AGENT_METRIC_SIGNALS = [
  "finish_reason",
  "empty_final",
  "provider_error",
  "cache_hit",
  "tool_error",
  "updater_error",
  "updater_event",
  "compaction",
  "turns",
  "desktop_hang",
  "desktop_hang_age",
  "desktop_exit",
  "desktop_exit_phase",
  "desktop_uptime",
  "desktop_install",
  "desktop_update_transition",
  "desktop_restore",
  "desktop_webview2_failure",
  "cli_turn_latency",
  "cli_exit",
  "recovery_failure",
  "recovery_rule_continue",
  "recovery_review_continue",
  "recovery_human_prompt",
  "recovery_human_continue",
  "recovery_human_revise",
  "recovery_review_error",
  "recovery_repeat_prompt",
  "recovery_review_latency",
];
const DEFAULT_OPEN_SETTING_GROUPS = new Set(["Client", "Models", "Providers"]);

const SETTINGS_METRIC_GROUPS: { en: string; signals: string[] }[] = [
  {
    en: "Client",
    signals: ["client_surface", "client_version", "settings_language", "cli_mode", "cli_profile", "cli_permission_mode", "cli_session_mode"],
  },
  {
    en: "Appearance and layout",
    signals: [
      "settings_desktop_layout",
      "settings_theme",
      "settings_theme_style",
      "settings_display_mode",
      "settings_status_bar_style",
      "settings_status_bar_items_count",
    ],
  },
  {
    en: "Models",
    signals: [
      "settings_default_model",
      "settings_planner_model",
      "settings_subagent_model",
      "settings_subagent_effort",
      "settings_patty_code_language",
    ],
  },
  {
    en: "Providers",
    signals: ["settings_provider_count", "settings_provider_access_count", "settings_provider_access"],
  },
  {
    en: "Behavior toggles",
    signals: ["settings_close_behavior", "settings_check_updates"],
  },
  {
    en: "Bots",
    signals: [
      "settings_bot_enabled",
      "settings_bot_model",
      "settings_bot_tool_approval",
      "settings_bot_allowlist",
      "settings_bot_allow_all",
      "settings_bot_connection_count",
      "settings_bot_connection_provider",
      "settings_bot_connection_enabled",
      "settings_bot_connection_status",
      "settings_bot_connection_model",
      "settings_bot_connection_approval",
    ],
  },
];

function metricSignalLabel(signal: string): string {
  const label = METRIC_SIGNAL_LABELS[signal];
  return label ? i18n(label) : esc(signal);
}

function metricsBySignal(rows: MetricRow[]): Map<string, { label: string; users: number }[]> {
  const bySignal = new Map<string, { label: string; users: number }[]>();
  for (const r of rows) {
    const list = bySignal.get(r.signal) ?? [];
    list.push({ label: r.bucket, users: r.total });
    bySignal.set(r.signal, list);
  }
  return bySignal;
}

function metricBlocks(bySignal: Map<string, BarRow[]>, signals: string[], options: { barLimit?: number } = {}): string {
  return signals
    .filter((signal) => bySignal.has(signal))
    .map((signal) => {
      const rows = bySignal.get(signal) ?? [];
      return `<div class="metric-block"><h3>${metricSignalLabel(signal)}<span>${rows.length}</span></h3>${listBars(rows, {
        limit: options.barLimit ?? 5,
        className: "metric-bars",
        labelFormatter: (label) => bucketDisplayLabel(signal, label),
      })}</div>`;
    })
    .join("");
}

function metricsCards(rows: MetricRow[], signals = AGENT_METRIC_SIGNALS): string {
  if (!rows.length)
    return `<div class="empty">${i18n("No metrics yet — flows in once an opt-in build ships")}</div>`;
  const bySignal = metricsBySignal(rows);
  const blocks = metricBlocks(bySignal, signals);
  return blocks ? `<div class="metrics">${blocks}</div>` : `<div class="empty">${i18n("No data in this window")}</div>`;
}

function settingsDashboard(rows: MetricRow[], options: { collapseSections?: boolean } = {}): string {
  const bySignal = metricsBySignal(rows);
  const sections = SETTINGS_METRIC_GROUPS.map((group) => {
    const availableSignals = group.signals.filter((signal) => bySignal.has(signal));
    const blocks = metricBlocks(bySignal, group.signals);
    if (!blocks) return "";
    const heading = `<h3>${i18n(group.en)}<span>${i18nHTML(`${availableSignals.length} metrics`)}</span></h3>`;
    if (options.collapseSections && !DEFAULT_OPEN_SETTING_GROUPS.has(group.en)) {
      return `<details class="pref-section pref-section-collapsed"><summary>${heading}</summary><div class="metrics pref-metrics">${blocks}</div></details>`;
    }
    return `<section class="pref-section">${heading}<div class="metrics pref-metrics">${blocks}</div></section>`;
  })
    .filter(Boolean)
    .join("");
  if (!sections) return `<div class="empty">${i18n("No settings preference metrics yet")}</div>`;
  return `<div class="preference-dashboard">${sections}</div>`;
}

function healthLevel(kind: "cache" | "rate", value: number | null): "good" | "warn" | "bad" | "unknown" {
  if (value === null) return "unknown";
  if (kind === "cache") {
    if (value >= 80) return "good";
    if (value >= 50) return "warn";
    return "bad";
  }
  if (value <= 1) return "good";
  if (value <= 5) return "warn";
  return "bad";
}

function countHealthLevel(value: number): "good" | "warn" | "bad" {
  if (value <= 0) return "good";
  if (value <= 2) return "warn";
  return "bad";
}

function levelText(level: "good" | "warn" | "bad" | "unknown"): string {
  if (level === "good") return i18n("Good");
  if (level === "warn") return i18n("Watch");
  if (level === "bad") return i18n("Risk");
  return i18n("No data");
}

function healthCard(
  label: string,
  value: string,
  level: "good" | "warn" | "bad" | "unknown",
  deltaHTML: string,
  detailHTML: string,
): string {
  return `<div class="health-card ${level}"><div class="health-top"><span>${i18n(label)}</span><b>${levelText(level)}</b></div>
<strong>${esc(value)}</strong><small>${deltaHTML}</small><p>${detailHTML}</p></div>`;
}

function healthDeltaHTML(value: string): string {
  return i18nHTML(`${esc(value)} vs previous window`);
}

function healthDetailHTML(rows: MetricRow[], signal: string): string {
  return i18nHTML(`${esc(topMetricBucket(rows, signal))} top bucket`);
}

function agentHealth(rows: MetricRow[], previousRows: MetricRow[]): string {
  if (!rows.length) return `<div class="empty">${i18n("No agent health metrics yet")}</div>`;
  const cache = cacheHitRate(rows);
  const prevCache = cacheHitRate(previousRows);
  const desktopHangs = sumMetric(rows, "desktop_hang");
  const prevDesktopHangs = sumMetric(previousRows, "desktop_hang");
  const abnormalExits = rows.filter((r) => r.signal === "desktop_exit" && r.bucket === "abnormal").reduce((sum, r) => sum + r.total, 0);
  const prevAbnormalExits = previousRows.filter((r) => r.signal === "desktop_exit" && r.bucket === "abnormal").reduce((sum, r) => sum + r.total, 0);
  const webViewFailures = sumMetric(rows, "desktop_webview2_failure");
  const prevWebViewFailures = sumMetric(previousRows, "desktop_webview2_failure");
  const rateCard = (signal: string, en: string) => {
    const value = ratioPer100(rows, signal);
    const prev = ratioPer100(previousRows, signal);
    return healthCard(
      en,
      value === null ? "n/a" : `${Number(value.toFixed(value < 10 ? 1 : 0))}/100`,
      healthLevel("rate", value),
      healthDeltaHTML(deltaLabel(value, prev, "/100")),
      healthDetailHTML(rows, signal),
    );
  };
  return `<div class="health-grid">
${healthCard(
  "Cache hit rate",
  pct(cache),
  healthLevel("cache", cache),
  healthDeltaHTML(deltaLabel(cache, prevCache, "pp")),
  healthDetailHTML(rows, "cache_hit"),
)}
${rateCard("provider_error", "Provider errors")}
${rateCard("tool_error", "Tool errors")}
${rateCard("empty_final", "Empty final guard")}
${rateCard("compaction", "Compactions")}
${healthCard(
  "Desktop hangs",
  String(desktopHangs),
  countHealthLevel(desktopHangs),
  healthDeltaHTML(deltaLabel(desktopHangs, prevDesktopHangs)),
  healthDetailHTML(rows, "desktop_hang_age"),
)}
${healthCard(
  "Abnormal desktop exits",
  String(abnormalExits),
  countHealthLevel(abnormalExits),
  healthDeltaHTML(deltaLabel(abnormalExits, prevAbnormalExits)),
  healthDetailHTML(rows, "desktop_exit_phase"),
)}
${healthCard(
  "WebView2 process failures",
  String(webViewFailures),
  countHealthLevel(webViewFailures),
  healthDeltaHTML(deltaLabel(webViewFailures, prevWebViewFailures)),
  healthDetailHTML(rows, "desktop_webview2_failure"),
)}
</div>`;
}

function statusPill(status: string): string {
  if (status === "resolved") return `<span class="pill resolved">resolved</span>`;
  if (status === "ignored") return `<span class="pill ignored">ignored</span>`;
  return "";
}

type CrashRow = {
  fingerprint: string;
  kind: string;
  count: number;
  first_version: string;
  last_version: string;
  seen: string;
  status: string;
  title: string;
  source: string;
  label: string;
  error_type: string;
  top_frame: string;
  severity: string;
  last_os: string;
  last_arch: string;
  last_channel: string;
  regressed_at: string;
  development?: boolean;
};

function clip(s: string, n: number): string {
  return s.length > n ? `${s.slice(0, n - 1)}…` : s;
}

function filterTab(label: string, href: string, active: boolean): string {
  return `<a class="filter-tab${active ? " active" : ""}" href="${esc(href)}">${i18n(label)}</a>`;
}

function facetChip(row: { label: string; users: number }, active: string, hrefFor: (label: string) => string): string {
  const label = row.label || "legacy";
  return `<a class="facet-chip${active === row.label ? " active" : ""}" href="${esc(hrefFor(row.label))}" title="${esc(label)}"><span class="facet-label">${esc(label)}</span><b>${row.users}</b></a>`;
}

function facetChips(rows: { label: string; users: number }[], active: string, hrefFor: (label: string) => string, limit = 5): string {
  if (!rows.length) return `<span class="filter-empty">${i18n("none")}</span>`;
  const visible = rows.slice(0, limit);
  const activeRow = active ? rows.find((r) => r.label === active) : undefined;
  if (activeRow && !visible.some((r) => r.label === activeRow.label)) visible.push(activeRow);
  const visibleKeys = new Set(visible.map((r) => r.label));
  const hidden = rows.filter((r) => !visibleKeys.has(r.label));
  const chips = visible.map((r) => facetChip(r, active, hrefFor)).join("");
  if (!hidden.length) return chips;
  return `${chips}<details class="facet-more"><summary>${i18nHTML(`More ${hidden.length}`)}</summary><div class="facet-more-list">${hidden
    .map((r) => facetChip(r, active, hrefFor))
    .join("")}</div></details>`;
}

function statCard(label: string, value: string, note: string, href: string, tone = ""): string {
  return `<a class="overview-card ${tone}" href="${esc(href)}"><span>${i18n(label)}</span><strong>${esc(value)}</strong><small>${note}</small></a>`;
}

function latestVersionShare(adoptionPct: number | null): string {
  return adoptionPct === null ? "n/a" : `${Math.round(adoptionPct)}%`;
}

function topSeverityTone(openReports: number, regressedReports: number, criticalOpenReports: number): string {
  if (criticalOpenReports || regressedReports) return "bad";
  if (openReports) return "warn";
  return "good";
}

function navLink(href: string, label: string, active = false): string {
  return `<a${active ? ` class="active" aria-current="page"` : ""} href="${esc(href)}">${i18n(label)}</a>`;
}

function preferencePanel(title: string, body: string, active: boolean): string {
  return `<section class="module-panel preference-panel${active ? " active" : ""}"${active ? ` aria-current="true"` : ""}>
<h3>${title}</h3>${body}</section>`;
}

function reportGroups(rows: CrashRow[], compact = false): string {
  if (!rows.length) return `<div class="empty">${i18n("No diagnostic reports yet — that's the good kind of empty")}</div>`;
  return `<div class="crash-list${compact ? " compact" : ""}"><div class="crash-head"><span>${i18n("summary")}</span><span>${i18n("scope")}</span><span>${i18n("health")}</span><span title="${i18n("Groups are filtered by the selected window; occurrence totals are lifetime counts")}">${i18n("lifetime count")}</span></div>${rows
    .map((c) => {
      const platform = [c.last_os, c.last_arch].filter(Boolean).join("/");
      const versions = `${c.first_version || "?"} → ${c.last_version || "?"}`;
      const title = c.title || c.error_type || c.top_frame || c.fingerprint;
      return `<a class="crash-item" href="/stats/group/${esc(c.fingerprint)}" title="${esc(title)}">
<span class="crash-summary"><span>${c.title ? esc(clip(c.title, compact ? 88 : 120)) : `<span class="muted">${i18n("No summary captured")}</span>`}</span><small>${esc(c.fingerprint.slice(0, 8))} · ${esc(c.seen)}</small>${
        c.regressed_at ? `<em>${i18nHTML(`regressed ${esc(c.regressed_at.slice(0, 10))}`)}</em>` : ""
      }</span>
<span class="crash-scope"><small>${esc(c.source || "legacy")}</small><small>${esc(versions)}</small><small>${platform ? esc(platform) : "unknown platform"}</small>${c.last_channel && c.last_channel !== "stable" ? `<small>${esc(c.last_channel)}</small>` : ""}</span>
<span class="crash-health"><span class="pill">${esc(c.severity || "medium")}</span><span class="pill ${c.kind === "crash" ? "crash" : ""}">${esc(c.kind)}</span>${statusPill(c.status)}</span>
<span class="crash-count">${c.count}</span>
</a>`;
    })
    .join("")}</div>`;
}

export function renderStats(
  data: {
    daily: Daily[];
    versions: { label: string; users: number }[];
    platforms: { label: string; users: number }[];
    crashes: CrashRow[];
    metrics: MetricRow[];
    previousMetrics: MetricRow[];
    metricUsers: MetricRow[];
    metricUsersUnavailable: boolean;
    /** Oldest computed_at in the rollup; empty when the window was queried live. */
    metricUsersComputedAt: string;
    sources: { label: string; users: number }[];
    overview: OverviewCounts;
    latestVersion: string;
    filters: {
      surface: "desktop" | "cli";
      status: string;
      source: string;
      version: string;
      os: string;
      platform: string;
      newLatest: boolean;
      regressed: boolean;
      windowDays: 7 | 30;
      preferenceMode: "users" | "opens";
    };
  },
  user: User,
  activeModule: StatsModule = "usage",
): string {
  const days = lastDays(data.daily, data.filters.windowDays);
  const range = data.filters.windowDays;
  const rangeText = `${range}d`;
  const totalUsers = days.at(-1)?.users ?? 0;
  const anyPing = days.some((d) => d.opens > 0);
  const agentMetrics = data.metrics.filter((r) => AGENT_METRIC_SIGNALS.includes(r.signal));
  const previousAgentMetrics = data.previousMetrics.filter((r) => AGENT_METRIC_SIGNALS.includes(r.signal));
  const agentMetricUsers = data.metricUsers.filter((r) => AGENT_METRIC_SIGNALS.includes(r.signal));
  const isSettingsSignal = (signal: string) =>
    signal === "client_surface" || signal === "client_version" || signal.startsWith("settings_") ||
    ["cli_mode", "cli_profile", "cli_permission_mode", "cli_session_mode"].includes(signal);
  const settingsMetrics = data.metrics.filter((r) => isSettingsSignal(r.signal));
  const settingsMetricUsers = data.metricUsers.filter((r) => isSettingsSignal(r.signal));
  const cache = cacheHitRate(agentMetrics);
  const providerRate = ratioPer100(agentMetrics, "provider_error");
  const toolRate = ratioPer100(agentMetrics, "tool_error");
  const desktopHangs = sumMetric(agentMetrics, "desktop_hang");
  const abnormalExits = agentMetrics
    .filter((r) => r.signal === "desktop_exit" && r.bucket === "abnormal")
    .reduce((sum, r) => sum + r.total, 0);
  const webViewFailures = sumMetric(agentMetrics, "desktop_webview2_failure");
  const healthWatchCount =
    [healthLevel("cache", cache), healthLevel("rate", providerRate), healthLevel("rate", toolRate)].filter((v) => v === "warn" || v === "bad").length +
    (desktopHangs > 0 ? 1 : 0) +
    (abnormalExits > 0 ? 1 : 0) +
    (webViewFailures > 0 ? 1 : 0);
  const modulePath = (module: StatsModule) => (module === "usage" ? "/stats" : `/stats/${module}`);
  const filterQS = (patch: Record<string, string>, module: StatsModule = activeModule) => {
    const params = new URLSearchParams();
    const put = (k: string, v: string) => {
      if (v) params.set(k, v);
    };
    put("status", data.filters.status);
    put("source", data.filters.source);
    put("version", data.filters.version);
    put("os", data.filters.os);
    put("platform", data.filters.platform);
    put("surface", data.filters.surface === "cli" ? "cli" : "");
    if (data.filters.newLatest) params.set("new", "latest");
    if (data.filters.regressed) params.set("regressed", "1");
    if (data.filters.windowDays === 7) params.set("window", "7d");
    if (module === "preferences" && data.filters.preferenceMode === "opens") params.set("prefs", "opens");
    for (const [k, v] of Object.entries(patch)) {
      if (v) params.set(k, v);
      else params.delete(k);
    }
    const qs = params.toString();
    const path = modulePath(module);
    return qs ? `${path}?${qs}` : path;
  };
  const clearFiltersHref = filterQS({ status: "", source: "", version: "", os: "", platform: "", new: "", regressed: "" });
  const hasFilters = Boolean(
    data.filters.status || data.filters.source || data.filters.version || data.filters.os || data.filters.platform || data.filters.newLatest || data.filters.regressed,
  );
  const windowControls = `<div class="segmented" aria-label="Time window">
<a class="${range === 7 ? "active" : ""}"${range === 7 ? ` aria-current="true"` : ""} href="${esc(filterQS({ window: "7d" }))}">7d</a>
<a class="${range === 30 ? "active" : ""}"${range === 30 ? ` aria-current="true"` : ""} href="${esc(filterQS({ window: "" }))}">30d</a>
</div>`;
  const surfaceControls = `<div class="segmented" aria-label="Client surface">
<a class="${data.filters.surface === "desktop" ? "active" : ""}"${data.filters.surface === "desktop" ? ` aria-current="true"` : ""} href="${esc(filterQS({ surface: "" }))}">${i18n("Desktop")}</a>
<a class="${data.filters.surface === "cli" ? "active" : ""}"${data.filters.surface === "cli" ? ` aria-current="true"` : ""} href="${esc(filterQS({ surface: "cli" }))}">CLI</a>
</div>`;
  const preferenceControls = `<div class="segmented" aria-label="Preference metric mode">
<a class="${data.filters.preferenceMode === "users" ? "active" : ""}"${data.filters.preferenceMode === "users" ? ` aria-current="true"` : ""} href="${esc(
    filterQS({ prefs: "" }, "preferences"),
  )}">${i18n("Installs")}</a>
<a class="${data.filters.preferenceMode === "opens" ? "active" : ""}"${data.filters.preferenceMode === "opens" ? ` aria-current="true"` : ""} href="${esc(
    filterQS({ prefs: "opens" }, "preferences"),
  )}">${i18n("Opens")}</a>
</div>`;
  const overviewTone = topSeverityTone(data.overview.openReports, data.overview.regressedReports, data.overview.criticalOpenReports);
  const isDevelopmentDiagnostic = (row: CrashRow) => row.development ?? row.fingerprint.startsWith("dev:");
  const releaseCrashes = data.crashes.filter(
    (row) => row.kind !== "performance" && row.severity !== "low" && !isDevelopmentDiagnostic(row),
  );
  const performanceDiagnostics = data.crashes.filter(
    (row) => row.kind === "performance" && !isDevelopmentDiagnostic(row),
  );
  const developmentDiagnostics = data.crashes.filter(isDevelopmentDiagnostic);
  const overview = `<section class="overview-grid">
${statCard("Active today", String(totalUsers), i18n("anonymous installs"), filterQS({}, "usage"))}
${statCard("Latest adoption", latestVersionShare(data.overview.latestAdoptionPct), i18nHTML(`latest ${esc(data.latestVersion || "n/a")}`), filterQS({}, "usage"))}
${statCard("Open reports", String(data.overview.openReports), i18n("needs triage"), filterQS({}, "diagnostics"), overviewTone)}
${statCard("New in latest", String(data.overview.newLatestReports), i18n("first seen on latest"), filterQS({}, "diagnostics"), data.overview.newLatestReports ? "warn" : "good")}
${statCard("Regressions", String(data.overview.regressedReports), i18n("previously resolved"), filterQS({}, "diagnostics"), data.overview.regressedReports ? "bad" : "good")}
${statCard("Agent health", healthWatchCount ? String(healthWatchCount) : "OK", i18nHTML(`${pct(cache)} cache · ${providerRate === null ? "n/a" : Number(providerRate.toFixed(1))}/100 provider · ${desktopHangs} hangs`), filterQS({}, "health"), healthWatchCount ? "warn" : "good")}
</section>`;
  const pageOverview = activeModule === "usage" ? overview : "";
  const dashboardNav = `<nav class="site-nav" aria-label="Stats navigation">
${navLink(filterQS({}, "usage"), "Home", activeModule === "usage")}
${navLink(filterQS({}, "diagnostics"), "Diagnostics", activeModule === "diagnostics")}
${navLink(filterQS({}, "preferences"), "Preferences", activeModule === "preferences")}
${navLink(filterQS({}, "health"), "Agent Health", activeModule === "health")}
</nav>`;
  const filters = `<div class="filter-card"><div class="filter-head"><h2>${i18n("Report filters")}</h2><span>${i18nHTML(`latest ${esc(data.latestVersion || "n/a")}`)}</span></div>
<div class="filter-tabs">
${filterTab("All", clearFiltersHref, !hasFilters)}
${filterTab("Open", filterQS({ status: "open" }), data.filters.status === "open")}
${filterTab("Resolved", filterQS({ status: "resolved" }), data.filters.status === "resolved")}
${filterTab("Ignored", filterQS({ status: "ignored" }), data.filters.status === "ignored")}
${filterTab("New in latest", filterQS({ new: data.filters.newLatest ? "" : "latest" }), data.filters.newLatest)}
${filterTab("Regressed", filterQS({ regressed: data.filters.regressed ? "" : "1" }), data.filters.regressed)}
</div>
<div class="facet-grid">
<section><h3>${i18n("Source")}</h3><div class="facet-list">${facetChips(data.sources, data.filters.source, (label) => filterQS({ source: label }), 4)}</div></section>
<section><h3>${i18n("Version")}</h3><div class="facet-list">${facetChips(data.versions, data.filters.version, (label) => filterQS({ version: label }), 5)}</div></section>
<section><h3>${i18n("Platform")}</h3><div class="facet-list">${facetChips(data.platforms, data.filters.platform, (label) => filterQS({ platform: label }), 4)}</div></section>
</div></div>`;
  const usageModule = `<section id="usage" class="card full module-card"><div class="module-head"><div><span>${i18n("Module")}</span><h2>${i18n("Usage distribution")}</h2></div></div>
<div class="module-panel wide"><h3>${i18nHTML(`Daily active installs <b>— ${rangeText}</b> (solid: users, faded: opens)`)}</h3>
${anyPing ? dailyChart(days) : `<div class="empty">${i18n("No pings yet — data starts flowing once a telemetry-enabled build ships")}</div>`}</div>
<div class="module-split">
<section class="module-panel"><h3>${i18nHTML(`Versions <b>— ${rangeText}</b>`)}</h3>${listBars(data.versions)}</section>
<section class="module-panel"><h3>${i18nHTML(`Platforms <b>— ${rangeText}</b>`)}</h3>${listBars(data.platforms)}</section>
</div></section>`;
  const diagnosticsModule = `<section id="diagnostics" class="card full module-card"><div class="module-head"><div><span>${i18n("Module")}</span><h2>${i18n("Diagnostic triage")}</h2></div><a class="module-action" href="#top">${i18n("Back to overview")}</a></div>
<section class="module-panel"><h3>${i18nHTML("Needs attention <b>— top 10 release crashes and exceptions</b>")}</h3>${reportGroups(releaseCrashes.slice(0, 10), true)}</section>
${performanceDiagnostics.length ? `<section class="module-panel"><h3>${i18nHTML("Performance signals <b>— tracked separately from crashes</b>")}</h3>${reportGroups(performanceDiagnostics.slice(0, 5), true)}</section>` : ""}
${developmentDiagnostics.length ? `<section class="module-panel"><h3>${i18nHTML("Development diagnostics <b>— excluded from release priority</b>")}</h3>${reportGroups(developmentDiagnostics.slice(0, 5), true)}</section>` : ""}
${filters}
<section class="module-panel"><h3>${i18nHTML("All report groups <b>— open, regression, severity, count, recency</b>")}</h3>${reportGroups(data.crashes)}</section>
</section>`;
  const sevenDayHref = esc(filterQS({ window: "7d" }, "preferences"));
  const unavailableNotice =
    range === 30
      ? i18nHTML(
          `The 30-day deduplication is precomputed hourly and has not reached every signal yet. <a href="${sevenDayHref}">Use 7d</a> meanwhile.`,
        )
      : i18nHTML(`The ${rangeText} deduplication did not finish.`);
// [[[[[[A precomputed window can silently go stale if the rollup cron stops, so the]]]]]]
// [[[[[[heading carries how old the least recently recomputed signal is.]]]]]]
  const computedAt = data.metricUsersComputedAt
    ? ` <b>${esc(data.metricUsersComputedAt.slice(0, 16).replace("T", " "))}Z</b>`
    : "";
  const healthComputedAt = data.metricUsersComputedAt
    ? ` ${esc(data.metricUsersComputedAt.slice(0, 16).replace("T", " "))}Z`
    : "";
  const installsPanel = preferencePanel(
    i18nHTML(
      `Deduplicated installs <b>— ${rangeText}</b>${computedAt ? ` computed${computedAt}` : ""}`,
    ),
    data.metricUsersUnavailable
      ? `<div class="empty">${unavailableNotice}</div>`
      : settingsDashboard(settingsMetricUsers, { collapseSections: true }),
    data.filters.preferenceMode === "users",
  );
  const opensPanel = preferencePanel(
    i18nHTML(`Launch/open snapshots <b>— ${rangeText}</b>`),
    settingsDashboard(settingsMetrics, { collapseSections: true }),
    data.filters.preferenceMode === "opens",
  );
  const preferencePanels = data.filters.preferenceMode === "opens" ? `${opensPanel}${installsPanel}` : `${installsPanel}${opensPanel}`;
  const preferencesModule = `<section id="preferences" class="card full module-card"><div class="module-head"><div><span>${i18n("Module")}</span><h2>${i18n("Settings preferences")}</h2></div><div class="module-actions">${preferenceControls}</div></div>
<div class="preference-compare">${preferencePanels}</div></section>`;
  const healthModule = `<section id="health" class="card full module-card"><div class="module-head"><div><span>${i18n("Module")}</span><h2>${i18n("Agent health")}</h2></div><div class="module-actions"><a class="module-action" href="${esc(filterQS({}, "preferences"))}">${i18n("Preferences")}</a></div></div>
<section class="module-panel"><h3>${i18nHTML(`Health summary <b>— ${rangeText}, compared with previous window</b>`)}</h3>${agentHealth(agentMetrics, previousAgentMetrics)}</section>
<section class="module-panel"><h3>${i18nHTML(`Affected installs <b>— ${rangeText}, deduplicated${healthComputedAt ? `, computed ${healthComputedAt}` : ""}</b>`)}</h3>${
    data.metricUsersUnavailable
      ? `<div class="empty">${range === 30
          ? i18nHTML(`The 30-day deduplication is not ready. <a href="${esc(filterQS({ window: "7d" }, "health"))}">Use 7d</a> meanwhile.`)
          : i18n(`The ${rangeText} deduplication did not finish.`)}</div>`
      : metricsCards(agentMetricUsers, ["desktop_hang", "desktop_hang_age", "desktop_webview2_failure", "desktop_restore", "desktop_exit"])
  }</section>
<section class="module-panel"><h3>${i18nHTML(`Signal distributions <b>— ${rangeText}, opt-in aggregate</b>`)}</h3>${metricsCards(agentMetrics)}</section>
</section>`;
  const activeModuleHTML: Record<StatsModule, string> = {
    diagnostics: diagnosticsModule,
    usage: usageModule,
    preferences: preferencesModule,
    health: healthModule,
  };

  return page(
    "Patty Code · Crash & Telemetry",
    "health",
    `${dashboardNav}
<div id="top" class="hero-line"><div><h1>${i18n("Crash & Telemetry")}</h1><p class="sub">${i18nHTML(
      `${rangeText} window · anonymous launch pings, opt-in aggregate metrics, and user-sent diagnostic reports only`,
    )}</p></div><div class="module-actions">${surfaceControls}${windowControls}</div></div>
${pageOverview}
<div class="grid">
${activeModuleHTML[activeModule]}
</div>`,
    userNav(user),
  );
}

function fmtDevice(deviceJSON: string): string {
  try {
    const d = JSON.parse(deviceJSON) as { osVersion?: string; cpu?: string; cores?: number; ramGb?: number };
    return [d.osVersion, d.cpu, d.cores ? `${d.cores} cores` : "", d.ramGb ? `${d.ramGb} GB RAM` : ""]
      .filter(Boolean)
      .join(" · ");
  } catch {
    return "";
  }
}

export type Group = {
  fingerprint: string;
  kind: string;
  count: number;
  first_seen: string;
  last_seen: string;
  first_version: string;
  last_version: string;
  status: string;
  note: string;
  title: string;
  source: string;
  label: string;
  error_type: string;
  top_frame: string;
  severity: string;
  last_os: string;
  last_arch: string;
  last_build_commit: string;
  last_channel: string;
  resolved_in: string;
  resolved_at: string;
  regressed_at: string;
};

type ReportSample = {
  version: string;
  os: string;
  arch: string;
  message: string;
  device: string;
  created_at: string;
  source: string;
  label: string;
  error_type: string;
  error_message: string;
  top_frame: string;
  build_commit: string;
  channel: string;
  language: string;
  view: string;
  breadcrumbs: string;
  component_stack: string;
  stack: string;
  occurred_at: string;
};

function manageGroup(group: Group): string {
  const fp = esc(group.fingerprint);
  const setStatus = (s: string, label: string, cls: string) =>
    group.status === s
      ? ""
      : `<form method="post" action="/stats/group/${fp}" class="inline"><input type="hidden" name="action" value="status"><input type="hidden" name="status" value="${s}"><button class="btn ${cls} sm" type="submit">${i18n(label)}</button></form>`;
  return `<div class="card full manage-card"><div class="manage-head"><h2>${i18nHTML("Manage <b>— admin</b>")}</h2><div class="manage-actions">${setStatus("resolved", "Mark resolved", "ghost")}${setStatus("ignored", "Ignore", "ghost")}${setStatus("open", "Reopen", "ghost")}
<form method="post" action="/stats/group/${fp}" class="inline" onsubmit="return confirm('Delete this crash group and all its samples?')"><input type="hidden" name="action" value="delete"><button class="btn danger sm" type="submit">${i18n("Delete group")}</button></form></div></div>
<div class="manage-grid">
<form method="post" action="/stats/group/${fp}" class="manage-form"><input type="hidden" name="action" value="resolution"><label>${i18n("Resolved in")}<input type="text" name="resolvedIn" placeholder="v1.10.1" value="${esc(group.resolved_in)}"></label><button class="btn sm" type="submit">${i18n("Save")}</button></form>
<form method="post" action="/stats/group/${fp}" class="manage-form"><input type="hidden" name="action" value="severity"><label>${i18n("Severity")}<select name="severity"><option${group.severity === "low" ? " selected" : ""}>low</option><option${group.severity === "medium" ? " selected" : ""}>medium</option><option${group.severity === "high" ? " selected" : ""}>high</option><option${group.severity === "critical" ? " selected" : ""}>critical</option></select></label><button class="btn sm" type="submit">${i18n("Save")}</button></form>
<form method="post" action="/stats/group/${fp}" class="manage-form wide"><input type="hidden" name="action" value="note"><label>${i18n("Note")}<input type="text" name="note" placeholder="${esc("Add investigation note")}" value="${esc(group.note)}"></label><button class="btn sm" type="submit">${i18n("Save")}</button></form>
</div></div>`;
}

function breadcrumbsList(json: string): string {
  try {
    const rows = JSON.parse(json) as { cat?: string; msg?: string }[];
    if (!Array.isArray(rows) || rows.length === 0) return "";
    return `<details class="sample-nested"><summary>${i18n("breadcrumbs")}</summary><pre>${esc(rows.map((b) => `[${b.cat ?? ""}] ${b.msg ?? ""}`).join("\n"))}</pre></details>`;
  } catch {
    return "";
  }
}

function sampleReport(r: ReportSample, i: number): string {
  const dev = fmtDevice(r.device);
  const platform = [r.os, r.arch].filter(Boolean).join("/");
  const title = r.error_message || r.message.split("\n").find((line) => line.trim()) || r.error_type || "sample";
  const structured = [
    r.source && [i18n("source"), r.source],
    r.label && [i18n("label"), r.label],
    r.error_type && [i18n("type"), r.error_type],
    r.top_frame && [i18n("top"), r.top_frame],
    r.build_commit && [i18n("build"), r.build_commit],
    r.channel && [i18n("channel"), r.channel],
    r.view && [i18n("view"), r.view],
  ]
    .filter(Boolean)
    .map(([label, value]) => `<span><b>${label}</b>${esc(value)}</span>`)
    .join("");
  const stack = r.stack || r.component_stack;
  return `<details class="sample" ${i === 0 ? "open" : ""}><summary>
<span class="sample-id"><b>${esc(r.version)}</b><small>${esc(platform || "unknown platform")}</small></span>
<span class="sample-title">${esc(clip(title, 110))}</span>
<span class="sample-time">${esc((r.occurred_at || r.created_at).slice(0, 19).replace("T", " "))}</span>
</summary>
<div class="sample-body">
<div class="sample-meta">${dev ? `<span><b>${i18n("device")}</b>${esc(dev)}</span>` : ""}${structured}</div>
<div class="sample-actions"><button class="btn ghost sm copy-btn" type="button" data-copy="${esc(r.message)}"><span class="copy-label">${i18n("Copy message")}</span></button>${
    stack
      ? `<button class="btn ghost sm copy-btn" type="button" data-copy="${esc(stack)}"><span class="copy-label">${i18n("Copy stack")}</span></button>`
      : ""
  }</div>
<pre>${esc(r.message)}</pre>
${stack ? `<details class="sample-nested"><summary>${i18n("stack")}</summary><pre>${esc(stack)}</pre></details>` : ""}
${breadcrumbsList(r.breadcrumbs)}
</div></details>`;
}

function sampleReports(reports: ReportSample[], options: { limit?: number } = {}): string {
  if (!reports.length) return `<div class="empty">${i18n("No raw samples stored for this group")}</div>`;
  const limit = options.limit ?? 10;
  const visible = reports.slice(0, limit);
  const hidden = reports.slice(limit);
  const visibleSamples = visible.map((r, i) => sampleReport(r, i)).join("");
  const hiddenSamples = hidden.map((r, i) => sampleReport(r, i + limit)).join("");
  const history =
    hidden.length > 0
      ? `<details class="sample-more"><summary>${i18nHTML(`Historical samples ${hidden.length}`)}</summary><div class="sample-more-list">${hiddenSamples}</div></details>`
      : "";
  return `<div class="sample-list">${visibleSamples}${history}</div>`;
}

export function renderGroup(
  group: Group,
  reports: ReportSample[],
  user: User,
): string {
  const samples = sampleReports(reports);
  const platform = [group.last_os, group.last_arch].filter(Boolean).join("/");
  const status = statusPill(group.status) || `<span class="pill open">${i18n("open")}</span>`;
  const tags = [
    [i18n("source"), group.source || "legacy"],
    group.label && [i18n("label"), group.label],
    group.error_type && [i18n("type"), group.error_type],
    group.top_frame && [i18n("top frame"), group.top_frame],
    platform && [i18n("platform"), platform],
    group.last_build_commit && [i18n("build"), group.last_build_commit],
    group.last_channel && [i18n("channel"), group.last_channel],
  ]
    .filter(Boolean)
    .map(([label, value]) => `<span><b>${label}</b>${esc(value)}</span>`)
    .join("");
  const metrics = [
    [i18n("Occurrences"), String(group.count)],
    [i18n("First seen"), `${group.first_seen.slice(0, 10)} · ${group.first_version || "?"}`],
    [i18n("Last seen"), `${group.last_seen.slice(0, 10)} · ${group.last_version || "?"}`],
    [i18n("Version range"), `${group.first_version || "?"} → ${group.last_version || "?"}`],
    group.resolved_in && [i18n("Resolved in"), group.resolved_in],
    group.regressed_at && [i18n("Regressed"), group.regressed_at.slice(0, 10)],
  ]
    .filter(Boolean)
    .map(([label, value]) => `<div><span>${label}</span><b>${esc(value)}</b></div>`)
    .join("");

  return page(
    `Patty Code · ${group.fingerprint.slice(0, 8)}`,
    `stats / ${group.fingerprint.slice(0, 8)}`,
    `<section class="group-hero"><div class="group-nav"><a class="back" href="/stats">${i18n("Back to stats")}</a><button class="btn ghost sm copy-btn" type="button" data-copy="${esc(group.fingerprint)}"><span class="copy-label">${i18n("Copy fingerprint")}</span></button></div>
<div class="group-title"><span class="pill ${group.kind === "crash" ? "crash" : ""}">${esc(group.kind)}</span><h1>${esc(group.fingerprint.slice(0, 8))}</h1>${status}</div>
${group.title ? `<p class="summary group-summary">${esc(group.title)}</p>` : ""}
<div class="group-tags">${tags}</div>
<div class="group-metrics">${metrics}</div>
${group.note ? `<p class="group-note">${i18n("Note")}: ${esc(group.note)}</p>` : ""}</section>
<div class="card full sample-card"><h2>${i18nHTML("Samples <b>— newest first, first sample plus latest 5 kept</b>")}</h2>${samples}</div>
${user.role === "admin" ? manageGroup(group) : ""}
<a class="back" href="/stats">${i18n("Back to stats")}</a>`,
    userNav(user),
  );
}
