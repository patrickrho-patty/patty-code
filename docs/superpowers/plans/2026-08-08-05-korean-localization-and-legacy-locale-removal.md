# Plan 05: Korean Localization and Legacy Locale Removal

**Status:** Draft  
**Date:** 2026-08-08  
**Owner:** Implementation team  
**Dependencies:** Plan 03 (Go module rename) complete; Plan 04 (config isolation) partially done  
**Gates:** G4 Language  

## 1. Purpose

Make Korean the default and completeness source for all user-facing surfaces. Add complete English parity. Remove retired locale resources and code paths after parity is verified.

## 2. Scope

- Go i18n catalogs: keep `internal/i18n/messages_ko.go` as the canonical catalog and `messages_en.go` as the English parity table
- Desktop frontend locales: Korean (`ko.ts`) and English (`en.ts`) catalogs
- Response language policy defaults to Korean across Patty Code surfaces
- CLI/TUI text replacement with localizable strings
- Retired locale resource removal after parity checks

### Exclusions
- User documentation translations (not part of product)
- Third-party/vendor string extraction

## 3. Task List

### T1: Maintain `internal/i18n/messages_ko.go` as the canonical catalog
- Keep every typed message key in the Korean catalog
- Translate every user-facing value into Korean
- Validate: every English key exists in the Korean catalog

### T2: Maintain `messages_en.go` as the secondary translation table
- Korean remains the source of truth
- English keys are auto-generated or manually maintained parity translations

### T3: Update locale detection and persistence
- Default locale resolves to `"ko"`; English requires an explicit opt-in
- Locale preference persistence key changes from `patty_lang` to neutral
- `PATTY_LANG` remains the explicit per-run locale override

### T4: Replace hardcoded UI strings in CLI/TUI code
- Audit: `rg 'say|tprintf|t.Errorf.*"' internal/cli/ | head -100`
- Replace each with localizable lookup function
- Korean-first resolution: `/resume` → displays `/이어하기` when locale=ko

### T5: Update desktop frontend locale system
- Maintain `desktop/frontend/src/locales/ko.ts` as the primary desktop catalog
- Keep `en.ts` aligned with the current Korean strings
- Remove retired locale files after Korean and English parity passes

### T6: Add completion-check tests
- Test that ko catalog has ≥ en catalog entry count
- Test that no raw hardcoded non-localized strings exist in critical paths
- Test IME composition compatibility

### T7: Delete retired locale resources (after parity pass)
- Delete retired Go locale catalogs and aliases
- Delete retired desktop locale catalogs and aliases
- Delete retired documentation translations while preserving Korean and English docs

### T8: Run forbidden-locale scan
- CI gate checks no legacy locale resources remain
- Tests verify zero retired locale aliases or resources

## 4. Definition of Done

- [ ] Korean catalog is complete and serves as the source of truth
- [ ] English catalog maintains full parity
- [ ] All retired locale files deleted
- [ ] No hardcoded retired locale references in i18n code
- [ ] Forbidden-locale scan passes in CI
- [ ] Gate G4 proof: Korean completeness and English parity pass; retired locale resources absent
