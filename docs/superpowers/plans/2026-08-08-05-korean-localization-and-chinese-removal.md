# Plan 05: Korean Localization and Chinese Removal

**Status:** Draft  
**Date:** 2026-08-08  
**Owner:** Implementation team  
**Dependencies:** Plan 03 (Go module rename) complete; Plan 04 (config isolation) partially done  
**Gates:** G4 Language  

## 1. Purpose

Make Korean the default and completeness source for all user-facing surfaces. Add complete English parity. Remove all Chinese locale resources and code paths after parity is verified.

## 2. Scope

- Go i18n catalog: `internal/i18n/messages_ko.go` (complete), `messages_en.go` → update scope
- Desktop frontend locales: Korean (`ko.ts`) and English (`en.ts`) catalogs
- Response/patty code language policy defaults to Korean
- CLI/TUI text replacement with localizable strings
- Chinese resource removal after parity check

### Exclusions
- User documentation translations (not part of product)
- Third-party/vendor string extraction

## 3. Task List

### T1: Create `internal/i18n/messages_ko.go` from messages_en.go + zh.go
- Copy all keys from existing English catalog as base
- Translate all values to Korean (canonical)
- Validate: every en key exists in ko catalog

### T2: Rewrite `messages_en.go` to be a secondary translation table
- Source of truth is now Korean
- En keys are auto-generated or manually maintained translations

### T3: Update locale detection and persistence
- Default locale changed from `"zh"` / `"auto"` to `"ko"`
- Locale preference persistence key changes from `patty_lang` to neutral
- Environment variable from `PATTY_LANG` to `PATTY_LANG`

### T4: Replace hardcoded UI strings in CLI/TUI code
- Audit: `rg 'say|tprintf|t.Errorf.*"' internal/cli/ | head -100`
- Replace each with localizable lookup function
- Korean-first resolution: `/resume` → displays `/이어하기` when locale=ko

### T5: Update desktop frontend locale system
- Create `desktop/frontend/src/locales/ko.ts` from scratch
- Update `en.ts` with current correct strings
- Remove `zh.ts`, `en-US.ts` files

### T6: Add completion-check tests
- Test that ko catalog has ≥ en catalog entry count
- Test that no raw hardcoded non-localized strings exist in critical paths
- Test IME composition compatibility

### T7: Delete Chinese resources (after parity pass)
- Delete `internal/i18n/messages_ko.go`, `messages_ko_tw.go`
- Delete `desktop/frontend/src/locales/en-US.ts`
- Delete 33+ `*.ko-KR.md` docs under `docs/`
- Delete `README.ko-KR.md`, `sdk/go/examples/starterextension/README.ko-KR.md`

### T8: Run forbidden-locale scan
- CI gate checks no Chinese resources remain
- Tests verify zero `zh` or `ko-KR` or `en-US` locale entries

## 4. Definition of Done

- [ ] Korean catalog complete and serves as source-of-truth
- [ ] English catalog maintains full parity
- [ ] All Chinese locale files deleted
- [ ] No hardcoded Chinese references in i18n code
- [ ] Forbidden-locale scan passes in CI
- [ ] Gate G4 proof: Korean completeness and English parity pass; Chinese resources absent