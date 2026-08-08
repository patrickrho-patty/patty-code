# Plan 12: Release Qualification and Master Plan Reconciliation

**Status:** Draft  
**Date:** 2026-08-08  
**Owner:** Implementation team  
**Dependencies:** All prior plans complete (G0-G9)  
**Gates:** G10 Qualification  

## 1. Purpose

Run the final whole-tree audit across source, generated artifacts, binaries, packages, databases, locale coverage, legal notices, and reconciled master plan references. Produce the evidence bundle for release qualification.

## 2. Scope

- Clean-build from scratch → verify all artifacts produce correct identities
- Binary inspection via `strings`, archive member listing, XML parsing of SVGs
- Locale completeness: ko ≥ en ≥ 0 zh resources
- Brand gate: zero unauthorized first-party identity families
- Legal notice placement review (§14)
- Transitional document sanitation (plans/docs in .superpowers/)
- GongCode master-plan reconciliation (§17)

## 3. Task List

### T1: Clean build and artifact inspection
```bash
# Build everything fresh
rm -rf dist/
go build ./cmd/patty/ && echo "patty binary OK"
go build ./cmd/patty-launcher/ && echo "launcher binary OK"

# Inspect binary strings
strings cmd/patty/goos-darwin_goarch-arm64/Patty\ Code.app/Contents/MacOS/Patty\ Code | grep -i 'reasonix' > /tmp/final_reasonix_strings.txt

# Check NPM package content
cd npm/patty && npm pack --dry-run && cd ../../
```

### T2: Brand gate scanner
Scan entire tree including generated files:
```bash
# Primary forbidden family scan
rg -i 'reasonix|esengine|REASONIX_|\.reasonix' \
    --hidden --no-ignore --glob '!.git/**' \
    --glob '!THIRD_PARTY_NOTICES.md' \
    --glob '!docs/superpowers/**/*.md' \
    --output=file:line | tee /tmp/brand_scan_result.txt

# Expected results: only THIRD_PARTY_NOTICES.md, Git history, and .superpowers/ plans
```

### T3: Locale completeness test
```bash
# Korean catalog has ≥ English entries
ko_count=$(grep -c '^"' internal/i18n/messages_ko.go 2>/dev/null || echo 0)
en_count=$(grep -c '^"' internal/i18n/messages_en.go 2>/dev/null || echo 0)
echo "ko=$ko_count en=$en_count" # ko >= en required

# No Chinese resources remain
zh_files=$(find . -name '*zh*' -not -path '*/node_modules/*' -not -path '*/.git/*' | wc -l)
# zh_files == 0 required
```

### T4: Legal notice review
- Create `THIRD_PARTY_NOTICES.md` with upstream MIT copyright
- Verify root LICENSE is Patty Code copyright
- Confirm runtime UI does not display Reasonix brand
- Git history retained but not present in distributable tree

### T5: Sanitize transitional documents
- Plans in `docs/superpowers/plans/` use neutral terms ("upstream project")
- Or move plans to non-distributable location (.pi/, docs/.internal/)
- Remove any remaining "Reasonix analog of..." phrasing from PATTY.md

### T6: Reconcile with GongCode Master Plan
- Review §17 decisions: control plane separation, closed-network operation
- Update master plan if product profile architecture supersedes earlier drafts
- Ensure mandatory modules align with governance requirements

### T7: Produce evidence bundle
Collect:
- Brand scan output (`brand-audit.log`)
- Test output (`tests_passed.jsonl`)
- Generated profile manifest + digest
- Package content inventory
- Before/after behavior comparison notes
- Known exceptions table with owners/expires

## 4. Definition of Done

- [ ] Clean build produces artifacts with correct Patty/GongCode identities
- [ ] Brand scanner returns only legal exception and .superpowers/ paths
- [ ] Korean catalog covers ≥ all English keys; zero Chinese resources
- [ ] License file is Patty; notices are in THIRD_PARTY_NOTICES.md
- [ ] Transitional documents sanitized or non-distributable
- [ ] Master plan reconciled with new profile architecture
- [ ] Evidence bundle produced
- [ ] Gate G10 proof: whole-tree, generated, binary, package, install, locale, legal, and master-plan gates pass