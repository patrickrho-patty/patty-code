# Plan 03: Go Module and Core Semantic Rebrand

**Status:** Draft  
**Date:** 2026-08-08  
**Owner:** Implementation team  
**Dependencies:** Plan 02 complete (G1 gate); Plan 01 baseline ready  
**Gates:** G2 Core identity  

## 1. Purpose

Rename Go root module from `patty` to `patty`, move all `cmd/patty*` entries, neutralize shared symbols that reference Patty Code, and update URNs/schema/built-in identifiers — all in one controlled compile-green change.

## 2. Scope

- Root Go module rename (`go.mod`, goimports everywhere)
- `cmd/patty/*` → `cmd/patty/*`
- `cmd/patty-launcher/*` → `cmd/patty-launcher/*`
- `cmd/patty-legacy-migrator/*` → DELETE (clean break)
- `cmd/patty-plugin-example/*` → `cmd/patty-plugin-example/*`
- All import paths updated across entire codebase
- Shared symbol renaming: `patty-code.` → `patty.` where product-specific

### Exclusions
- i18n/catalog changes (Plan 05)
- TUI redesign (Plan 07)
- Desktop frontend UI text changes (Plan 08)
- Packaging/release names (Plan 09)
- GongCode profile (Plan 11)

## 3. Exact File Moves Required (§13.3)

| Current | New | Action |
|---|---|---|
| `cmd/patty/main.go` | `cmd/patty/main.go` | Move + module path update |
| `cmd/patty/main_test.go` | `cmd/patty/main_test.go` | Move + identity assertion update |
| `cmd/patty-launcher/main.go` | `cmd/patty-launcher/main.go` | Move + profile name source |
| `cmd/patty-launcher/main_test.go` | `cmd/patty-launcher/main_test.go` | Move + profile name test |
| `cmd/patty-legacy-migrator/` | *(delete)* | Remove entirely |
| `cmd/patty-plugin-example/` | `cmd/patty-plugin-example/` | Move + manifest update |
| `internal/skill/builtincontent/patty-guide/` | `internal/skill/builtincontent/patty-guide/` | Move + rewrite SKILL.md |
| `npm/patty/` | `npm/patty/` | Replace directory |

## 4. Task List

### T1: Update go.mod
- Change `module patty code` → `module patty`
- Verify all internal imports resolve after change
- Expected: `go mod tidy` succeeds, `go build ./...` compiles

### T2: Rename cmd directories
- `mv cmd/patty cmd/patty`
- `mv cmd/patty-launcher cmd/patty-launcher`
- `rm -rf cmd/patty-legacy-migrator`
- `mv cmd/patty-plugin-example cmd/patty-plugin-example`
- Update all internal import paths referencing these packages

### T3: Update all Go import paths
- Search: `import "patty/...` → `import "patty/...`
- Use semantic search for indirect references
- Run `go mod tidy` after each batch; verify compile green

### T4: Neutralize shared symbols
- `HarnessConfig` → `HarnessConfig` or `ProfileConfig`
- `HarnessHomeDir` → `HarnessHomeDir` or `ProductHome`
- `PATTY_HOME` → env-neutral constants resolved via Profile
- Every symbol change verified by tests

### T5: Update URNs, schemas, built-ins
- Schema field renames where Patty Code-named
- Built-in command IDs moved to Patty namespace
- Test every schema migration still functions

### T6: Update examples and SDK
- NPM package `npm/patty/package.json`
- SDK example manifests updated to patty coordinates
- Test: example compiles and loads

## 5. Commands

```bash
# Verify current build
go build ./cmd/patty/ && echo "BUILD OK"

# After module rename
go mod tidy && go build ./cmd/patty/ && echo "POST-RENAME BUILD OK"

# Import path audit
rg -r 'import "patty/' --include '*.go' --output=file > /tmp/pre_rename_imports.txt
# ... perform renames ...
rg -r 'import "patty/' --include '*.go' --output=file > /tmp/post_rename_imports.txt
# post_rename_imports.txt should be empty
```

## 6. Definition of Done

- [ ] `go.mod` declares `module patty`
- [ ] All Go files use `patty/` import paths (zero `patty/` imports remain)
- [ ] `cmd/patty/` exists with correct entry point behavior
- [ ] `cmd/patty-launcher/` exists, uses profile-derived names
- [ ] `cmd/patty-*` directories fully removed
- [ ] No hardcoded product-specific symbols in shared packages
- [ ] Gate G2 proof: repository compiles; core tests pass; no mixed import namespace
