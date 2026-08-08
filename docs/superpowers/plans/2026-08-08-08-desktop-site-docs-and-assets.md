# Plan 08: Desktop, Site, Docs, and Assets

**Status:** Draft  
**Date:** 2026-08-08  
**Owner:** Implementation team  
**Dependencies:** Plan 03 (Go module rename) complete; Plan 05 (i18n) complete; Plan 07 (TUI) partially done  
**Gates:** G6 partial — desktop identity surface  

## 1. Purpose

Replace all Reasonix-specific UI text, assets, build metadata, and website content with Patty Code equivalents. Generate new icons/artwork from profile coordinates.

## 2. Scope

- Desktop frontend: component text, window titles, menu labels, branding
- Desktop build: macOS `.app` name, bundle ID placeholder, Linux `.desktop` file
- Public site (`site/`): page titles, meta tags, download links, community pages
- Release notes, embedded docs accessibility text
- Logo/icon/SVG regeneration from profile assets

### Exclusions
- Governance features on public site (out of scope for patty rebrand)
- Windows installer resource editing (Plan 09)

## 3. Task List

### T1: Rename `reasonix-example.toml` → `patty.example.toml`
- Update all example values to patty coordinates
- Remove Chinese locale examples

### T2: Rename `.reasonix/commands/review.md` → `.patty/commands/review.md`
- Update command-root discovery code to look in `.patty/commands/`

### T3: Update `REASONIX.md` → `PATTY.md`
- Rewrite as Patty project instructions
- Update CLAUDE.md cross-references if needed

### T4: Replace skill guide identity
- Move `internal/skill/builtincontent/reasonix-guide/` → `patty-guide/`
- Rewrite SKILL.md to reflect Patty, not Reasonix
- Update `patty_guide_test.go` expectations

### T5: Desktop frontend text replacement
- Replace Reasonix logo references with patty wordmark
- Update window/app title strings via Profile.DisplayName
- Replace language picker options (ko/en only)

### T6: Replace NPM package
- Move `npm/reasonix/` → `npm/patty/`
- Update `package.json` names, scopes, bin entries
- Update `bin/reasonix.js` → `bin/patty.js`

### T7: SDK examples
- Rename `sdk/go/examples/fullsidecar/reasonix-plugin.json` → `patty-plugin.json`
- Update SDK documentation references

### T8: Website overhaul
- Page titles: "Reasonix" → "Patty Code"
- Download links point to patty-artifact URLs
- Remove zh-CN SEO routing / sitemap entries
- Social card images regenerated

### T9: Accessibility audit
- All ARIA labels in Korean by default, English secondary
- No orphaned Reasonix strings in accessible text

## 4. Definition of Done

- [ ] `patty.example.toml` replaces reasonix.example.toml
- [ ] `PATTY.md` replaces REASONIX.md
- [ ] Skill guide renamed and rewritten
- [ ] NPM package uses patty coordinates
- [ ] SDK manifests updated
- [ ] Website text fully replaced
- [ ] Accessibility labels in Korean
- [ ] Gate G6 proof: desktop and TUI smoke tests pass at supported widths/platforms