export type DecisionSurfaceKind =
  | "tool_approval"
  | "plan_approval"
  | "ask"
  | "workspace_conflict"
  | "mode_jobs"
  | "close_active"
  | "clear_context";

export const DECISION_SURFACE_MOCK_TRIGGERS: Readonly<Record<DecisionSurfaceKind, string>> = {
  tool_approval: "/mock-tool-approval",
  plan_approval: "/mock-plan-approval",
  ask: "/mock-ask",
  workspace_conflict: "/mock-workspace-conflict",
  mode_jobs: "/mock-mode-jobs",
  close_active: "/mock-close-active",
  clear_context: "/mock-clear-context",
};

// QA-only stress scene. Keep this separate from the seven product decision
// surfaces so the canonical inventory stays stable while copy/layout extremes
// can be exercised through the same Ask card used in production.
export const LONG_DECISION_OPTIONS_MOCK_TRIGGER = "/mock-long-options";

const aliases: Readonly<Record<string, DecisionSurfaceKind>> = {
  ...Object.fromEntries(
    Object.entries(DECISION_SURFACE_MOCK_TRIGGERS).map(([kind, trigger]) => [trigger, kind]),
  ) as Record<string, DecisionSurfaceKind>,
  "/approve-preview": "tool_approval",
  "approve preview": "tool_approval",
  "approve-preview": "tool_approval",
  "/plan-approve-preview": "plan_approval",
  "plan approve preview": "plan_approval",
  "plan-approve-preview": "plan_approval",
  "/ask-preview": "ask",
  "ask preview": "ask",
  "ask-preview": "ask",
  "mock tool approval": "tool_approval",
  "mock plan approval": "plan_approval",
  "mock ask": "ask",
  "mock workspace conflict": "workspace_conflict",
  "mock mode switch": "mode_jobs",
  "mock close task": "close_active",
  "mock clear context": "clear_context",
};

export function decisionSurfaceMockFromInput(input: string): DecisionSurfaceKind | null {
  return aliases[input.trim().toLowerCase()] ?? null;
}

export function isLongDecisionOptionsMockInput(input: string): boolean {
  const normalized = input.trim().toLowerCase();
  return normalized === LONG_DECISION_OPTIONS_MOCK_TRIGGER || normalized === "mock long option text";
}
