# Plan 07: TUI Redesign and Launch Artwork

**Status:** Draft  
**Date:** 2026-08-08  
**Owner:** Implementation team  
**Dependencies:** Plan 05 (Korean) complete; Plan 06 (slash commands) complete  
**Gates:** G6 TUI/desktop  

## 1. Purpose

Redesign the Bubble Tea TUI from Patty Code to the "한지 작업대" aesthetic: terminal-native, Korean-first, restrained ink palette with 청·홍 accents. Add profile-sourced launch artwork renderer.

## 2. Scope

- TUI transcript display redesign
- Korean typography (cell width, IME compatibility, wrapping)
- No permanent sidebar navigation (conversation-first only)
- Contextual pickers for sessions, models, skills, settings
- Inline approvals and diffs in transcript flow
- Launch banner rendered from Profile.BannerArtwork

### Exclusions
- Desktop Wails-specific UI code (Plan 08)
- Governance inspector menu (not needed for engineer TUI)

## 3. Task List

### T1: Define visual direction tokens
- Palette from Profile.ColorPalette: hanji-workstation theme
- Font: terminal monospace with Korean support
- Spacing: wide enough for Korean text, no cramped 6-line wraps

### T2: Restructure TUI layout components
- Primary surface: conversation transcript (full-width)
- Input bar at bottom with Korean command palette trigger (`/`)
- Status line: model info, token count, trust status — contextual only
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

### T5: Launch artwork renderer
- Reads Profile.BannerArtworth.Path at startup
- Full-color variant for capable terminals
- Monochrome fallback for NO_COLOR / limited terminals
- Narrow-terminal variant (80 columns or less)
- Suppressed for: piped input, --quiet, non-interactive, CI, version checks

### T6: Responsive terminal behavior
- Wide (>120 col): full transcript with sidebar context
- Standard (80-120 col): compact transcript
- Narrow (<80 col): minimal interface, essential info only
- Resize during streaming handled gracefully
- Scrollback buffer maintained

### T7: Integration tests
- Golden snapshot tests for each supported width
- Interactive test: slash palette open/close/select
- Verify all TUI text uses localizable strings from plan-05 catalog

## 4. Definition of Done

- [ ] TUI uses 한지 작업대 visual identity
- [ ] Conversation is primary surface with no permanent sidebar
- [ ] Korean typography passes cell-width and wrapping tests
- [ ] Launch artwork renders correctly (or omitted in quiet mode)
- [ ] Smoke tests pass at all supported widths
- [ ] Gate G6 proof: Korean-first TUI smoke tests pass at supported widths/platforms