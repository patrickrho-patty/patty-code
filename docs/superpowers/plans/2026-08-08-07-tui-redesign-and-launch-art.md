# Plan 07: TUI Redesign and Launch Artwork

**Status:** Draft  
**Date:** 2026-08-08  
**Owner:** Implementation team  
**Dependencies:** Plan 05 (Korean) complete; Plan 06 (slash commands) complete  
**Gates:** G6 TUI/desktop  

## 1. Purpose

Redesign the Bubble Tea TUI around the approved Patty Code launch composition: terminal-native, Korean-first, restrained ink palette with 청·홍 accents, strong panel edges, and clean input hierarchy. Add the paired Taegeukgi and Patty launch artwork.

## 2. Scope

- TUI transcript display redesign
- Korean typography (cell width, IME compatibility, wrapping)
- No permanent sidebar navigation (conversation-first only)
- Contextual pickers for sessions, models, skills, settings
- Inline approvals and diffs in transcript flow
- Static bordered `Patty Code` titlebar with a separate instrument strip below it
- Centered paired Taegeukgi and Patty launch artwork rendered from `Profile.BannerArtwork`
- Bounded startup stage that grows naturally with the conversation instead of filling an otherwise empty terminal

### Exclusions
- Desktop Wails-specific UI code (Plan 08)
- Governance inspector menu (not needed for engineer TUI)

## 3. Task List

### T1: Define visual direction tokens
- Palette from Profile.ColorPalette: hanji-workstation theme
- Font: terminal monospace with Korean support
- Spacing: wide enough for Korean text, no cramped 6-line wraps
- Border tokens must remain legible in dark, light, reduced-color, and `NO_COLOR` modes
- Themes may change color tokens without changing the approved information architecture

### T2: Restructure TUI layout components
- Primary surface: conversation transcript (full-width)
- First row: static, fully bordered `Patty Code` titlebar
- Second row: individually bounded status instruments such as 작업, 모델, 추론, and 여유 (localized as RUN, MODEL, DEPTH, and BUDGET in English)
- Input surface: a rounded full rectangle with a distinct background, internal padding, a visible insertion cursor, and no leading `>` prompt
- Placeholder and `/명령어 · @파일 · !셸 · ?단축키` hints are visually subordinate to typed text
- Mode/help instruction sits below the input with breathing room; `일반` replaces `물어보기`, and status/context metadata shares that line rather than creating a stray extra row
- Remove: permanent left sidebar, file tree, changed-files inspector

### T3: Implement contextual pickers
- `/` opens searchable overlay (session resume, settings, models, etc.)
- Picker uses Korean keyword index
- Arrow-key navigation + Enter selection
- Closes on Escape

### T4: Korean typography compliance
- Terminal cell width measurement for Hangul syllables (double-width)
- Truncation uses ellipsis that works with double-width chars
- Screen reader labels in Korean
- Grapheme-aware cursor motion and deletion work for precomposed Hangul, decomposed jamo, emoji, and combining marks
- macOS Kitty requests associated-text keyboard reporting so a physical Backspace remains distinguishable from intentional standalone jamo input

### T5: Launch artwork renderer
- Reads `Profile.BannerArtwork.Path` at startup
- Renders the Taegeukgi and Patty marks as one centered group, aligned both horizontally and vertically within the bounded launch stage
- Preserves the approved Taegeukgi proportions and centered taegeuk; Patty's three bars remain vertically centered inside its frame
- Full-color variant for capable terminals
- Monochrome fallback for NO_COLOR / limited terminals
- Narrow-terminal variant (80 columns or less)
- Suppressed for: piped input, --quiet, non-interactive, CI, version checks

### T6: Responsive terminal behavior
- Wide (>120 col): full transcript with centered launch group and no sidebar
- Standard (80-120 col): compact transcript
- Narrow (<80 col): minimal interface, essential info only
- Empty startup uses a bounded stage (currently capped at 34 rows) rather than the terminal's full height; active conversation may grow to the available height
- Resize during streaming handled gracefully
- Scrollback buffer maintained

### T7: Integration tests
- Golden snapshot tests for each supported width
- Interactive test: slash palette open/close/select
- PTY replay tests cover Kitty enhanced-key sequences and preserve deliberate standalone jamo
- Verify all TUI text uses localizable strings from plan-05 catalog

## 4. Definition of Done

- [ ] TUI uses 한지 작업대 visual identity
- [ ] Conversation is primary surface with no permanent sidebar
- [ ] Korean typography passes cell-width and wrapping tests
- [ ] Launch artwork renders correctly (or omitted in quiet mode)
- [ ] Smoke tests pass at all supported widths
- [ ] Gate G6 proof: Korean-first TUI smoke tests pass at supported widths/platforms
