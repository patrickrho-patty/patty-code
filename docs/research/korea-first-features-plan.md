# Korea-First Features — Spec & Implementation Plan

> **Status**: Draft  
> **Date**: 2025-07-25  
> **Scope**: 10 features that differentiate Patty Code for the South Korean market  

## Table of Contents

1. [KRW Currency Support](#1-krw-currency-support)
2. [HWP/HWPX Document Parsing](#2-hwphwpx-document-parsing)
3. [Korean Documentation Hub (Context7-KR)](#3-korean-documentation-hub-context7-kr)
4. [Korean Compliance Scanning (PIPA / KISA / CSAP)](#4-korean-compliance-scanning-pipa--kisa--csap)
5. [Choseong Search Everywhere](#5-choseong-search-everywhere)
6. [EUC-KR / CP949 Encoding Support](#6-euc-kr--cp949-encoding-support)
7. [Korean-Natural i18n Audit](#7-korean-natural-i18n-audit)
8. [DLP Expansion — Full PIPA PII Coverage](#8-dlp-expansion--full-pipa-pii-coverage)
9. [Korean Date/Time/Number Formatting Awareness](#9-korean-datetimenumber-formatting-awareness)
10. [Jamo-Aware Diff & Edit Matching (NFC Normalization)](#10-jamo-aware-diff--edit-matching-nfc-normalization)

## Architecture Layers

Every feature maps to exactly one of three layers:

| Layer | Description | Examples in codebase |
|-------|-------------|---------------------|
| **Harness core** | Compiled Go in `internal/`. Always present, runs without the model. Deterministic, latency-sensitive. | `internal/billing`, `internal/dlp`, `internal/fileutil/encoding`, `internal/cli` |
| **System skill** | Embedded in the binary via `go:embed` (`internal/skill/builtincontent/`) or Go string constants (`internal/skill/builtins.go`). Model-driven but shipped with curated prompts. Not user-editable on disk. | `explore`, `review`, `security-review`, `test` |
| **Plugin / MCP server** | Separate process, connected at runtime. Own dependencies and update cadence. | External MCP servers, plugin packages |

---

## 1. KRW Currency Support

**Layer**: Harness core (`internal/billing`, `internal/config`)  
**Priority**: Low (not user-facing today since billing display is disabled for the subscription gateway model, but the source must be updated for correctness and future use)

### Description

The billing module's `symbol()` function maps ISO currency codes to display symbols. It currently handles `CNY`/`RMB` → `¥` and `USD` → `$`, but has no entry for `KRW`. The `normalizeCurrency()` function similarly only normalizes CNY and USD variants. For a Korean-first product, KRW (`₩`) must be a first-class citizen in the source even though the real-time cost display is currently disabled.

### User Stories

- **Future billing display**: When/if per-turn cost receipts are re-enabled (e.g. for a metered tier alongside the subscription), Korean users see `₩3,200` instead of `KRW 3200` or a raw fallback.
- **Desktop currency selector**: The desktop app's `Settings → Model → Access` currency picker should offer KRW alongside USD and CNY, so Korean users who connect third-party providers (e.g. their own OpenAI key) see costs in Won.
- **Internal correctness**: Any code path that touches `normalizeCurrency()` or `symbol()` (balance display, per-turn receipts, config auto-currency) handles KRW without falling through to the unknown-code path.

### Where This Lives

| File | Change |
|------|--------|
| `internal/billing/balance.go` | Add `case "KRW", "WON", "₩": return "₩"` to `symbol()`. Add `case "KRW", "WON", "₩": return "KRW"` to `normalizeCurrency()`. |
| `internal/billing/balance_test.go` | Add `TestSymbolKRW`, `TestSymbolWon`, `TestNormalizeCurrencyKRW` test cases. |
| `internal/config/backfill_test.go` | Add a test that verifies `SetDesktopCurrency("KRW")` works and flows through to pricing display. |

### Edge Cases

- **Won formatting**: Korean Won has no decimal subdivision in practice. `₩3200` is correct, not `₩3,200.00`. If the balance API returns a decimal string like `"3200.00"`, the display should strip trailing `.00` for KRW specifically.
- **Number grouping**: Korean number grouping uses commas at thousands (`₩1,000,000`), same as USD. However, colloquial Korean groups at ten-thousands (man/eok). The display should use standard comma grouping, not Korean counting units — this is a financial display, not prose.
- **CNY fallback**: The current `DisplayForCurrency` falls back to CNY when the preferred currency isn't found. This CNY-first assumption was inherited from the Chinese fork. When KRW is the preferred currency and no KRW balance exists, the fallback should show the raw provider currency with its ISO prefix (e.g. `USD $2.31`), not silently show CNY.

### Future Upgrades

- **Live exchange rate hint**: When a provider reports balances in USD but the user's currency is KRW, show an approximate Won equivalent as a hint (not a conversion — `~₩3,200` next to `$2.31`), fetched from a lightweight rate source or a static daily rate.
- **Subscription usage dashboard**: When the subscription model is in place, a `/usage` command could show monthly usage in Won with tier boundaries.

---

## 2. HWP/HWPX Document Parsing

**Layer**: Plugin (MCP server), auto-installed and auto-connected by the harness  
**Priority**: High — this is one of the strongest Korea-specific differentiators

### Description

HWP (Hancom Word Processor) and HWPX (the newer XML-based variant) are the dominant document formats in Korean government, education, military, and large enterprises. Requirements specs, RFPs, internal policies, and technical documentation are routinely distributed as `.hwp` or `.hwpx` files. No competing coding agent (Claude Code, Codex, Cursor) can read these formats.

Patty Code already has an HWP/HWPX parsing engine built separately. The question is where the integration layer sits. The recommendation is: **ship it as a default-installed MCP plugin** that the harness auto-connects on startup, exposing `read_hwp` and `read_hwpx` tools that the model can call like any other file-read tool.

### User Stories

- **Reading requirements specs**: A Korean enterprise developer receives a `.hwpx` requirements document from a government client. They drop it in their project directory and say "read the requirements spec and scaffold the API." The model calls `read_hwpx`, gets the structured text, and proceeds.
- **Extracting tables from government forms**: A developer working on a public-sector integration needs to parse field names from a government form template distributed as `.hwp`. The model reads the document and generates corresponding struct definitions or database schema.
- **Comparing spec versions**: A user has two versions of an `.hwpx` spec and asks "what changed between v2 and v3?" The model reads both and diffs the content.
- **Searching across HWP files**: A user asks "find all mentions of the authentication requirements across the spec documents" and has 5 `.hwpx` files in a `docs/` folder. The model reads each and synthesizes.

### Where This Lives

| Component | Location | Description |
|-----------|----------|-------------|
| **HWP/HWPX parser** | Separate repository (already exists) | The parsing engine itself — OLE2 container handling for HWP, XML extraction for HWPX, text/table/image extraction. |
| **MCP server wrapper** | Separate repository, packaged as a binary | Thin MCP stdio server that exposes `read_hwp(path)` and `read_hwpx(path)` tools. Returns structured text (markdown-formatted with tables preserved). |
| **Auto-install hook** | `internal/boot` or `internal/pluginpkg` | On first run or upgrade, the harness ensures the HWP plugin is installed in the global plugin directory. Uses the same install mechanism as `install_source`. |
| **Auto-connect config** | `internal/config` | The default `patty.toml` template includes the HWP plugin entry. Existing installs get it via the migration path. |

### Edge Cases

- **Large documents**: Government RFPs can be 200+ pages. The parser should stream text section-by-section or return a structured outline first, then allow the model to request specific sections. A single 200-page dump would blow the context window.
- **Embedded images**: HWP files often contain screenshots, diagrams, and scanned stamps. The parser should extract alt-text or captions where available and note `[image: filename.png]` placeholders for images it cannot interpret.
- **Password-protected files**: Some government HWP files are password-protected. The tool should return a clear error ("This HWP file is password-protected. Please provide the password or an unprotected version.") rather than failing silently.
- **Encoding variety**: Older `.hwp` files may use EUC-KR internally. The parser must handle this transparently (ties into Feature 6).
- **Table structure preservation**: Korean government documents are heavily table-structured (forms, checklists, matrices). The parser must preserve table structure as markdown tables, not flatten them to prose.
- **Mixed content**: HWP files can contain embedded OLE objects (Excel sheets, images, other HWP files). The parser should extract what it can and flag what it skips.

### Future Upgrades

- **HWP write support**: Beyond reading, allow the model to generate `.hwpx` output — e.g., "create a test report in HWPX format." This is a much larger scope but would be unique in the market.
- **HWP diff tool**: A dedicated `diff_hwp` tool that structurally compares two HWP documents and reports changes (not just text diff, but table/section-aware diff).
- **@-reference support**: Allow `@document.hwpx` in the chat input to attach an HWP file to the conversation context, similar to how `@file.py` works for code files.
- **Government form field extraction**: A specialized mode that identifies form fields (labeled input areas in HWP tables) and generates corresponding data structures or API request/response shapes.

---

## 3. Korean Documentation Hub (Context7-KR)

**Layer**: Plugin (MCP server for the index/API) + System skill (for invocation UX)  
**Priority**: High — addresses the model knowledge gap for Korean platform APIs

### Description

Context7 (by Upstash) provides a live documentation index that coding agents can query to get up-to-date, version-specific docs injected directly into the LLM context. It covers mainstream open-source libraries but has no coverage of Korean-specific platforms.

Patty Code needs a **Korean equivalent**: a documentation index that crawls, indexes, and serves the latest docs for Korean platform APIs, government frameworks, and Korean-language translations of popular global frameworks. The index should be a separate service (with its own cron/scraper) and the harness should ship a built-in MCP client and a system skill for invocation.

### What Gets Indexed

**Korean Platform APIs (Tier 1 — must-have)**:
- Naver Cloud Platform (maps, AI, storage, messaging)
- Kakao (Kakao Login, Kakao Maps, KakaoTalk messaging, Kakao Pay)
- Toss Payments (payment processing, virtual accounts, webhooks)
- NHN (KCP payments, Toast UI, Toast Cloud)
- NICE Payments / Inicis (legacy PG integration)
- Coupang (Open API for sellers, logistics)
- Samsung SDS / Samsung Knox (enterprise MDM APIs)

**Korean Government & Public Sector (Tier 2)**:
- eGovFrame (electronic government standard framework — Spring-based)
- data.go.kr public data portal APIs
- KISA security standard reference documents
- National Tax Service (HomeTax) API specifications
- Health Insurance Review & Assessment Service (HIRA) APIs

**Korean-Language Global Framework Docs (Tier 3)**:
- Korean official docs for React, Next.js, Vue, Spring Boot, Django
- Korean-translated MDN Web Docs
- Korean AWS/GCP documentation

### User Stories

- **Integrating Kakao Login**: A developer says "add Kakao Login to this Next.js app." The model calls the doc hub, gets the latest Kakao Login REST API spec (auth flow, token endpoints, required parameters), and generates the integration code with the correct OAuth flow — not a hallucinated version from training data.
- **Toss Payments webhook setup**: A developer asks "set up Toss Payments webhooks for our order service." The model fetches the current Toss webhook spec (which changes across API versions) and generates the correct handler, including the signature verification pattern that's unique to Toss.
- **eGovFrame project setup**: A developer at a government contractor asks "scaffold an eGovFrame project with the standard structure." The model fetches the current framework documentation and generates the correct Maven/Gradle structure, not a generic Spring Boot scaffold.
- **Checking the latest API version**: A developer asks "what's the current version of the Naver Maps SDK?" The model queries the hub and gives a current, cited answer instead of a training-data guess.

### Where This Lives

| Component | Location | Description |
|-----------|----------|-------------|
| **Documentation scraper/indexer** | Separate repository / service | Cron-driven scraper that crawls target doc sites, chunks content, and stores in a search-ready index. Could use the same architecture as Context7 (chunked markdown + embeddings) or a simpler BM25 index. |
| **API server** | Separate service (hosted) | REST/MCP API that accepts `resolve_library(name)` and `get_docs(library_id, topic, tokens)` calls. Rate-limited, cached. |
| **MCP client plugin** | Ships with harness as a default plugin | Thin MCP client that connects to the API server. Registered in the default plugin list. |
| **System skill** | `internal/skill/builtins.go` | A built-in skill (Go string constant like `builtinResearchBody`) that teaches the model when and how to call the doc hub tools. Invokable via `/docs` or automatically when the model detects a Korean platform integration task. |

### Architecture Decision: On-Demand vs Automatic

The model should call the doc hub **both ways**:

1. **On-demand via `/docs`**: The user explicitly asks for docs. The system skill fires, calls the MCP server, and returns the relevant documentation.
2. **Autonomous**: The model detects that it's working with a Korean platform (e.g., it sees `kakao` in `package.json` or the user mentions "Toss Payments") and proactively calls the doc hub before generating integration code. This is the same pattern as Context7's "use context7" instruction — the system prompt tells the model to check docs before coding against Korean APIs.

The skill prompt should include a trigger list of Korean platform names so the model knows when to call the hub without being asked.

### Edge Cases

- **Version pinning**: Korean APIs change frequently and don't always follow semver. The doc hub must track API versions and let the model request docs for a specific version (e.g., "Toss Payments v2023-12-01" vs the latest).
- **Rate limiting**: If many Patty Code users hit the doc hub simultaneously, the service needs appropriate caching and rate limiting. The MCP client should cache responses locally for the session.
- **Stale docs**: Some Korean platforms update their docs without clear versioning. The scraper should track last-crawl timestamps and the skill should surface the freshness date to the model ("docs last updated: 2025-07-20").
- **Auth-gated docs**: Some platform docs (e.g., Samsung Knox) require API key registration to access. The scraper handles this at the service level; the end user's Patty Code never needs platform-specific auth to read docs.
- **Mixed Korean/English**: Many Korean platform docs mix Korean explanations with English code samples and parameter names. The indexer should preserve both languages and not strip either.

### Future Upgrades

- **Community contributions**: Allow Korean developers to submit documentation sources (URLs) that the hub should crawl, similar to Context7's library submission flow.
- **Private instance**: For enterprises behind firewalls, offer a self-hosted doc hub that can crawl internal API documentation.
- **Code example extraction**: Beyond reference docs, extract working code examples from Korean tech blogs (Velog, Tistory) that demonstrate integration patterns, ranked by recency and quality.
- **SDK changelog tracking**: Alert the model when a Korean SDK has breaking changes since the version in the user's `package.json` or `build.gradle`.

---

## 4. Korean Compliance Scanning (PIPA / KISA / CSAP)

**Layer**: System skill (embedded in binary via `go:embed` under `internal/skill/builtincontent/`)  
**Priority**: Medium — differentiator for enterprise and government contractor users

### Description

Korean software must comply with several regulatory frameworks depending on the deployment context. The three most common are:

- **PIPA** (Personal Information Protection Act): Korea's primary data privacy law, analogous to GDPR. Requires consent management, data minimization, breach notification, and specific handling of Korean PII types (RRN, phone numbers, etc.).
- **KISA** (Korea Internet & Security Agency) standards: Security certification requirements for software sold to or used by Korean government entities. Covers secure coding practices, vulnerability management, and incident response.
- **CSAP** (Cloud Security Assurance Program): Certification required for cloud services used by Korean government agencies. Covers access control, encryption, logging, and data residency.

This feature adds a system skill that can scan a codebase and report compliance posture against these frameworks. It operates like the existing `security-review` built-in skill but with Korean regulatory specificity.

### User Stories

- **PIPA compliance check**: A developer building a user registration system asks "is this PIPA compliant?" The model reads the codebase, checks for consent collection before PII storage, checks that RRNs are encrypted at rest (not just hashed), checks that data retention periods are enforced, and reports findings with specific PIPA article references.
- **KISA secure coding review**: A government contractor preparing for KISA certification asks "review this for KISA secure coding standards." The model checks for the KISA-defined common vulnerability patterns (SQL injection, XSS, path traversal, etc.) using the KISA-specific checklist items, not generic OWASP.
- **CSAP readiness**: A SaaS startup targeting government contracts asks "are we CSAP ready?" The model reviews access control patterns, encryption usage, audit logging, and data residency constraints and reports a gap analysis.
- **Pre-audit preparation**: Before a formal compliance audit, a developer runs the skill to identify and fix issues proactively, saving weeks of audit remediation.

### Where This Lives

| Component | Location | Description |
|-----------|----------|-------------|
| **Skill body** | `internal/skill/builtincontent/korean-compliance/SKILL.md` | Embedded markdown with frontmatter. Contains the PIPA/KISA/CSAP checklist items, the scanning methodology, and the report format. Runs as a subagent. |
| **Invocation** | `/compliance` slash command (Korean: `/compliance`) | Triggers the skill. Optional argument: `pipa`, `kisa`, `csap`, or `all`. |
| **Report output** | Model-generated markdown | The skill produces a structured compliance report with pass/fail/warning items, grouped by regulation section. |

### How It Works (not user-editable — this is the skill prompt architecture)

The skill prompt contains:

1. **Checklist definitions**: Each compliance framework as a numbered list of check items, with the Korean regulation article reference and a description of what to look for in code.
2. **Scanning instructions**: The model reads the codebase structure, identifies data-handling code (user models, API handlers, database queries, auth flows), and evaluates each checklist item.
3. **Report template**: A structured output format with severity levels (critical / warning / info) and remediation guidance with Korean legal context.

The model uses the existing built-in tools (`read_file`, `grep`, `glob`, `code_index`) to scan — no new tools needed.

### Edge Cases

- **Framework-specific patterns**: Django vs Spring Boot vs Express vs Go each handle PII differently. The skill prompt must be framework-agnostic in its checks but framework-aware in its remediation suggestions.
- **False positives on PII handling**: Not every string that looks like an RRN pattern in code is actual PII handling — it might be test data, validation regex, or documentation. The skill should distinguish between PII storage/transmission code and PII validation/format code.
- **Scope boundaries**: A compliance scan of a large monorepo shouldn't take 30 minutes. The skill should ask the user to scope to a specific directory or module if the codebase is too large, or focus on files that touch user data.
- **Regulatory updates**: Korean regulations change. The embedded checklist will need periodic updates with harness releases. The skill prompt should include a "last updated" date so users know the regulatory baseline.

### Future Upgrades

- **Auto-remediation mode**: Beyond reporting, offer to fix common compliance gaps automatically (e.g., add consent logging middleware, add encryption wrappers around PII storage).
- **Compliance CI integration**: A non-interactive mode that runs the compliance scan in CI and produces a machine-readable report (JSON/SARIF) for integration with enterprise security dashboards.
- **ISMS-P checklist**: Add support for ISMS-P (Information Security Management System - Personal Information), the combined security/privacy certification that Korean enterprises increasingly require.
- **Regulation-linked documentation**: Generate compliance documentation artifacts (privacy policy templates, data processing agreements) that reference the specific code implementations found during the scan.

---

## 5. Choseong Search Everywhere

**Layer**: Harness core (`internal/textutil`, `internal/cli`)  
**Priority**: High — deeply native Korean UX that no competitor will match

### Description

Choseong (initial consonant) search is a fundamental Korean UX pattern. Korean users routinely type initial consonants to find items quickly — typing `ㅋㅋ` to find "Kakao", or `ㅂㄹㅊ` to find a command with certain Korean characters. Patty Code already supports choseong matching for slash commands via `chosungOf()` in `internal/cli/slash_registry.go` and `hangulLeadingJamo()` in `internal/cli/composer_selection.go`. However, this capability is scoped **only** to the slash command palette.

This feature extracts the choseong decomposition logic into a shared utility in `internal/textutil` and wires it into every search/filter surface in the harness.

### Surfaces That Need Choseong Support

| Surface | Current state | Change needed |
|---------|---------------|---------------|
| Slash command palette (`/`) | Has choseong matching | Refactor to use shared utility |
| `@`-reference file search | No choseong support | Wire choseong filter for Korean filenames |
| Plugin marketplace search (`/plugin search`) | No choseong support | Wire choseong filter for plugin names/descriptions |
| MCP tool discovery | No choseong support | Wire choseong filter for tool names |
| Skill search (`/skill`) | No choseong support | Wire choseong filter for skill names |
| Session/history search | No choseong support | Wire choseong filter for session topics |
| Memory search | No choseong support | Wire choseong filter for memory entries |

### User Stories

- **Finding a Korean-named file**: A project has Korean-named source files (common in Korean enterprise codebases). The user types `@ㅅㅈ` and the completion menu shows files starting with corresponding Korean characters.
- **Searching plugins**: The user types `/plugin search ㅎㄱ` and finds a Korean-named plugin without typing the full name.
- **Searching skills**: The user types `/skill ㅋㅁ` to find a Korean-named skill.
- **Quick navigation**: A user looking for a specific slash command types the initial consonants of the Korean name and gets immediate fuzzy results.

### Where This Lives

| File | Change |
|------|--------|
| **`internal/textutil/choseong.go`** (new) | Extract `chosungOf()` and `hangulLeadingJamo()` from `internal/cli/slash_registry.go` and `internal/cli/composer_selection.go` into a shared package. Export as `textutil.ChoseongOf(s string) string` and `textutil.HangulLeadingJamo(r rune) rune`. Add `textutil.ChoseongMatch(candidate, query string) bool` that returns true if the query (which may be choseong characters) matches the leading consonants of the candidate. |
| **`internal/textutil/choseong_test.go`** (new) | Comprehensive tests covering: full choseong match, partial prefix match, mixed Korean/Latin input, empty strings, non-Korean input passthrough. |
| **`internal/cli/slash_registry.go`** | Replace inline `chosungOf()` with `textutil.ChoseongOf()`. |
| **`internal/cli/composer_selection.go`** | Replace inline `hangulLeadingJamo()` with `textutil.HangulLeadingJamo()`. |
| **`internal/cli/complete.go`** | In `fuzzyFilterSlash()` and `aliasMatches()`, use `textutil.ChoseongMatch()`. Extend `@`-reference completion to use the same filter. |

### Edge Cases

- **Ambiguous consonants**: The consonant `ㅂ` is the leading jamo for many common syllables. A single-consonant query should show all matches, not try to disambiguate — the user will refine by typing more consonants.
- **Double consonants**: `ㄲ`, `ㄸ`, `ㅃ`, `ㅆ`, `ㅉ` are distinct initial consonants and must match separately from their single versions. `ㄱ` should NOT match syllables starting with `ㄲ`.
- **Non-Korean passthrough**: If the query contains no Hangul jamo characters, the choseong matcher should return false immediately and let the normal fuzzy filter handle it. No performance penalty for English-only users.
- **Mixed queries**: A query like `ㅋㅋlogin` (Korean choseong + English) should match items where the Korean part matches choseong and the English part matches literally. This requires splitting the query at script boundaries.
- **File path separators**: For `@`-reference search, Korean directory names may contain choseong-matchable characters but path separators (`/`) should not be decomposed.

### Future Upgrades

- **Choseong in grep results**: When `grep` returns results from files with Korean content, highlight choseong matches in the output.
- **Choseong in desktop app search**: Extend to the desktop app's global search (Cmd+K / Ctrl+K) for sessions, files, and commands.
- **Weighted choseong ranking**: Rank choseong prefix matches higher than choseong subsequence matches, so exact initial-consonant matches appear first.

---

## 6. EUC-KR / CP949 Encoding Support

**Layer**: Harness core (`internal/fileutil/encoding`)  
**Priority**: Medium-high — fills a gap in the existing encoding cascade

### Description

The file encoding module (`internal/fileutil/encoding/encoding.go`) already handles UTF-8, UTF-8 BOM, UTF-16 LE/BE (with and without BOM), and GB18030 (Chinese legacy encoding). The detection cascade is: BOM → UTF-16 NoBOM heuristic → strict UTF-8 → GB18030 → lossy UTF-8.

**EUC-KR and CP949 are completely absent.** This is a critical gap for a Korean-first product. EUC-KR (and its superset CP949, also called "Unified Hangul Code") is still encountered in:

- Legacy CSV exports from Korean banking systems
- Government data portal downloads (data.go.kr)
- Database dumps from older Korean enterprise systems (Oracle Korea deployments, Tibero)
- Telecom billing data and insurance documents
- Legacy web scraping artifacts from older Korean websites
- Configuration files from Korean enterprise software (SAP Korea localizations, domestic ERP systems)

The `golang.org/x/text/encoding/korean` package provides `EUCKR` (which handles CP949 as a superset), making this a straightforward addition to the existing cascade.

### User Stories

- **Reading legacy CSV data**: A developer working on a data migration project has a CSV file exported from a Korean bank's legacy system in EUC-KR. They `@data.csv` or ask "read this CSV and generate the import schema." Currently, the file either shows as garbled UTF-8 or falls through to GB18030 (which may partially decode but produces wrong characters). With EUC-KR support, the file is transparently converted to UTF-8 before the model sees it.
- **Working with government data**: A developer downloads a dataset from data.go.kr in CSV format. Many of these are still EUC-KR encoded. The read_file tool auto-detects and converts, with a note in the output: "Detected EUC-KR encoding, converted to UTF-8."
- **Editing legacy config files**: A developer maintaining a Korean enterprise system needs to edit a configuration file in CP949. The harness reads it correctly, the model suggests changes, and the write-back preserves the original CP949 encoding (same round-trip pattern as the existing GB18030 and UTF-16 support).

### Where This Lives

| File | Change |
|------|--------|
| **`internal/fileutil/encoding/encoding.go`** | Add `EUCKR Kind = iota` constant. Add EUC-KR detection in `Detect()` after GB18030 (or before — see edge cases). Add `Decode`/`Encode` cases using `korean.EUCKR`. Add `Decoder()` case for streaming. |
| **`internal/fileutil/encoding/encoding_test.go`** | Add `TestDetectEUCKR` (pure Korean EUC-KR text), `TestDetectCP949` (CP949 extended characters), `TestDecodeEUCKR`, `TestEncodeEUCKR`, `TestRoundTripEUCKR`. |
| **`go.mod`** | `golang.org/x/text` is already a dependency (used for `simplifiedchinese`). The `encoding/korean` subpackage is included automatically. |

### Detection Strategy

The critical question is: **how to distinguish EUC-KR from GB18030** when both are valid decodings of the same byte sequence.

Both EUC-KR and GB18030 use multi-byte sequences in the 0x81-0xFE range. Some byte sequences are valid in both encodings but decode to different characters. The detection heuristic:

1. **BOM check** — same as today (eliminates UTF-8 BOM, UTF-16).
2. **UTF-16 NoBOM** — same as today.
3. **Strict UTF-8** — if all bytes are valid UTF-8, use UTF-8.
4. **EUC-KR first, then GB18030**: Try EUC-KR decoding. If it succeeds AND the decoded text contains Hangul syllable characters (Unicode range U+AC00–U+D7A3), classify as EUC-KR. If EUC-KR decoding succeeds but produces no Hangul, try GB18030. If EUC-KR decoding fails, try GB18030.

This "Hangul presence" heuristic works because:
- EUC-KR files from Korean systems invariably contain Korean characters.
- A file that decodes as valid EUC-KR but contains zero Hangul is almost certainly not Korean text.
- GB18030 files from Chinese systems rarely contain Hangul (and if they do, they'd also be valid EUC-KR for those characters).

### Edge Cases

- **EUC-KR vs GB18030 ambiguity**: Some byte sequences (0xA1-0xFE range) are valid in both encodings. The Hangul-presence heuristic handles the common case, but a file containing only ASCII + a few ambiguous multi-byte sequences with no Hangul could be misclassified. In practice, this is rare — Korean legacy files always contain Korean text.
- **CP949 extensions**: CP949 is a strict superset of EUC-KR, adding ~8,000 additional Hangul syllables. The `korean.EUCKR` codec in `golang.org/x/text` handles CP949 fully, so no separate handling is needed.
- **Mixed-encoding files**: Some legacy Korean systems produce files with mixed encoding (e.g., EUC-KR body with UTF-8 BOM prepended by a later processing step). The BOM check catches this — a UTF-8 BOM means UTF-8 regardless of body content.
- **Round-trip preservation**: When the model edits an EUC-KR file, the write-back must re-encode to EUC-KR. The existing `writeFileEncoded()` in `internal/tool/builtin/encoding_helpers.go` already handles this pattern for GB18030 — EUC-KR follows the same path.
- **Binary files**: The existing binary-file detection (NUL-byte check in `read_file`) must run before encoding detection. EUC-KR's multi-byte sequences don't contain NUL bytes (unlike UTF-16), so this is safe.
- **Performance**: Encoding detection runs on every file read. The EUC-KR check adds one `transform.Bytes` call (same cost as the existing GB18030 check). For the common case (UTF-8 files), the check is never reached because `utf8.Valid()` returns true first.

### Future Upgrades

- **Encoding override**: A config option or tool parameter to force a specific encoding when auto-detection is wrong: `read_file path=data.csv encoding=euc-kr`.
- **Batch conversion**: A utility command that converts all EUC-KR files in a directory to UTF-8 in place, for teams migrating off legacy systems.
- **Encoding warning in tool output**: When EUC-KR is detected, add a note to the tool output suggesting UTF-8 conversion for long-term maintainability.

---

## 7. Korean-Natural i18n Audit

**Layer**: Harness core (`internal/i18n`)  
**Priority**: High — a Korean-first product must feel Korean, not translated

### Description

The i18n module (`internal/i18n/messages_ko.go`, 628 lines) provides Korean translations for all UI strings. However, a comprehensive audit is needed to ensure:

1. **No English leakage**: Strings that bypass the i18n system and appear as hardcoded English in Korean mode.
2. **Natural Korean phrasing**: Translations that read like natural Korean, not literal English-to-Korean translations. Korean has different sentence structures, politeness levels, and technical vocabulary conventions than English.
3. **Consistent register**: All user-facing text should use a consistent politeness level. Korean has multiple speech levels; a coding tool for professional developers should use polite but not overly formal language (hapsyo-che or haerache depending on context).
4. **Korean-specific error context**: Error messages should include Korean-specific troubleshooting hints where relevant (e.g., Korean path encoding issues on Windows, IME-related input problems, Korean-specific firewall/proxy issues in enterprise environments).

### User Stories

- **First-run experience**: A Korean developer installs Patty Code and runs `patcode setup`. Every prompt, confirmation, error message, and hint reads naturally in Korean. Nothing feels "translated."
- **Error diagnosis**: When a tool call fails due to a Korean-specific cause (e.g., a file path with Korean characters fails on a system with incorrect locale settings), the error message explains the Korean-specific cause and suggests a Korean-specific fix.
- **Help text**: Running `/help` or `/?` shows help text that uses Korean developer vocabulary naturally — using the terms Korean developers actually use (which are often different from direct translations of English terms).

### Where This Lives

| File | Change |
|------|--------|
| `internal/i18n/messages_ko.go` | Audit all 628 lines. Rewrite strings that are literal translations. Ensure consistent politeness level. |
| `internal/i18n/messages_en.go` | Reference for parity check — ensure every English key has a Korean counterpart. |
| `internal/i18n/catalog_parity_test.go` | Existing test that checks key parity between EN and KO. Strengthen to catch new keys added without Korean translations. |
| **Grep for hardcoded English** | Scan all `internal/` `.go` files for user-facing strings (error messages, fmt.Sprintf, log output) that bypass the i18n system. |

### Audit Checklist

- [ ] Every key in `messages_en.go` has a corresponding key in `messages_ko.go`
- [ ] No hardcoded English strings in user-facing paths (CLI output, TUI elements, error messages, help text)
- [ ] Consistent politeness level across all Korean strings (target: polite informal, haeyo-che)
- [ ] Technical terms use standard Korean developer vocabulary (e.g., "branch" stays as "branch" not a forced translation, but "commit" could be either depending on context)
- [ ] Error messages include Korean-specific troubleshooting hints where applicable
- [ ] Number formatting in Korean strings follows Korean conventions
- [ ] Pluralization handling (Korean doesn't pluralize nouns the way English does — remove any forced plural patterns)

### Edge Cases

- **Loanword decisions**: Korean developers use many English loanwords. The audit must decide which terms to keep in English (model, provider, plugin, commit, branch, merge) and which to translate (file → sometimes kept as-is, tool → sometimes kept as-is). The rule of thumb: keep loanwords that Korean developers universally use in English; translate terms where a Korean word is more commonly used.
- **Dynamic string composition**: Some messages are built by concatenating translated fragments. Korean word order is SOV (Subject-Object-Verb), opposite to English SVO. Concatenated messages that worked in English may read unnaturally in Korean. These need restructuring, not just translation.
- **Contextual length**: Korean text is often shorter than English for the same meaning (no articles, more compact syntax), but sometimes longer (topic markers, polite endings). UI elements with fixed widths may need adjustment.
- **Terminal width**: CJK characters are typically double-width in terminal rendering. The existing `go-runewidth` usage handles this, but any new strings must be tested for TUI layout correctness.

### Future Upgrades

- **Community-contributed translations**: An open translation file that the Korean developer community can submit improvements to, reviewed and merged by the team.
- **Regional terminology variants**: Some Korean technical vocabulary differs between South Korea and overseas Korean communities. For now, target South Korean standard usage.
- **Dynamic language detection**: If the user switches their terminal locale mid-session, the UI language should update accordingly (currently set at startup).

---

## 8. DLP Rule Catalog Expansion — Multi-Repo Alignment

**Layer**: Relay (authoritative) → Harness (mirror) → Admin UI (presets)  
**Priority**: P0 — security differentiator for enterprise adoption, multi-phase

### Architecture

The DLP rule system spans three codebases that must stay aligned:

```
PCCP Relay (authoritative source of truth)
  └─ internal/security/service.go     → rule definitions + detection engine
  └─ internal/security/workflows.go   → per-org lexicon overrides
  └─ internal/models/security.go      → SecurityRule, PIILexicon persistence
  └─ web/src/pages/Security.tsx       → admin UI: rule toggles, builder, tester
       │
       │ DARI MsgDLPRulePack (class toggles → extended to per-rule)
       ▼
Patty Code Harness (mirrors relay rules)
  └─ internal/dlp/scan.go             → Scanner, rule definitions
  └─ internal/dlp/scan_test.go        → positive/negative rule tests
  └─ internal/dariproto/dlprules.go   → classPrefixes bridge, DLPRulePackWire
  └─ internal/agent/dlp_wrap.go       → applyPackToScanner sink
```

### Current State vs. Target State

| Dimension | Current | Target |
| :--- | :--- | :--- |
| Relay rules | 16 (4 PII + 5 secrets + 4 injection + 3 paths) | ~60+ organized hierarchically |
| Harness rules | 11 (4 PII + 4 secrets + 3 injection) | Mirror relay catalog |
| Rule ID convention | Relay: `pii-kr-rrn`, Harness: `kr-rrn` | Standardize on relay convention |
| Sync mechanism | Class-level toggles only (`classPrefixes`) | Per-rule enabled/severity/action |
| Scope model | Org-wide only | Org → Team → User → Harness (cascading) |
| Admin UI | Hardcoded `RULE_PRESETS` in Security.tsx | Synced from authoritative catalog |
| PII Lexicon | DB model exists, not wired to harness | Deferred to Phase 3 |

### Full Rule Catalog (Category-Subcategory Hierarchy)

All rule IDs use the relay convention (`pii-kr-*`, `secret-*`, `injection-*`, `path-*`, `compliance-*`).

```
korean_pii/                              [class: korean_pii] [prefix: pii-kr-]
├── national_id/
│   ├── pii-kr-rrn                       주민등록번호 (Critical)
│   ├── pii-kr-foreign-rrn               외국인등록번호 (Critical)
│   └── pii-kr-driver-license            운전면허번호 (High)
├── contact/
│   ├── pii-kr-phone                     휴대전화번호 (High)
│   ├── pii-kr-phone-landline            유선전화번호 (Medium)
│   └── pii-kr-email                     이메일주소 (Low)
├── financial/
│   ├── pii-kr-bank-account              계좌번호 (High)
│   ├── pii-kr-credit-card               신용카드번호 (Critical)
│   ├── pii-kr-business                  사업자등록번호 (High)
│   └── pii-kr-health-insurance          건강보험번호 (Critical)
├── travel/
│   └── pii-kr-passport                  여권번호 (High)
└── digital/
    ├── pii-kr-ipin                      아이핀 (High)
    └── pii-kr-certificate               공동인증서 (Medium)

secret/                                  [class: secret] [prefixes: secret-aws-, secret-gcp-, etc.]
├── cloud/
│   ├── secret-aws-key                   (Critical)
│   ├── secret-gcp-key                   (Critical)
│   ├── secret-azure-key                 (Critical)
│   └── secret-ncloud-key                네이버클라우드 API 키 (Critical)
├── api/
│   ├── secret-github-pat                (Critical)
│   ├── secret-gitlab-token              (Critical)
│   ├── secret-openai-key                (Critical)
│   ├── secret-slack-webhook             (High)
│   └── secret-jwt-token                 (High)
├── crypto/
│   ├── secret-private-key-pem           (Critical)
│   ├── secret-ssh-key                   (Critical)
│   └── secret-pgp-key                   (Critical)
└── database/
    ├── secret-mysql-connstring           (High)
    ├── secret-postgres-connstring        (High)
    └── secret-redis-connstring           (High)

prompt_injection/                        [class: prompt_injection] [prefix: injection-]
├── override/
│   └── injection-override               ignore previous instructions (Critical)
├── jailbreak/
│   └── injection-jailbreak              DAN/do anything now (Critical)
├── system/
│   └── injection-system-prompt          fake system prompt (High)
├── exfil/
│   ├── injection-exfil-email            ask to email output (High)
│   └── injection-exfil-url              ask to POST output (High)
└── encoding/
    └── injection-base64                 base64-encoded instruction (High)

sensitive_path/                          [class: sensitive_path] [prefix: path-]
├── system/
│   ├── path-etc-passwd                  /etc/passwd (Critical)
│   ├── path-etc-shadow                  /etc/shadow (Critical)
│   └── path-proc-self                   /proc/self/* (High)
├── credentials/
│   ├── path-dotenv                      .env file (Critical)
│   ├── path-aws-credentials             ~/.aws/credentials (Critical)
│   ├── path-gcp-key                     GCP service account JSON (Critical)
│   └── path-kube-config                 ~/.kube/config (High)
└── source/
    ├── path-git-config                  .git/config (Medium)
    └── path-npmrc                       .npmrc (Medium)

compliance/                              [NEW class: compliance] [prefixes: pipa-, kisa-, csap-]
├── pipa/
│   ├── pipa-consent-missing             동의 없는 수집 (High)
│   ├── pipa-retention-violation         보관기간 초과 (High)
│   ├── pipa-encryption-weak             취약한 암호화 (Critical)
│   └── pipa-log-plaintext-pii           PII 평문 로깅 (Critical)
├── kisa/
│   ├── kisa-sql-injection               SQL 인젝션 패턴 (Critical)
│   ├── kisa-hardcoded-credential        하드코딩된 자격증명 (Critical)
│   └── kisa-weak-hash                   MD5/SHA1 해시 사용 (High)
└── csap/
    ├── csap-log-retention               로그 보관 1년 미만 (High)
    ├── csap-data-residency              데이터 국외 이전 (Critical)
    └── csap-no-mfa                      MFA 미적용 (High)
```

### Scope Model (Cascading Override)

```
Organization (default rule pack from relay)
     │
     ▼
Team override: can enable additional rules or disable some
     │
     ▼
User override: further narrows for specific users
     │
     ▼
Harness override: machine-level final override

Rule precedence: Harness > User > Team > Organization
```

### DARI Protocol Changes

Extend `DLPRulePackWire` (in `internal/dariproto/dlprules.go`) with:
- `Rules []DLPRuleWire` — per-rule enabled/severity/action overrides (in addition to class toggles)
- `Scope DLPRuleScope` — Level (org/team/user/harness) + ID
- `DLPRuleScope` struct: `Level string`, `ID string`

### Implementation Order

| Phase | Issue | What | Repo |
| :---: | :--- | :--- | :--- |
| **P0** | PAT-1406 | **Align rule catalog**: standardize IDs, add authoritative `defaultSecurityRuleDefs()` with all ~60 rules | Relay |
| **P0** | PAT-1407 | **Mirror rules to harness**: create per-category rule files, update `NewScanner()`, extend `classPrefixes`, comprehensive tests | Harness |
| **P1** | PAT-1408 | **Upgrade sync protocol**: extend `DLPRulePackWire` for per-rule toggles + severity, update `applyPackToScanner` | Both repos |
| **P1** | PAT-1409 | **Scope model**: add `DLPRuleScope` to protocol, cascade logic in `applyPackToScanner`, org/team/user/harness hierarchy | Both repos |
| **P2** | PAT-1410 | **Admin UI alignment**: sync `RULE_PRESETS` with authoritative catalog, add scope selector, per-rule severity adjustment | Web |
| **P3** | FUTURE | **PII Lexicon push**: wire org-custom regex patterns from relay to harness | Both repos |

### Edge Cases (from PAT-1396)

- **Phone number false positives**: 11-digit sequences appear in code (timestamps, IDs, hashes). Require `010` prefix and reasonable grouping.
- **Bank account false positives**: Current regex too broad. Tighten to actual Korean bank formats.
- **Test data exemption**: Fake PII (`000000-0000000`, `010-0000-0000`) should pass through via allowlist.
- **Code fence context**: PII inside markdown code blocks may be intentional (validation regex demos). Flag but don't block.
- **Checksum validation**: RRN checksum dramatically reduces false positives at minimal CPU cost.

### Reuse Strategy

| Existing Component | How to Reuse |
| :--- | :--- |
| Relay's `SecurityRule` model | Already persists per-org rule toggles. Add new rule IDs to `defaultSecurityRuleDefs()`. |
| Relay's `SetRule()` API | Already lets admins toggle rules. Works with any rule ID. |
| Relay's `CheckContext()` pipeline | Already runs korean PII + secrets + injection + paths. Add new patterns to arrays. |
| Harness's `classPrefixes` | Add prefixes for new categories. |
| Harness's `applyPackToScanner()` | Already disables by prefix. Extend for per-rule toggles. |
| Harness's `NewScanner()` merge logic | Already appends: `append(append(PII(), secrets()...), injection()...)`. Add new categories. |
| Admin UI `Security.tsx` | Already has rule builder, tester, toggle, severity selector. Extend `RULE_PRESETS`. |
| Existing test infrastructure | `mustCompileRegex`, `containsFinding`, `TestClassPrefixesMatchRealRuleIDs`. |

---

## 9. Korean Date/Time/Number Formatting Awareness

**Layer**: System skill (embedded in binary, Go string constant in `internal/skill/builtins.go`)  
**Priority**: Medium — polishing feature that reinforces the "built for Korea" identity

### Description

When the model generates code for Korean-targeted applications, it frequently defaults to American formatting conventions (MM/DD/YYYY dates, 12-hour time, thousand-separator grouping). This is incorrect for Korean applications and requires manual correction by the developer.

This feature adds a system skill that provides the model with Korean formatting standards and instructs it to default to Korean conventions when the project context indicates a Korean target audience.

### Korean Formatting Standards

| Category | Korean Standard | American Default (wrong) | Example |
|----------|----------------|-------------------------|---------|
| Date (formal) | `YYYY. MM. DD.` or `YYYY-MM-DD` | `MM/DD/YYYY` | `2025. 07. 25.` |
| Date (prose) | `YYYY년 MM월 DD일` | `July 25, 2025` | `2025년 7월 25일` |
| Time | 24-hour (`14:30`) or Korean AM/PM (`오후 2:30`) | `2:30 PM` | `14:30` or `오후 2시 30분` |
| Currency | `₩1,000,000` (no decimals) | `$1,000,000.00` | `₩50,000` |
| Phone display (domestic) | `010-1234-5678` | `+82-10-1234-5678` | `010-1234-5678` |
| Phone display (international) | `+82-10-1234-5678` | Same | Context-dependent |
| Number grouping | Comma at thousands (standard) | Same | `1,000,000` |
| Number prose | Man/eok units (`1억 2천만`) | Million/billion | Business context |
| Postal code | 5 digits, no dash | ZIP+4 | `06236` |
| Address order | Large → small (province → city → district → street) | Small → large | Reverse of American convention |

### User Stories

- **Generating a date picker component**: The developer asks "add a date picker to the registration form." The model generates code with the date format `YYYY. MM. DD.` or `YYYY-MM-DD`, not `MM/DD/YYYY`. The placeholder text reads a Korean date format hint.
- **Formatting currency in a shopping cart**: The developer asks "display the total price." The model generates `₩50,000` formatting with no decimal places, not `$50,000.00` or `50,000 KRW`.
- **Generating an address form**: The model generates address fields in Korean order (postal code → province → city → district → detail address), not the reverse American order.
- **Code review catch**: A developer submits code with American date formatting for a Korean app. The compliance/formatting skill catches it and suggests the Korean format.

### Where This Lives

| Component | Location | Description |
|-----------|----------|-------------|
| **Formatting reference** | Go string constant in `internal/skill/builtins.go` | A skill body constant (like `builtinReviewBody`) that contains the Korean formatting standards table and instructions for the model. |
| **System prompt injection** | `internal/agent/` | When the project context indicates a Korean target (Korean language setting, Korean comments in code, Korean strings in i18n files), append the formatting reference to the system prompt as a low-priority directive. |
| **Code review integration** | Part of the `review` skill | Add Korean formatting checks to the review skill's checklist when the project is Korean-targeted. |

### Edge Cases

- **International projects**: Not every Korean developer is building a Korean-only app. A Korean developer building an English-language SaaS product should NOT get Korean formatting defaults. The skill should only activate when there's evidence of a Korean target audience (Korean i18n files, Korean UI strings, `.ko` locale files, Korean README).
- **Mixed locales**: Some Korean apps show dates in ISO 8601 (`YYYY-MM-DD`) for API responses but Korean format (`YYYY년 MM월 DD일`) for UI. The skill should understand this distinction and not force one format everywhere.
- **Existing formatting libraries**: If the project already uses a formatting library (date-fns, moment, dayjs with Korean locale), the model should use the library's Korean locale API, not hardcode format strings.
- **Database vs display**: Date storage should always be ISO 8601 or UTC timestamps regardless of display locale. The skill should only affect display/formatting code, not storage/query code.

### Future Upgrades

- **Korean postal code validation**: Generate valid Korean postal code validation (5-digit format, valid range checking against the Korean postal code database).
- **Korean address autocomplete integration**: When generating address forms, suggest integrating Kakao or Juso (Korean address search) APIs for autocomplete.
- **Korean business number formatting**: Auto-format business registration numbers in the `XXX-XX-XXXXX` display format.

---

## 10. Jamo-Aware Diff & Edit Matching (NFC Normalization)

**Layer**: Harness core (`internal/tool/builtin`, `internal/textutil`)  
**Priority**: Medium — reliability improvement for Korean-heavy codebases

### Description

Korean text in Unicode can be represented in two forms:
- **NFC (Composed)**: Each Korean syllable is a single code point (e.g., U+AC00 for one syllable). This is the standard form used by most Korean input methods and text processing.
- **NFD (Decomposed)**: Each Korean syllable is decomposed into 2-3 separate jamo code points (leading consonant + vowel + optional trailing consonant). macOS's HFS+ filesystem historically stored filenames in NFD.

When the model generates an `old_string` for an edit operation on Korean text, the NFC/NFD form of the model's output may not match the file's form. This causes silent edit failures — the `old_string` isn't found in the file, even though the text appears identical to the user.

The edit tool (`internal/tool/builtin/encoding_helpers.go`) already has a fuzzy matching pipeline that handles CRLF mismatches, trailing whitespace, and tab/space differences. NFC normalization should be added as another fuzzy matching stage in this same pipeline.

### The Problem Illustrated

Consider a file containing the Korean string "Hello" (in Korean). The model reads this file (NFC form), generates an edit with `old_string` containing the same text. But if the file was saved with NFD-normalized content (e.g., copied from a macOS filename or from a different text editor), the byte sequences are different even though the rendered text is identical:

- NFC: 3 code points per syllable block (compact)
- NFD: 6-9 code points for the same text (decomposed into individual jamo)

The current fuzzy matching in `applyOldStringEdit()` would fail because it doesn't normalize Unicode forms.

### User Stories

- **Editing Korean comments**: A developer asks "update the Korean comment in this file." The model reads the file, sees the Korean comment, generates an `old_string` with the comment text. If the file stores the comment in NFD (e.g., it was pasted from a macOS Finder filename), the edit fails with "old_string not found." With NFC normalization, it matches and succeeds.
- **Editing i18n files**: Korean i18n/l10n files (JSON/YAML with Korean values) are the most common place where NFC/NFD mismatches occur. The model edits a Korean translation string; the edit should work regardless of the file's Unicode normalization form.
- **macOS filename handling**: A developer working on macOS has Korean-named files. The `@`-reference system or `read_file`/`edit_file` tools need to handle NFD filenames transparently.
- **Cross-platform projects**: A project shared between macOS (NFD filenames) and Linux/Windows (NFC filenames) users can have mixed normalization. The harness should handle both transparently.

### Where This Lives

| File | Change |
|------|--------|
| **`internal/textutil/normalize.go`** (new) | `NormalizeNFC(s string) string` using `golang.org/x/text/unicode/norm`. Simple wrapper: `norm.NFC.String(s)`. Also `IsNFCNormalized(s string) bool` for diagnostics. |
| **`internal/textutil/normalize_test.go`** (new) | Tests with Korean NFD → NFC conversion, already-NFC passthrough, mixed Korean/Latin content, empty strings. |
| **`internal/tool/builtin/encoding_helpers.go`** | Add NFC normalization as a fuzzy matching mode in `fuzzyEditRanges()`. Add a new `fuzzyMode` field `normalizeNFC bool`. When enabled, both content lines and old_string lines are NFC-normalized before comparison. Insert this mode AFTER the existing trailing-whitespace and tab-expansion modes in the mode cascade. |
| **`internal/tool/builtin/fuzzy_edit_test.go`** | Add test cases: NFC/NFD mismatch on Korean strings, mixed Korean/English content, NFC-normalized file with NFD old_string and vice versa. |

### Implementation Detail

The fuzzy matching cascade in `fuzzyEditRanges()` currently tries these modes in order:

```
1. {trimTrailing: true}
2. {trimTrailing: true, expandTabs: true}
3. {stripOldReadPrefixes: true, trimTrailing: true}           // only if old has read-file prefixes
4. {stripOldReadPrefixes: true, trimTrailing: true, expandTabs: true}
```

Add NFC normalization as modes 5-8 (the same four modes but with `normalizeNFC: true`). This way, NFC normalization is only attempted after all cheaper fuzzy modes fail, minimizing the performance impact for non-Korean codebases:

```
5. {trimTrailing: true, normalizeNFC: true}
6. {trimTrailing: true, expandTabs: true, normalizeNFC: true}
7. {stripOldReadPrefixes: true, trimTrailing: true, normalizeNFC: true}
8. {stripOldReadPrefixes: true, trimTrailing: true, expandTabs: true, normalizeNFC: true}
```

The `normalizeFuzzyLine()` function adds:
```go
if mode.normalizeNFC {
    body = norm.NFC.String(body)
}
```

### Edge Cases

- **Performance**: `norm.NFC.String()` is a no-op for ASCII-only strings (returns immediately). For Korean text, it's O(n) in the string length. Since NFC modes are only tried after cheaper modes fail, the performance impact is negligible for English-only codebases and minimal for Korean codebases.
- **Already-NFC content**: The vast majority of Korean text is already NFC (standard IME output). NFC normalization of NFC content is a no-op. The modes only matter when there's a genuine NFC/NFD mismatch.
- **Diff output**: When the edit matches via NFC normalization, the fuzzy flag is set to `true` and the matched content sample shows the actual file bytes (not the normalized version). This is important for the model's next turn — it sees what the file actually contained.
- **Write-back normalization**: The replacement text (new_string) is written as-is, in whatever form the model provided (typically NFC). We do NOT normalize the existing file content — only the matching. This means after the edit, the file may contain a mix of NFC (the new content) and NFD (the untouched content). This is acceptable because NFC and NFD render identically, and full-file normalization is a separate concern.
- **Non-Korean CJK**: NFC normalization also benefits Chinese and Japanese text, but those are secondary concerns. The implementation is language-agnostic — `norm.NFC.String()` handles all Unicode.
- **File path normalization**: macOS returns NFD-normalized filenames from directory listings. When the model passes a Korean filename to `read_file` or `edit_file`, the file path itself may need NFC→NFD conversion to match the filesystem. This is a separate concern from content normalization and should be handled in `internal/fileutil` or `internal/tool/builtin/readfile.go`.

### Future Upgrades

- **Automatic NFC normalization on write**: Optionally normalize all Korean text to NFC on write, so files are always in the canonical form. This would be a config option (`normalize_unicode = true`) since some projects may intentionally use NFD.
- **Diff display with jamo granularity**: When showing a diff of Korean text, decompose changed syllable blocks to jamo level so the user can see exactly which component changed (e.g., only the final consonant of a syllable changed, not the entire block). This is a display enhancement in `internal/diff` or the TUI transcript renderer.
- **NFD filename compatibility layer**: A transparent layer that normalizes Korean filenames between NFC (user input, model output) and NFD (macOS filesystem) so tools work correctly regardless of platform.

---

## Implementation Order

Recommended implementation sequence based on dependencies and impact:

| Phase | Features | Rationale |
|-------|----------|-----------|
| **Phase 1** — Quick wins | #1 (KRW), #6 (EUC-KR), #7 (i18n audit) | Small, contained changes in existing modules. Already completed. |
| **Phase 2** — Core UX | #5 (Choseong everywhere), #10 (NFC normalization) | Core Korean UX improvements. #10 completed. |
| **Phase 3** — System skills | #4 (Compliance scanning), #9 (Formatting awareness) | Model-driven features. Both completed. |
| **Phase 4** — DLP rule catalog | #8 (DLP expansion — multi-repo catalog alignment) | Expanded scope: relay authoritative catalog → harness mirror → admin UI sync. Multi-phase, P0 priority. See Section 8 for full plan. |
| **Phase 5** — Infrastructure | #2 (HWP plugin), #3 (Korean Doc Hub) | Require separate repositories, services, and potentially hosting infrastructure. Longest lead time.
