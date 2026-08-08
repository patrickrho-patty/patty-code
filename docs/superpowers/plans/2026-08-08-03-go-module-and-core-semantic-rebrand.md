# Plan 03: Go Module and Core Semantic Rebrand

**Status:** Draft  
**Date:** 2026-08-08  
**Owner:** Implementation team  
**Dependencies:** Plan 02 complete (G1 gate); Plan 01 baseline ready  
**Gates:** G2 Core identity  

## 1. Purpose

Rename Go root module from `reasonix` to `patty`, move all `cmd/reasonix*` entries, neutralize shared symbols that reference Reasonix, and update URNs/schema/built-in identifiers — all in one controlled compile-green change.

## 2. Scope

- Root Go module rename (`go.mod`, goimports everywhere)
- `cmd/reasonix/*` → `cmd/patty/*`
- `cmd/reasonix-launcher/*` → `cmd/patty-launcher/*`
- `cmd/reasonix-legacy-migrator/*` → DELETE (clean break)
- `cmd/reasonix-plugin-example/*` → `cmd/patty-plugin-example/*`
- All import paths updated across entire codebase
- Shared symbol renaming: `reasonix.` → `patty.` where product-specific

### Exclusions
- i18n/catalog changes (Plan 05)
- TUI redesign (Plan 07)
- Desktop frontend UI text changes (Plan 08)
- Packaging/release names (Plan 09)
- GongCode profile (Plan 11)

## 3. Exact File Moves Required (§13.3)

| Current | New | Action |
|---|---|---|
| `cmd/reasonix/main.go` | `cmd/patty/main.go` | Move + module path update |
| `cmd/reasonix/main_test.go` | `cmd/patty/main_test.go` | Move + identity assertion update |
| `cmd/reasonix-launcher/main.go` | `cmd/patty-launcher/main.go` | Move + profile name source |
| `cmd/reasonix-launcher/main_test.go` | `cmd/patty-launcher/main_test.go` | Move + profile name test |
| `cmd/reasonix-legacy-migrator/` | *(delete)* | Remove entirely |
| `cmd/reasonix-plugin-example/` | `cmd/patty-plugin-example/` | Move + manifest update |
| `internal/skill/builtincontent/reasonix-guide/` | `internal/skill/builtincontent/patty-guide/` | Move + rewrite SKILL.md |
| `npm/reasonix/` | `npm/patty/` | Replace directory |

## 4. Task List

### T1: Update go.mod
- Change `module reasonix` → `module patty`
- Verify all internal imports resolve after change
- Expected: `go mod tidy` succeeds, `go build ./...` compiles

### T2: Rename cmd directories
- `mv cmd/reasonix cmd/patty`
- `mv cmd/reasonix-launcher cmd/patty-launcher`  
- `rm -rf cmd/reasonix-legacy-migrator`
- `mv cmd/reasonix-plugin-example cmd/patty-plugin-example`
- Update all internal import paths referencing these packages

### T3: Update all Go import paths
- Search: `import "reasonix/...` → `import "patty/...`
- Use semantic search for indirect references
- Run `go mod tidy` after each batch; verify compile green

### T4: Neutralize shared symbols
- `ReasonixConfig` → `HarnessConfig` or `ProfileConfig`
- `ReasonixHome` → `HarnessHomeDir` or `ProductHome`
- `REASONIX_HOME` → env-neutral constants resolved via Profile
- Every symbol change verified by tests

### T5: Update URNs, schemas, built-ins
- Schema field renames where Reasonix-named
- Built-in command IDs moved to Patty namespace
- Test every schema migration still functions

### T6: Update examples and SDK
- NPM package `npm/patty/package.json`
- SDK example manifests updated to patty coordinates
- Test: example compiles and loads

## 5. Commands

```bash
# Verify current build
go build ./cmd/reasonix/ && echo "BUILD OK"

# After module rename
go mod tidy && go build ./cmd/patty/ && echo "POST-RENAME BUILD OK"

# Import path audit
rg -r 'import "reasonix/' --include '*.go' --output=file > /tmp/pre_rename_imports.txt
# ... perform renames ...
rg -r 'import "reasonix/' --include '*.go' --output=file > /tmp/post_rename_imports.txt
# post_rename_imports.txt should be empty
```

## 6. Definition of Done

- [ ] `go.mod` declares `module patty`
- [ ] All Go files use `patty/` import paths (zero `reasonix/` imports remain)
- [ ] `cmd/patty/` exists with correct entry point behavior
- [ ] `cmd/patty-launcher/` exists, uses profile-derived names
- [ ] `cmd/reasonix-*` directories fully removed
- [ ] No hardcoded product-specific symbols in shared packages
- [ ] Gate G2 proof: repository compiles; core tests pass; no mixed import namespace