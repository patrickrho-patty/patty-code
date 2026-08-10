# Plan 01: Inventory and Baseline

**Status:** Draft  
**Date:** 2026-08-08  
**Owner:** Implementation team  
**Dependencies:** None (plan-01 starts the chain)  
**Gates:** G0 Baseline  

## 1. Purpose

Establish the authoritative machine-readable match ledger of all Patty Code/pattycorp references in the repository, capture the current behavior baseline via test matrices, and freeze legal exceptions before any rebrand edits begin.

## 2. Scope

- Literal string scans for `patty`, `pattycorp`, Chinese locales, branded paths
- Semantic category classification (16 categories from §13.4)
- Behavioral baseline: record passing/failing tests per workstream area
- Build matrix: verify clean builds on macOS, document Windows/Linux parity gaps
- Legal exception registry with exact file, exact string, documented reason

### Exclusions
- Any edit to source code (that belongs to later plans)
- Generated artifact scanning (done post-rebuild in Plan 12)

## 3. Measured Baseline (§13.1 already scanned)

| Surface | Files with matches | Matches |
|---|---:|---:|
| Full tree (tracked files) | 1,627 | 13,276 |
| Chinese docs + zh locale files | 35+7 | 35 docs + 7 code files |
| Branded paths from §13.3 | 28 tracked paths | 28 entries to disposition |

## 4. Required Commands and Outputs

### 4.1 Primary Inventory Scan
```bash
# Section 13.2 required commands
git ls-files -z | xargs -0 rg -i --count-matches --no-messages --text 'patty' > /tmp/patty_counts.txt
grep -v ':0$' /tmp/patty_counts.txt > /tmp/patty_active.txt

git ls-files | xargs -I{} sh -c 'rg -qi "patty" "{}" && echo {}' > /tmp/patty_files.txt

git ls-files | xargs -I{} sh -c 'rg -qi "pattycorp" "{}" && echo {}' > /tmp/pattycorp_files.txt

rg -i --hidden --no-ignore --glob '!.git/**' 'patty|pattycorp' --output=file | sort -u > /tmp/patty_pattycorp_files.txt

find . \( -name "*.zh*" -o -name "*ko-KR*" -o -name "*en-US*" -o -name "*messages_ko*" \) -not -path '*/node_modules/*' -not -path '*/.git/*' > /tmp/chinese_paths.txt
```

### 4.2 Expanded Brand Scans (Category-specific)

```bash
# Category 4: Filesystem paths
rg -i '\.patty|patty\.toml|PATTY_|patty-code-' --hidden --no-ignore --glob '!.git/**' --output=file > /tmp/paths_scan.txt

# Category 2: Go import paths  
rg -r 'import.*"patty/' --include '*.go' --output=file > /tmp/go_imports.txt

# Category 9: Network endpoints
rg -i 'patty\.io|urn:patty' --output=file > /tmp/network_scan.txt

# Category 8: Executable/package names
rg -i 'cmd/patty|patty-code-launcher|patty-code-legacy-migrator|patty-plugin-example' --output=file > /tmp/exec_scan.txt

# Category 5: Configuration keys
rg -i '"language".*zh|"currency".*CNY|default.*zh' --include '*.go' --include '*.ts' --output=file > /tmp/config_keys.txt

# Category 16: OS integration
rg -i 'io.patty|patty-desktop|Patty Code.app' --output=file > /tmp/os_integration.txt

# Binary strings (generated artifacts from last release)
for f in $(find dist/ -type f 2>/dev/null); do strings "$f" 2>/dev/null | grep -qi 'patty' && echo "$f"; done > /tmp/binary_strings.txt 2>/dev/null
```

### 4.3 Ledger Schema (`inventory.jsonl`)

Each line is JSON with these fields (per §13.2):
```json
{
  "path": "internal/cli/chat.go",
  "line": 142,
  "matched_identity": "patty",
  "semantic_category": 3,
  "owning_workstream": "plan-03",
  "disposition": "rename",
  "replacement": "patty",
  "dependency": null,
  "verification": "test(cli:chat_rename)",
  "status": "unclassified"
}
```

Categories map to numbers:
1 = go_module, 2 = shared_symbol, 3 = product_identity, 4 = filesystem_path, 5 = config_key,
6 = database, 7 = log_metric_event, 8 = executable_package, 9 = endpoint_domain,
10 = ui_localization, 11 = documentation_test, 12 = legal_notice, 13 = upstream_org,
14 = binary_asset, 15 = hosted_service, 16 = os_integration

## 5. Task List

### T1: Run primary literal scans and produce raw file lists
- **Output:** `/tmp/patty_files.txt`, `/tmp/pattycorp_files.txt`, `/tmp/chinese_paths.txt`
- **Expected exit:** 0
- **Done when:** All four scan commands above execute successfully and output files are non-empty

### T2: Run expanded category-specific scans
- **Output:** Category scan files in `/tmp/`
- **Expected exit:** 0
- **Done when:** All category scans produce their respective output files

### T3: Classify every unique file into owning workstream and semantic categories
- Map each file to one or more categories; assign to the most relevant workstream
- Cross-reference with the ownership table in §13.1

### T4: Generate the structured ledger (`docs/superpowers/plans/inventory.jsonl`)
- Parse scan results and produce one JSONL entry per (file, match_position, identity) tuple
- Mark all as `unclassified` initially; categorize in T3/T5

### T5: Classify the 28 branded paths from §13.3
- Each path gets its disposition row in the ledger
- File moves and renames get explicit dependency ordering

### T6: Classify pattycorp operational vs legal references
- Operational coordinates → replace with patty-controlled values
- Authorship mentions → mark as legal exception, move to notices

### T7: Capture behavior baseline
- Record which test suites pass on current main: `go test ./...` summary
- Document known failures and flaky tests
- Snapshot: run proportionate test groups per workstream area and record pass/fail

### T8: Freeze legal exception registry
- One file: `docs/superpowers/plans/legal-exceptions.md`
- Exact path, exact string, documented reason for each exception
- Requires legal review sign-off before distribution

### T9: Finalize ledger — no rows left unclassified
- Every discovered row has an owner, disposition, and status
- No row remains `unclassified` at gate check

## 6. Definition of Done

- [ ] All literal scans executed and outputs archived
- [ ] `inventory.jsonl` contains every file/identity combination found
- [ ] All rows classified into semantic category and workstream
- [ ] 28 branded paths have explicit dispositions
- [ ] Behavior baseline recorded with pass/fail per test group
- [ ] Legal exception registry frozen and reviewed
- [ ] Zero rows in `unclassified` status
- [ ] Gate G0 proof: ledger generated, legal exceptions frozen, baseline captured

## 7. Rollback Instructions

Plan 01 produces no changes to distributable source. The ledger is advisory documentation. If reverted, simply delete the ledger files and temp scan outputs.