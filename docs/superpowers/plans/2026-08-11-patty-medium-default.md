# Patty Medium Default Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make stock Patcode resolve, display, and exclusively list the Patty Omni `medium` combo with a 248124-token context and a 238123-token automatic-compaction threshold.

**Architecture:** Reuse the existing OpenAI-compatible provider, provider/model resolver, global compact-ratio pipeline, and model picker. Replace the stock provider catalog and ratio, then reconcile every stock-facing surface: onboarding, official-provider lifecycle, browser mocks, legacy migration, title generation, documentation, and picker presentation. Retain canonical refs internally and preserve optional/legacy DeepSeek support.

**Tech Stack:** Go 1.25, Bubble Tea v2, TOML configuration, React/TypeScript, Vitest-style script tests, Make/codesign.

## Global Constraints

- Provider base URL is exactly `https://omni.agents.patty.io/v1`.
- Wire and visible model name are exactly `medium`.
- Provider credential is sourced from `AGENTS_PATTY_API_KEY`, imported through Patty's existing secure credential-store path when required, and never written to repository or plaintext configuration files.
- Context window is exactly `248124`; `int(contextWindow * compactRatio)` is exactly `238123`.
- The stock `/model` picker has one visible entry, `medium`, while its internal ref remains `patty/medium`.
- Preserve customized provider behavior; do not hard-code the picker to hide user or extension models.
- Restore required Patty wire fields when official access is enabled while preserving safe custom headers and request extras.
- Hold the user-config lock before the credential lock while validating, storing the key, and committing official-provider access.
- Preserve unrelated worktree changes.

---

### Task 1: Stock provider and compaction defaults

**Files:**
- Create: `internal/config/defaults_test.go`
- Modify: `internal/config/config.go:1378-1425`
- Modify: `internal/config/edit.go:428-444`
- Modify: `internal/config/edit_test.go:623-653`

**Interfaces:**
- Consumes: `Config`, `ProviderEntry`, `Config.SetCompactRatio(float64) error`.
- Produces: a stock `Config` whose canonical model ref is `patty/medium` and whose compact ratio derives the requested token boundary.

- [ ] **Step 1: Write the failing default-contract test**

```go
func TestDefaultUsesPattyMedium(t *testing.T) {
	cfg := Default()
	if cfg.DefaultModel != "patty/medium" || len(cfg.Providers) != 1 {
		t.Fatalf("default model/providers = %q/%+v", cfg.DefaultModel, cfg.Providers)
	}
	p := cfg.Providers[0]
	if p.Name != "patty" || p.Kind != "openai" || p.BaseURL != "https://omni.agents.patty.io/v1" || p.Model != "medium" || p.APIKeyEnv != "AGENTS_PATTY_API_KEY" || p.ContextWindow != 248124 {
		t.Fatalf("default provider = %+v", p)
	}
	if got := int(float64(p.ContextWindow) * cfg.Agent.CompactRatio); got != 238123 {
		t.Fatalf("auto compact threshold = %d, want 238123", got)
	}
	if cfg.Agent.CompactForceRatio != 0.98 {
		t.Fatalf("force ratio = %v, want 0.98", cfg.Agent.CompactForceRatio)
	}
}
```

- [ ] **Step 2: Run the focused test and verify RED**

Run: `go test ./internal/config -run 'TestDefaultUsesPattyMedium|TestSetCompactRatio' -count=1`

Expected: `TestDefaultUsesPattyMedium` fails because the stock model is still DeepSeek.

- [ ] **Step 3: Replace the stock defaults and widen backend validation**

Use one OpenAI provider entry:

```go
DefaultModel: "patty/medium",
Agent: AgentConfig{
	SoftCompactRatio:    0.5,
	ToolResultSnipRatio: 0.6,
	CompactRatio:        float64(238123) / float64(248124),
	CompactForceRatio:   0.98,
},
Providers: []ProviderEntry{{
	Name: "patty", Kind: "openai", BaseURL: "https://omni.agents.patty.io/v1",
	Model: "medium", APIKeyEnv: "AGENTS_PATTY_API_KEY", ContextWindow: 248124,
}},
```

Change `SetCompactRatio`'s absolute upper validation boundary from `0.85` to `0.97`; retain the existing snip-ratio and force-ratio ordering checks.

- [ ] **Step 4: Update the ratio boundary table and verify GREEN**

Add the shipped ratio and `0.97` to accepted values, and replace the rejected `0.86` case with `0.98`. Run:

`go test ./internal/config -run 'TestDefaultUsesPattyMedium|TestSetCompactRatio' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit the config slice**

```bash
git add internal/config/config.go internal/config/edit.go internal/config/defaults_test.go internal/config/edit_test.go
git commit -m "feat(config): default to Patty medium"
```

### Task 2: Single-model TUI picker presentation

**Files:**
- Modify: `internal/cli/model_test.go:12-86`
- Modify: `internal/cli/model.go:105-129`

**Interfaces:**
- Consumes: `modelRefs() []string`, `chatTUI.openModelPicker()`.
- Produces: canonical picker item ID `patty/medium`, visible label `medium`, and top-bar label `medium` through the existing boot model-label path.

- [ ] **Step 1: Write failing model catalog and picker tests**

Configure `AGENTS_PATTY_API_KEY` in the isolated credential store, assert `modelRefs()` equals `[]string{"patty/medium"}`, then assert:

```go
m := newTestChatTUI()
m.modelRef = "patty/medium"
m.openModelPicker()
if len(m.quickPick.items) != 1 || m.quickPick.items[0].ID != "patty/medium" || m.quickPick.items[0].Label != "medium" {
	t.Fatalf("picker items = %+v", m.quickPick.items)
}
```

The production change that makes this pass is the stock provider replacement plus rendering the parsed model portion instead of the full canonical ref.

- [ ] **Step 2: Run the focused CLI tests and verify RED**

Run: `go test ./internal/cli -run 'TestModelRefsFromConfig|TestBareModelOpensKeyboardPicker' -count=1`

Expected: the picker label assertion fails with `patty/medium` before the rendering change.

- [ ] **Step 3: Reuse the parsed picker parts for the visible label**

In `openModelPicker`, retain `ID: ref` and set `Label` to `parts[1]` when a provider/model split exists, falling back to `ref` for extension or malformed references whose visible identity cannot be shortened safely.

- [ ] **Step 4: Verify the CLI slice**

Run: `go test ./internal/cli -run 'TestModelRefsFromConfig|TestBareModelOpensKeyboardPicker|TestModelArgCompletion|TestMergeExtensionModelRefs' -count=1`

Expected: PASS, including extension-ref preservation.

- [ ] **Step 5: Commit the TUI slice**

```bash
git add internal/cli/model.go internal/cli/model_test.go
git commit -m "feat(cli): present medium as the stock model"
```

### Task 3: Shared compact-ratio editing surfaces

**Files:**
- Modify: `internal/cli/cli.go:2592-2686`
- Modify: `desktop/frontend/src/components/SettingsPanel.tsx:3353-3372,3707-3721`
- Modify: `desktop/frontend/src/locales/en.ts:1738`
- Modify: `desktop/frontend/src/__tests__/settings-refresh-snapshot.test.tsx:320-420`

**Interfaces:**
- Consumes: `Config.SetCompactRatio`, Wails `SetCompactRatio(ratio)` binding.
- Produces: CLI and desktop editors that accept 95.9% and document a 65–97% range with a 98% force boundary.

- [ ] **Step 1: Change the existing interaction test to submit 95.9% and verify RED**

Replace the valid custom input fixture `75` with `95.9`; assert the apply button enables and the recorded call is `0.959`. Update subsequent visible summary assertions to `95.9% · Custom`.

- [ ] **Step 2: Run the frontend test and verify RED**

Run from `desktop/frontend`: `pnpm exec tsx src/__tests__/settings-refresh-snapshot.test.tsx`

Expected: FAIL because the current UI rejects values above 85%.

- [ ] **Step 3: Widen the existing UI boundary and copy**

Change both validation and the number input's `max` from `85` to `97`. Update the hint to: `Set 65%–97%. Tool output is trimmed at 60%; 98% forces compaction. Press Enter to apply or Esc to cancel.` Update CLI usage from `65..85` to `65..97`.

- [ ] **Step 4: Verify frontend and CLI help behavior**

Run:

```bash
cd desktop/frontend && pnpm exec tsx src/__tests__/settings-refresh-snapshot.test.tsx
cd ../.. && go test ./internal/cli ./internal/config
```

Expected: PASS.

- [ ] **Step 5: Commit the editing-surface slice**

```bash
git add internal/cli/cli.go desktop/frontend/src/components/SettingsPanel.tsx desktop/frontend/src/locales/en.ts desktop/frontend/src/__tests__/settings-refresh-snapshot.test.tsx
git commit -m "feat(settings): support Patty compact threshold"
```

### Task 4: Activate the local config and verify the built artifact

**Files:**
- Modify: `/Users/patrickrho/.patty/config.toml`
- Rebuild: `bin/patcode`
- Refresh tracked artifact: `patcode`

**Interfaces:**
- Consumes: current user configuration and repository build/signing targets.
- Produces: a launchable signed binary whose first screen displays model `medium` and whose `/model` picker contains one visible item.

- [ ] **Step 1: Patch only the requested user-config fields**

Set `default_model = "patty/medium"`, replace the two stock DeepSeek provider blocks with the one Patty provider from Task 1, set `compact_ratio` to the exact rendered ratio, and set `compact_force_ratio = 0.98`. Preserve language, theme, permissions, tools, and every unrelated user choice.

- [ ] **Step 2: Run repository verification**

Run:

```bash
gofmt -w internal/config/config.go internal/config/defaults_test.go internal/config/edit.go internal/config/edit_test.go internal/cli/model.go internal/cli/model_test.go internal/cli/cli.go
go test ./...
go vet ./...
cd desktop && go test . && go vet .
cd frontend && pnpm typecheck && pnpm test && pnpm build
cd ../../site && pnpm test && pnpm build
git diff --check
```

Expected: PASS with no formatting or vet findings.

Also cross-build the CGO-free CLI for Linux and Windows so the stock-provider
change and the existing cross-platform input paths remain buildable.

- [ ] **Step 3: Commit the verified source**

Commit the source and test changes before building so the executable embeds a
stable revision rather than a dirty or pre-review snapshot.

- [ ] **Step 4: Build and sign the final reviewed source**

Run `make tracked-patcode` to emit and sign `bin/patcode`, refresh the tracked root `patcode`, and verify the two files are byte-identical and ad-hoc signed on macOS.

- [ ] **Step 5: Capture a real PTY launch and model picker**

Launch `./bin/patcode` under the existing TUI PTY/screenshot workflow. Assert visually and from plain capture that the status pill says `medium`, the missing-key banner is absent, and `/model` displays one visible `medium` entry. Exit cleanly with Ctrl-D.

- [ ] **Step 6: Run the live gateway smoke test**

Send a minimal OpenAI Chat Completions request to `https://omni.agents.patty.io/v1/chat/completions` with model `medium` and `AGENTS_PATTY_API_KEY`. Treat an upstream 502 independently from local config/build success and report its sanitized error.

Expected: authenticated HTTP 200 with a streamed response from the `medium` combo. If the gateway instead reports a transient upstream failure, preserve the sanitized response as separate evidence rather than conflating it with local config or build behavior.

- [ ] **Step 7: Commit the refreshed artifacts**

```bash
git add patcode
git commit -m "build: refresh Patty medium launcher"
```

- [ ] **Step 8: Final review and push**

Run the configured changed-file review workflow, fix actionable findings with fresh RED/GREEN cycles, rerun verification, then rebuild with `make tracked-patcode` after the final source commit. Stage the intended worktree, commit the refreshed binary, and push branch `pattycode`.
