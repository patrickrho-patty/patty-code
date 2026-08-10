// Run: tsx src/__tests__/tool-approval-mode.test.ts

import { restorableToolApprovalMode, toggleYoloToolApprovalMode } from "../lib/toolApprovalMode";
import { en } from "../locales/en";

let passed = 0;
let failed = 0;

function eq(a: unknown, b: unknown, label: string) {
  if (a === b) {
    process.stdout.write(`  PASS  ${label}\n`);
    passed += 1;
  } else {
    process.stdout.write(`  FAIL  ${label}: expected ${JSON.stringify(b)}, got ${JSON.stringify(a)}\n`);
    failed += 1;
  }
}

console.log("\ntool approval mode");

eq(restorableToolApprovalMode("ask"), "ask", "ask restores to ask");
eq(restorableToolApprovalMode("auto"), "auto", "auto restores to auto");
eq(restorableToolApprovalMode("yolo"), "ask", "yolo cannot be a restore base");

let next = toggleYoloToolApprovalMode("ask");
eq(next.mode, "yolo", "Ctrl+Y turns ask into yolo");
eq(next.restore, "ask", "Ctrl+Y remembers ask");

next = toggleYoloToolApprovalMode("auto");
eq(next.mode, "yolo", "Ctrl+Y turns auto into yolo");
eq(next.restore, "auto", "Ctrl+Y remembers auto");

next = toggleYoloToolApprovalMode("yolo", "auto");
eq(next.mode, "auto", "Ctrl+Y restores auto from yolo");

next = toggleYoloToolApprovalMode("yolo");
eq(next.mode, "ask", "Ctrl+Y falls back to ask when no restore base exists");

eq(en["composer.accessAskTitle"].includes("not read-only"), true, "English Ask copy is not presented as read-only");
eq(en["heartbeat.approvalModeAskHint"].includes("not read-only"), true, "heartbeat Ask hint preserves the same boundary");
eq(en["composer.taskModePlanDesc"].includes("permissions and sandbox"), true, "Plan copy names permissions and sandbox");
eq(en["composer.accessFullDesc"].includes("ordinary") && en["composer.accessFullDesc"].includes("fresh reviews"), true, "English Full access preserves fresh-review boundary");
eq(en["composer.accessFullDesc"].includes("all tool permission approvals"), false, "Full access does not promise to bypass every approval");
eq(en["status.yoloTitle"].includes("ordinary") && en["status.yoloTitle"].includes("fresh reviews"), true, "English YOLO status preserves fresh-review boundary");
eq(en["settings.yolo"].includes("ordinary"), true, "English YOLO setting names ordinary prompts only");

console.log(`\n${passed} passed, ${failed} failed, ${passed + failed} total`);
if (failed > 0) process.exit(1);
