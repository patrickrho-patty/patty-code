# PAT-1348 Seoul Flow-Board TUI Re-Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the Reasonix-like terminal chrome with the approved Korean-first Seoul flow-board, exact Taegeukgi and Patty ASCII marks, borderless composer, timeline transcript, and selectable semantic theme templates.

**Architecture:** Keep Bubble Tea, the existing transcript-source/reflow model, and the custom-statusline contract. Add a focused `tui_chrome.go` renderer for launch artwork, a persistent top instrument strip, and composer chrome. Artwork remains launch-only in transcript scrollback. Live session facts render above the transcript viewport in alternate-screen mode and above the pinned bottom rail in native-scrollback mode. Extend the existing CLI palette system into full semantic templates and keep legacy theme IDs as compatibility aliases.

**Tech Stack:** Go, Bubble Tea v2, Bubbles textarea/viewport, Lip Gloss v2, typed `internal/i18n` catalogs, TOML config persistence, Go unit/golden-style rendering tests.

## Global Constraints

- Korean is the default and canonical locale; English remains complete.
- English uses `MODEL`, never `ENGINE`.
- Korean composer copy is exactly `명령 / 메시지`, `명령 또는 질문을 입력하세요`, `/ 명령어`, `@ 파일`, `! 셸`, `? 단축키`.
- English composer copy is exactly `COMMAND / MESSAGE`, `Type a command or ask a question`, `/ commands`, `@ files`, `! shell`, `? shortcuts`.
- The input has no chevron and no top, bottom, or side box border.
- The old left-mode/right-model-and-effort footer arrangement must disappear.
- Full artwork uses the approved 31×16 Taegeukgi and 31×16 vertically centered Patty mark; limited-color, monochrome, and narrow fallbacks remain readable.
- Artwork remains interactive-launch-only and does not leak into piped, JSON, quiet, version, or other non-interactive output.
- Theme changes alter semantic color tokens, not layout geometry.
- Preserve chat input, Korean IME cursor placement, text selection, streaming, approvals, tool activity, custom statusline, native scrollback, and resize behavior.
- Work directly on branch `pattycode` as requested. Do not revert or stage unrelated worktree changes. Because target files already contain user-owned rebrand edits, use verification checkpoints instead of commits that would capture overlapping work.

---

### Task 1: Lock the Korean/English Chrome Contract

**Files:**
- Modify: `internal/i18n/i18n.go`
- Modify: `internal/i18n/messages_ko.go`
- Modify: `internal/i18n/messages_en.go`
- Test: `internal/i18n/i18n_test.go`

**Interfaces:**
- Produces: `Messages.ChatComposerTitle`, `ChatComposerPlaceholder`, four composer hint fields, `ChatStatusModeLabel`, `ChatStatusHeadroomLabel`, and localized mode/effort display values.
- Consumes: existing `DetectLanguage`, `CurrentLanguage`, catalog completeness, placeholder parity, and code-token parity contracts.

- [x] **Step 1: Write failing catalog tests**

```go
func TestSeoulFlowChromeCopy(t *testing.T) {
	if Korean.ChatComposerTitle != "명령 / 메시지" || Korean.ChatStatusModelLabel != "모델" {
		t.Fatalf("Korean Seoul flow chrome is not canonical: %+v", Korean)
	}
	if English.ChatComposerTitle != "COMMAND / MESSAGE" || English.ChatStatusModelLabel != "MODEL" {
		t.Fatalf("English Seoul flow chrome is not canonical: %+v", English)
	}
	if strings.Contains(English.ChatStatusModelLabel, "ENGINE") {
		t.Fatal("English model label must never use ENGINE")
	}
}
```

- [x] **Step 2: Run the focused test and confirm RED**

Run: `go test ./internal/i18n -run 'TestSeoulFlowChromeCopy|TestCatalogsComplete|TestCatalogsAgree'`

Expected: compile failure because the new typed fields do not exist.

- [x] **Step 3: Add the typed fields and exact Korean/English values**

Add fields beside the existing chat status catalog fields; populate both catalogs. Mode display values include Ask/Auto/Plan/Goal/Shell combinations, and effort display values cover auto/low/medium/high/xhigh/max without changing stable config tokens.

- [x] **Step 4: Run the focused catalog suite and confirm GREEN**

Run: `go test ./internal/i18n`

Expected: PASS with catalog completeness, placeholder shape, and code-token parity intact.

- [x] **Step 5: Checkpoint**

Run: `git diff --check -- internal/i18n`

Expected: no whitespace errors.

### Task 2: Replace Accent Skins with Semantic Theme Templates

**Files:**
- Modify: `internal/cli/theme.go`
- Modify: `internal/cli/theme_test.go`
- Modify: `internal/config/config.go`
- Modify: `internal/config/render.go`
- Modify: `internal/config/edit_test.go`
- Modify: `internal/config/render_test.go`

**Interfaces:**
- Produces: canonical theme IDs `seoul-night`, `ink-night`, `hanji-light`, `jade-night`; full `cliPalette` token sets; legacy alias resolution; persisted `ui.theme_style`.
- Consumes: `resolveCLIThemeWithStyle`, `setCLIThemeStyle`, `persistTheme`, `normalizeThemeStyle`, environment overrides, and `/theme` completion.

- [x] **Step 1: Write failing theme-template tests**

```go
func TestSeoulFlowThemeTemplatesChangeSemanticPaletteNotGeometry(t *testing.T) {
	for _, name := range []string{"seoul-night", "ink-night", "hanji-light", "jade-night"} {
		p, ok := cliThemeByName(name)
		if !ok { t.Fatalf("missing theme %q", name) }
		if p.style != name || p.background.hex == "" || p.strong.hex == "" || p.signal.hex == "" {
			t.Fatalf("incomplete semantic theme %q: %+v", name, p)
		}
	}
}
```

Add config cases proving new IDs normalize and persist; add compatibility cases proving legacy IDs still resolve without becoming advertised canonical choices.

- [x] **Step 2: Run focused tests and confirm RED**

Run: `go test ./internal/cli ./internal/config -run 'Theme|SeoulFlow'`

Expected: compile/assertion failures for missing semantic tokens and IDs.

- [x] **Step 3: Implement full semantic templates**

Extend `cliPalette` with `background`, `surface`, `composer`, `strong`, and `signal`. Define the four approved palettes from the companion mockup. Make Seoul Night the dark/default template, Hanji Light the light default, and keep existing operational success/warn/error/diff/tool tokens coherent per template.

- [x] **Step 4: Add legacy aliases without duplicating templates**

Resolve old IDs such as `graphite`, `sandstone`, `aurora`, and `glacier` through an alias map. `/theme` descriptions and completions advertise only the four canonical templates plus `auto|light|dark`.

- [x] **Step 5: Keep persistence and environment overrides stable**

Update config normalization/comments so `PATTY_THEME`, `PATTY_THEME_STYLE`, `[ui].theme`, and `[ui].theme_style` accept the new IDs. Runtime access resolves legacy aliases to canonical templates while rendered TOML preserves accepted user spelling for stable round trips.

- [x] **Step 6: Run focused tests and confirm GREEN**

Run: `go test ./internal/cli ./internal/config -run 'Theme|SeoulFlow'`

Expected: PASS; ANSI256/truecolor/NO_COLOR behavior and theme sweep safety remain intact.

- [x] **Step 7: Checkpoint**

Run: `git diff --check -- internal/cli/theme.go internal/cli/theme_test.go internal/config`

Expected: no whitespace errors.

### Task 3: Build the Approved Launch Masthead and Live Fact Strip

**Files:**
- Create: `internal/cli/tui_chrome.go`
- Create: `internal/cli/tui_chrome_test.go`
- Modify: `internal/cli/chat_tui.go`
- Modify: `internal/cli/transcript.go`
- Modify: `internal/cli/launch_banner_test.go`
- Modify: `products/patty/assets/banner.txt`

**Interfaces:**
- Produces: `renderLaunchMasthead(m chatTUI, missing string, width int) string`, `renderLaunchArtwork(width int) string`, `renderSessionHeader(m chatTUI, width int) string`, `renderSessionFacts(m chatTUI, width int) string`, `refreshLaunchMasthead()`.
- Consumes: `transcriptSourceBanner`, `reflowTranscript`, `modeTagText`, controller context snapshot, effort/model fields, theme semantic tokens, and `visibleWidth`.

- [x] **Step 1: Write failing exact-artwork tests**

Assert the ANSI-stripped full-width artwork equals the approved 31×16 marks, including the Taegeuk center at column 15, right trigram inset, and Patty mark with three blank rows above and below. Assert every line fits the supplied terminal width.

- [x] **Step 2: Write failing responsive/fallback tests**

Cover full side-by-side, narrow stacked/compact, ANSI256, and NO_COLOR output. NO_COLOR must retain recognizable artwork with no escape sequences. Widths below the full-art threshold must never emit an over-wide line.

- [x] **Step 3: Write failing fact-strip tests**

```go
func TestSessionFactsAreKoreanFirstAndCoLocated(t *testing.T) {
	i18n.DetectLanguage("ko")
	m := newTestChatTUIWithController()
	m.label, m.effortLevel = "deepseek-v4", "medium"
	plain := ansi.Strip(renderSessionFacts(m, 100))
	for _, want := range []string{"작업", "자동", "모델", "deepseek-v4", "추론", "보통", "여유"} {
		if !strings.Contains(plain, want) { t.Fatalf("facts missing %q:\n%s", want, plain) }
	}
}
```

Add an English case requiring `MODE`, `MODEL`, `EFFORT`, `HEADROOM` and rejecting `ENGINE`.

- [x] **Step 4: Run masthead tests and confirm RED**

Run: `go test ./internal/cli -run 'Launch|Masthead|SessionFacts|Artwork'`

Expected: failures because the current 6-row placeholder artwork and banner API do not satisfy the contract.

- [x] **Step 5: Implement the focused chrome renderer**

Move launch-specific rendering out of `chat_tui.go`. Render exact artwork from immutable row data, apply red/blue/foreground/accent semantic colors, compose the localized persistent brand/fact strip, and hard-wrap only the missing-configuration warning—not the artwork grid.

- [x] **Step 6: Pin live facts without pinning the large artwork**

Keep artwork as the first launch transcript source per the platform spec. Render brand, mode, model, effort, and context headroom in a separate top region on every frame, reserving its exact height above the viewport and translating mouse/cursor coordinates around it. Native scrollback and alternate-screen paths use the same header renderer; extremely constrained terminals may yield the header to preserve a two-row editor and readable transcript. Live fact changes must not rewrite launch scrollback.

- [x] **Step 7: Update the embedded Patty artwork**

Replace the stale seven-row `products/patty/assets/banner.txt` with the approved monochrome 31×16 pair so profile consumers and the CLI renderer share the same visible identity contract.

- [x] **Step 8: Run focused tests and confirm GREEN**

Run: `go test ./internal/cli ./products/patty/assets -run 'Launch|Masthead|SessionFacts|Artwork|Banner'`

Expected: PASS for full, narrow, monochrome, locale, reflow, persistent-header, and live-fact cases.

- [x] **Step 9: Checkpoint**

Run: `git diff --check -- internal/cli products/patty/assets`

Expected: no whitespace errors.

### Task 4: Replace the Bordered Chevron Composer with Borderless Korean Chrome

**Files:**
- Modify: `internal/cli/chat_tui.go`
- Modify: `internal/cli/composer_selection.go`
- Modify: `internal/cli/theme.go`
- Modify: `internal/cli/chat_tui_test.go`
- Modify: `internal/cli/composer_selection_test.go`
- Test: `internal/cli/tui_chrome_test.go`

**Interfaces:**
- Produces: `renderComposerChrome(m chatTUI, width int) string`; blank fixed-width textarea prompt gutter; correct terminal cursor offset.
- Consumes: `configureChatTextarea`, `renderComposerInput`, selection range offsets, `bottomRows`, `inputHeightLimit`, modal composer ownership, and textarea theme refresh.

- [x] **Step 1: Replace prompt/border assertions with failing borderless tests**

Assert that the first empty input row contains the localized placeholder and no `❯` or `>`; the complete composer contains localized heading/hints; `composerInputStyle` has no borders in every theme/NO_COLOR mode.

- [x] **Step 2: Add cursor, CJK, wrap, selection, and height regression cases**

The textarea keeps a blank prompt gutter so continuation rows, CJK cursor columns, mouse selection, and detached composer rendering stay aligned. The heading and hint rows replace the former two border rows, leaving `bottomRows()` arithmetic stable.

- [x] **Step 3: Run focused composer tests and confirm RED**

Run: `go test ./internal/cli -run 'Composer|InputHeight|TranscriptViewportSizing|RenderedHeight'`

Expected: failures showing the old chevron and border contract.

- [x] **Step 4: Implement borderless composer chrome**

Make `configureChatTextarea` emit spaces, never a chevron. Use the localized default placeholder outside chooser typing. Render heading + textarea + hints with semantic signal/muted/strong tokens and no enclosing border; keep shell mode as a signal-color change rather than a box-border change.

- [x] **Step 5: Correct cursor and selection offsets**

Update only the offsets proven by the new tests. Preserve the real terminal cursor and IME anchor, including native scrollback and alt-screen paths.

- [x] **Step 6: Run focused tests and confirm GREEN**

Run: `go test ./internal/cli -run 'Composer|InputHeight|TranscriptViewportSizing|RenderedHeight|Selection'`

Expected: PASS at Korean/CJK and narrow widths.

- [x] **Step 7: Checkpoint**

Run: `git diff --check -- internal/cli`

Expected: no whitespace errors.

### Task 5: Convert Transcript and Footer into the Seoul Flow Layout

**Files:**
- Modify: `internal/cli/transcript.go`
- Modify: `internal/cli/status_footer.go`
- Modify: `internal/cli/chat_tui.go`
- Modify: `internal/cli/transcript_test.go`
- Modify: `internal/cli/chat_render_test.go`
- Modify: `internal/cli/status_footer_test.go`
- Modify: `internal/cli/chat_tui_test.go`

**Interfaces:**
- Produces: timeline-style user/assistant identities and a runtime-only footer with no persistent mode/model/effort group.
- Consumes: semantic transcript sources, turn receipts, tool cards, custom statusline output, Git/telemetry packing, status height accounting, resize anchors, and clipboard math markers.

- [x] **Step 1: Write failing timeline transcript tests**

Require user turns to begin with a localized `●` identity and assistant turns with `◆ Patty Code`, both sharing a quiet vertical flow gutter. Reject `›` and card/background treatment. Verify replay and copy-math reconstruction remain exact.

- [x] **Step 2: Write failing footer signature tests**

Require the persistent footer to omit mode, model, and effort in Korean and English. Preserve current transient state, shortcut help, Git, cache/context/jobs/balance telemetry, custom statusline replacement, wrapping, and height accounting.

- [x] **Step 3: Run transcript/footer tests and confirm RED**

Run: `go test ./internal/cli -run 'Assistant|Replay|UserBubble|Timeline|StatusFooter|StatusLine'`

Expected: failures because user turns still use a chevron and model/effort remain right-anchored.

- [x] **Step 4: Implement the timeline rendering**

Use a shared two-cell/rail gutter for semantic turn blocks. Keep markdown width reduction, theme-aware identity colors, plan-mode labeling, wrapping, and clipboard reconstruction aligned.

- [x] **Step 5: Remove the old split status signature**

Stop composing `statusModelWorkGroup` into `renderStatusBlock`; remove or retire dead right-alignment helpers only when literal caller search confirms no remaining use. Keep transient state and optional telemetry bands compact and left-flowing.

- [x] **Step 6: Reconcile height and resize behavior**

Update `computeStatusLineCount`, `bottomRows`, and viewport tests against the rendered output. Every tested width must produce exactly the terminal height and no row wider than the terminal.

- [x] **Step 7: Run focused tests and confirm GREEN**

Run: `go test ./internal/cli -run 'Assistant|Replay|UserBubble|Timeline|StatusFooter|StatusLine|Viewport|Resize'`

Expected: PASS with no old `›`, bordered composer, or split status arrangement.

- [x] **Step 8: Checkpoint**

Run: `git diff --check -- internal/cli`

Expected: no whitespace errors.

### Task 6: Integration, Compatibility, and Proof

**Files:**
- Modify as required by failures only: files already listed above
- Update: `docs/superpowers/plans/2026-08-10-pat-1348-seoul-flow-board.md` checkboxes/results

**Interfaces:**
- Consumes: all prior tasks.
- Produces: evidence that PAT-1348 is implemented without unrelated regressions.

- [x] **Step 1: Run focused package suites**

Run: `go test ./internal/i18n ./internal/config ./internal/control ./internal/cli ./products/patty/assets`

Expected: PASS.

- [x] **Step 2: Run race-sensitive TUI tests**

Run: `go test -race ./internal/cli`

Expected: PASS.

- [x] **Step 3: Run repository tests**

Run: `go test ./...`

Expected: PASS, or report clearly separated pre-existing failures with exact commands/output.

- [x] **Step 4: Run static/build checks**

Run: `go vet ./...`

Run: `go build ./...`

Expected: PASS.

- [x] **Step 5: Run visual contract scan**

Run: `rg -n '❯|›|ENGINE|inputBoxStyle|statusModelWorkGroup|layoutStatusSides|composerBorderRows' internal/cli internal/i18n`

Expected: no production rendering of the rejected signatures; test-only negative assertions/comments are allowed.

- [x] **Step 6: Inspect the final scoped diff**

Run: `git diff --check -- <PAT-1348 file list>` and `git diff -- internal/cli internal/i18n internal/config internal/control products/patty/assets docs/superpowers/plans/2026-08-10-pat-1348-seoul-flow-board.md`

Expected: only PAT-1348 changes layered on top of the existing rebrand; no unrelated files reverted.

- [x] **Step 7: Record verification evidence**

Append the exact passing commands and any environmental limitations to this plan. Do not mark a checkbox complete without the corresponding command output.

## Self-Review

- Spec coverage: Korean/English copy, exact artwork, responsive/NO_COLOR behavior, semantic themes and persistence, borderless composer, persistent top fact strip, timeline transcript, footer relocation, native scrollback, IME, custom statusline, and verification each map to an explicit task.
- Placeholder scan: no `TBD`, `TODO`, “similar to,” or unspecified error-handling steps remain.
- Type consistency: the chrome renderer consumes the existing `chatTUI`; catalog field names are consistent across Tasks 1, 3, and 4; canonical theme IDs are consistent across CLI/config/tests; `transcriptSourceBanner` owns launch-only artwork, `transcriptSourceReplayHistory` independently owns replayed turns, and the persistent header reads live session state directly.

## Completion Record — 2026-08-10

- Implemented directly on the shared `pattycode` worktree without staging, committing, or reverting unrelated rebrand work.
- The embedded banner is the single artwork source. The CLI parses that asset into responsive side-by-side, stacked, and compact forms; color styling is applied by contiguous runs and is omitted under `NO_COLOR`.
- Launch artwork and replay history are independent semantic transcript sources. Live facts render in persistent top chrome without mutating either source, while the accepted artwork remains launch-only.
- Korean is the product default. English is an explicit locale selection and consistently uses `MODEL`; setup language choices compare normalized language tags.
- Theme descriptors and aliases are centralized in `internal/config`; CLI palettes and slash completion consume the shared registry. Runtime access canonicalizes legacy aliases while TOML round trips preserve accepted user spelling.
- The composer has a blank prompt gutter, localized heading/placeholder/hints, no surrounding border, exact height accounting, and mouse coverage for wrapped hint rows.
- User and assistant messages use a width-safe vertical timeline. The persistent footer contains operational state and telemetry but no old split mode/model/effort signature.

Verification evidence:

- `go test ./internal/i18n ./internal/config ./internal/control ./internal/cli ./products/patty/assets` — PASS.
- `go test ./... -count=1` — PASS.
- `go test -race ./internal/cli -count=1` — PASS (`34.894s`).
- `go vet ./...` — PASS.
- `go build ./...` — PASS.
- Focused launch/replay/composer and native-scrollback persistent-header regression tests — PASS.
- Rejected-signature scan found chevrons only in chooser, picker, and completion affordances plus negative tests; the chat composer and timeline do not render them.
- The repository-wide `git diff --check` reports one existing trailing-space change in `internal/control/controller.go`, outside PAT-1348. The PAT-1348 scoped file list passes whitespace validation.
