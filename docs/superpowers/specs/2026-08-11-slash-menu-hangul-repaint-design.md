# Slash Menu Hangul Repaint Design

## Problem

The CLI slash-command menu contains the correct Korean labels `브랜치만들기` and
`브랜치전환`, but navigating the menu can intermittently leave the `치` glyph
blank on screen. The command registry, view model, and raw output retain the
glyph; the failure is in incremental terminal repainting around wide Hangul
cells.

## Decision

Disable Bubble Tea's backspace cursor-movement optimization for the Patty Code
CLI TUI. The current redraw stream uses backspace movements, and those are
unsafe when the renderer is moving across wide Hangul cells on some terminals.
The change preserves the existing cell-diff renderer and command labels while
using cursor-position/forward/backward escape sequences that do not depend on
terminal backspace behavior.

The change is intentionally scoped to the CLI TUI. It will not alter the
Korean command registry, the desktop frontend, or terminal configuration.

## Validation

- Add a regression test that exercises the completion view with Hangul labels
  across successive selection frames and verifies the labels remain intact.
- Run the focused CLI completion and width tests.
- Run the broader CLI test package.
- Build the arm64 CLI binary into `./bin/patcode` and verify its version/build
  metadata so it is directly testable by the user.

## Alternatives considered

- Patching/forking Ultraviolet's wide-cell diff algorithm: potentially more
  general, but expands dependency ownership for a terminal-specific symptom.
- Forcing a full-screen repaint whenever the menu changes: likely to remove the
  artifact, but adds flicker and unnecessary work.
- Changing or shortening Korean labels: rejected because the source strings
  are correct and this would mask the rendering defect.
