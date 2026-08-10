# Patty Code Platform Rebrand, Korean-First UX, and Harness Architecture

**Status:** Plan-reviewed design and umbrella execution plan; detailed workstream plans pending written approval
**Date:** 2026-08-08
**Repository:** `github.com/patrickrho-patty/patty-code`
**Base product:** Patty Code
**First derived harness:** GongCode / 공코드
**Related authority:** `docs/GongCode_Master_Plan.md`

## 1. Purpose

This specification defines how to turn the current MIT-licensed Patty Code codebase into Patty Code as a clean hard fork, make Korean the primary product language, redesign the terminal experience, and establish a secure profile-and-module architecture from which GongCode and future harnesses can be produced.

This document defines the target design. It does not authorize a bulk search-and-replace. Implementation must be divided into independently verifiable plans and must preserve existing functionality except where this specification explicitly removes or changes behavior.

## 2. Confirmed Decisions

1. Patty Code is a new product and a hard fork. Upstream changes may be merged manually, but upstream compatibility is not a product requirement.
2. The product will not read or migrate Patty Code configuration, sessions, databases, paths, environment variables, or packages.
3. Patty Code is the base harness. GongCode and future public-sector or enterprise products derive from it through product profiles and modules, not repository forks.
4. Product profiles are resolved at build time. A shipped artifact has one immutable product identity and cannot switch brands at runtime.
5. Optional modules are easy to install and remove. Harness-enforced modules are signed, required, and non-disableable through supported runtime controls.
6. GongCode governance is a separate program following the phased scope and architecture in `GongCode_Master_Plan.md`.
7. GongCode Control is a separate administrative web application and service plane. Governance inspection is not an engineer-facing TUI menu.
8. Korean is the default and completeness baseline. English is optional. Retired locale support is removed completely.
9. Built-in slash commands use Korean canonical names, full 초성 aliases, and searchable English keywords. Skills and user commands retain their declared names.
10. The TUI is conversation-first. It has no permanent file browser, change inspector, governance inspector, or shortcut rail.
11. The selected visual direction is “한지 작업대”: terminal-native, restrained, Korean, and structurally distinct from Patty Code.
12. Literal Korean flag imagery is permitted in harness artwork. Official seals and claims of government endorsement remain prohibited.
13. Launch artwork appears only for interactive TUI startup and supports color, monochrome, narrow-terminal, `NO_COLOR`, quiet, and non-interactive modes.
14. Harness storage and identity are isolated intrinsically through signed identities and cryptographic binding, not only through different directories.
15. Patty Code has its own root `LICENSE`. The upstream MIT notice is retained in a separate third-party notices file.

## 3. Goals

### 3.1 Product goals

- Remove Patty Code identity from every current product and engineering surface except the legally required upstream notice.
- Preserve mature coding-agent behavior while changing identity, language, presentation, packaging, and extension composition.
- Make Korean complete across CLI, TUI, desktop, documentation, errors, help, onboarding, notifications, and built-in command descriptions.
- Permit a complete English interface for foreign workers without exposing retired locale resources or behavior.
- Make new harnesses declarative, independently branded, independently packaged, and independently secured.
- Allow derived enterprise harnesses to add governance, metering, billing, licensing, and other commercial capabilities as registered modules without placing those behaviors in the public Patty core.
- Reuse the existing extension graph, provider interception, tool interception, payload validation, required/optional runtime behavior, and replacement slots.
- Add trusted kernel seams only where the current extension contracts cannot enforce last-mile identity, transport, signing, evidence, or egress requirements.

### 3.2 Security goals

- An official GongCode build must fail closed if a required profile, module, trust root, scanner, policy service, identity binding, or audit path is missing or altered.
- Modified or locally rebuilt GongCode clients must not authenticate to GongCode Control as official trusted clients.
- One harness must not decode, import, trust, or execute another harness’s state or modules merely because files were copied into its storage directory.
- Patty Code must not collect or execute GongCode governance data paths when GongCode modules are absent.
- Product profiles must not introduce scattered product-name conditionals into shared code.

### 3.3 Delivery goals

- Produce plans detailed enough for junior engineers to execute without inventing architecture.
- Give every task exact files or discovery instructions, dependencies, tests, validation commands, rollback points, and acceptance evidence.
- Keep the repository buildable at phase boundaries.
- Use automated inventory and verification for completeness, but use semantic edits for implementation.

## 4. Non-Goals

- Maintaining automatic compatibility with Patty Code releases, data, configuration, or installation paths.
- Rewriting Git history to remove historical Patty Code references.
- Building all production GongCode government controls during the Patty Code rebrand.
- Claiming legal compliance, certification, or government endorsement from product controls alone.
- Exposing GongCode Control data, raw governance streams, or administrative audit exploration inside the engineer TUI.
- Translating source-code identifiers, protocol keys, structured event codes, file paths, or third-party technical terms merely for display language.
- Making an open-source binary literally impossible for a machine administrator to modify. Trust comes from signing, measurement, attestation, and server-side refusal.

## 5. Product Profile Architecture

### 5.1 Source layout

The target source layout is:

```text
products/
  patty/
    product.yaml
    assets/
    modules.yaml
  gongcode/
    product.yaml
    assets/
    modules.yaml
internal/
  product/
    schema
    validation
    generated profile API
    identity and capability interfaces
```

The exact filenames may follow repository conventions discovered during implementation, but these responsibilities must remain separate.

### 5.2 Profile inheritance

`gongcode` and future profiles declare `extends: patty`. Inheritance is resolved during the build:

1. Load and schema-validate the base Patty profile.
2. Apply the derived profile’s authorized overrides.
3. Reject unknown fields, duplicate capability owners, missing required modules, forbidden modules, and inconsistent identity fields.
4. Generate immutable product-profile source or build data.
5. Embed a canonical profile digest in the artifact.

No loose, user-editable product manifest is authoritative at runtime.

### 5.3 Profile responsibilities

A resolved profile owns:

- Display name and Korean/English product copy.
- Stable harness ID and edition ID.
- Executable, package, installer, service, and artifact names.
- Data roots, configuration filenames, and environment-variable prefixes.
- Desktop application IDs, protocol handlers, signing coordinates, and update channels.
- Website, documentation, support, update, registry, and telemetry endpoints.
- Launch banner, wordmark, icons, and theme tokens.
- Default and permitted locales.
- Required, optional, and prohibited modules.
- Module trust roots and accepted publishers.
- Security baseline, degraded-mode behavior, and protected operations.
- Storage and wire-format namespaces.

### 5.4 Core neutrality

Shared code uses neutral domain names such as `HarnessHomeDir`, `ProductProfile`, `HarnessID`, and `RequiredModule`. It must not use Patty-specific names for concepts shared by every derived product.

Implementation must reject patterns such as:

```go
if product == "gongcode" {
    // special behavior
}
```

Behavior varies through registered capabilities, strategies, interceptors, and profile policy.

## 6. Module Model

### 6.1 Optional modules

Optional modules:

- May be installed, enabled, disabled, upgraded, and removed.
- When removed, leave the base harness bootable with no orphaned registrations, unresolved optional dependencies, or accidental background execution.
- Preserve module-owned user data by default; destructive data purge requires a separate explicit operation.
- Use existing plugin/package discovery and extension contracts where possible.
- Cannot claim protected replacement slots unless permitted by the resolved profile.
- Cannot weaken a higher-level baseline.
- Are isolated by harness trust roots and package namespaces.

### 6.2 Harness-enforced modules

Harness-enforced modules:

- Are selected by the build profile.
- Are embedded or shipped as hash-pinned signed components.
- Cannot be disabled through UI, CLI, configuration, plugin state, or ordinary module APIs.
- Must declare dependencies, provided capabilities, intercepted points, replacement slots, version, digest, publisher, and failure mode.
- Cause protected operations to fail closed when unavailable or unhealthy.
- Produce an integrity and readiness result for attestation.

Removing or modifying such a module is treated as a security event. The application reports tampering factually; it does not render a legal conclusion.

### 6.3 Official GongCode trust chain

```text
signed launcher
  -> verifies executable signature and embedded profile digest
  -> verifies required module signatures, hashes, and dependency graph
  -> verifies device-bound harness identity
  -> establishes mTLS session with internal GongCode Control
  -> submits artifact measurements and readiness
  -> receives identity-, repository-, task-, model-, and time-bound authority
  -> enables protected operations
```

An administrator can modify or rebuild the open-source client, but cannot reproduce official signing keys, expected measurements, or device-bound credentials. GongCode Control rejects the altered client as untrusted.

### 6.4 Existing architecture to reuse

The implementation should reuse:

- Provider request and response interception.
- Tool before/after interception.
- System-prompt and context interception.
- Payload validation and blocking.
- Required versus optional extension failure behavior.
- Capability dependency graphs.
- Replacement slots and deterministic priority.
- Sidecar handshake and runtime readiness.

New trusted seams are required for:

- Authenticated harness and user envelopes.
- Last-mile transport headers, request signing, and approved endpoint enforcement.
- Cryptographic metadata and key identifiers.
- Immutable audit correlation.
- Code-edit and provenance envelopes.
- Egress gating after provider serialization.
- Control-plane attestation.

## 7. Product Identities

### 7.1 Patty Code

| Coordinate | Value |
|---|---|
| Product name | Patty Code |
| Repository and Go module | `github.com/patrickrho-patty/patty-code` |
| Harness ID | `patty` |
| CLI executable | `patty` |
| Environment prefix | `PATTY_` |
| User-data root | `.patty` |
| Configuration file | `patty.toml` |
| Package/artifact slug | `patty-code` |

### 7.2 GongCode

| Coordinate | Value |
|---|---|
| Product name | GongCode / 공코드 |
| Harness ID | `gongcode` |
| CLI executable | `gongcode` |
| Environment prefix | `GONGCODE_` |
| User-data root | `.gongcode` |

Desktop bundle IDs, code-signing identities, and production reverse-DNS coordinates require an owned domain. Release plans must treat that domain as a blocking input rather than inventing a permanent identifier.

## 8. Storage and Cross-Harness Isolation

Different harnesses are intended for different machines and use cases. Co-installation is not a primary workflow, but it must remain safe.

Each persisted envelope includes at least:

- Schema version.
- Harness ID and edition ID.
- Artifact/profile digest.
- Organization or tenant binding where applicable.
- Key ID and cryptographic protection metadata.
- Creation timestamp and writer identity where applicable.
- Integrity signature or MAC appropriate to the data class.

Each harness has independent:

- Data and cache roots.
- Configuration.
- Credentials and OS key-store entries.
- Encryption keys.
- Module registry and trust roots.
- Session namespace.
- Database and service identity.

A loader verifies the envelope before parsing sensitive content. A file copied from Patty Code to GongCode is rejected even when renamed or placed under `.gongcode`. Explicit import/export, when later supported, must be policy-approved, typed, signed, and audited.

## 9. GongCode Scope and Control Separation

### 9.1 Delivery programs

GongCode work is divided into three programs:

1. Patty Code rebrand, Korean-first UX, and harness-profile architecture.
2. Mandatory-module framework and a real GongCode distribution with fail-closed contracts.
3. Production government controls and certification work based on authoritative specifications and the phased `GongCode_Master_Plan.md`.

The first program must not fabricate government requirements. The third program requires official source texts, reviewed interpretations, schemas, cryptographic requirements, identity integration, retention rules, deployment scope, and named compliance ownership.

### 9.2 Control is separate

GongCode Control is a separate web application and backend for authorized platform, security, AI-governance, audit, and operations personnel.

The engineer-facing harness shows only information required to work safely:

- Current task and repository context.
- Active model and policy posture.
- Governance connection and trust status.
- Immediate approval requests.
- Relevant inline diffs.
- Action-specific policy explanations and denial reasons.

It does not provide a permanent administrative audit explorer.

### 9.3 Closed and air-gapped operation

Closed-network operation means no required public internet dependency. It does not mean operating without internal GongCode governance services.

Per the master plan:

- Required policy, identity, model-approval, scanner, and audit services are deployed locally inside the approved environment.
- Protected operations stop when a mandatory service is unavailable.
- The audit path may use a local signed buffer only until its configured threshold.
- A separately defined degraded mode may permit read-only explanation with a visible warning and no export.
- There is no pre-issued general bypass that permits governed edits while the internal control plane is unavailable.

## 10. Localization

### 10.1 Supported locales

- `ko`: default, canonical, and required to be complete.
- `en`: optional and complete.
- Retired locales, aliases, resources, documentation, tests, and language policies are removed.

Automatic locale detection selects Korean unless the user explicitly selects English or an approved harness policy fixes the locale.

### 10.2 Completeness boundary

Every user-facing surface must support Korean:

- CLI and TUI output.
- Desktop UI.
- Help, onboarding, settings, notices, errors, and confirmations.
- Built-in slash-command names, descriptions, arguments, and examples.
- Product documentation and embedded docs.
- Model-facing response-language policy.
- Installer, updater, diagnostics, repair, and migration-free first-run flows.
- Accessibility labels and screen-reader text.
- Release notes and support diagnostics intended for users.
- Human-readable logs and diagnostic narratives shown to users or operators.

Stable source identifiers, JSON keys, event codes, protocol fields, file paths, and third-party technical terms remain language-neutral. Human-readable log and diagnostic text uses Korean by default and English when English is selected. Structured events and errors expose stable language-neutral codes and fields plus localized display messages.

Korean documentation is canonical. English documentation is a maintained secondary translation.

### 10.3 Translation architecture

- Replace the previous locale assumptions with a typed Korean/English catalog contract.
- Make Korean the source-of-truth catalog for completeness checks.
- Generate or validate the English key set against Korean.
- Prohibit literal user-facing strings outside approved catalog and formatting layers.
- Add automated missing-key, orphan-key, placeholder-shape, and forbidden-locale-resource tests.
- Add Korean typography, width, wrapping, Unicode, IME, and terminal-cell-width tests.

## 11. Korean Slash Commands

### 11.1 Command identity

Every built-in command has:

- A stable internal command ID.
- One Korean canonical display and invocation name.
- One full 초성 alias generated or explicitly declared from that Korean name.
- Korean searchable keywords.
- English searchable keywords and alias.
- Localized descriptions, argument labels, examples, and completion items.

Example:

```text
internal ID: session.resume
Korean:      /이어하기
초성:        /ㅇㅇㅎㄱ
keywords:    resume, continue, 이어, 세션
```

When Korean is active, typing `/resume` resolves to and displays `/이어하기`. English aliases are not separately advertised in Korean listings. When English is active, English display names are shown while Korean and 초성 invocation continue to work.

### 11.2 Resolution

- Typing `/` opens the searchable palette.
- Nothing is permanently exposed in a sidebar.
- Exact canonical and exact alias matches win before keyword matches.
- Keyword matches rank deterministically.
- Ambiguous 초성 input does not execute. The palette asks for more characters or explicit selection.
- User commands and skills retain their own names.
- Existing collision priority and qualified built-in fallback are adapted to a neutral Patty namespace.
- Command handlers consume stable IDs or canonical registry entries rather than comparing localized strings throughout the codebase.

## 12. TUI and Startup Experience

### 12.1 Selected direction

The TUI uses the approved “한지 작업대” direction:

- Terminal-native layout and controls.
- Conversation and active work are the primary surface.
- Restrained ink-like base palette with limited 청·홍 accents.
- Korean spacing and hierarchy rather than decorative imitation.
- Structurally distinct from the current Patty Code presentation.

### 12.2 Removed concepts

The redesign does not add:

- Permanent left-side shortcut navigation.
- A TUI file tree.
- A permanent changed-files inspector.
- A governance or audit menu for engineers.
- A local Control Tower view.

Existing functionality remains available contextually:

- Session resume through a Korean slash command and overlay.
- Models, providers, skills, MCP, plugins, settings, help, and status through slash commands and pickers.
- File context through paths, drag/drop where supported, and `@` references.
- Proposed write diffs inline when approval or explanation is required.
- Tool progress and results inside the active transcript.

### 12.3 Responsive terminal behavior

The design must specify and test:

- Wide, standard, narrow, and minimum supported widths.
- Korean display-cell measurement.
- Window resize during streaming and approval.
- Keyboard-only operation.
- IME composition.
- Screen readers and reduced color.
- `NO_COLOR` and monochrome terminals.
- Termux/native scrollback behavior.
- Empty startup is height-bounded rather than stretched to the terminal floor; the conversation grows into available height as turns accumulate.
- Slash-palette opening does not collapse the launch stage or drop its background.

### 12.4 Launch artwork

Artwork is supplied by the compiled product profile. Korean flag imagery is permitted.

The approved launch composition centers the Taegeukgi and Patty marks as one group,
both horizontally and vertically, inside the bounded startup stage. A static bordered
`Patty Code` titlebar sits above a separate row of individually bounded status
instruments. The composer is a padded, rounded rectangle with a distinct input
background, visible insertion cursor, subdued placeholder and command hints, and no
leading `>` prompt.

The full banner appears once on interactive TUI startup. It does not appear for:

- JSON or machine-readable output.
- Piped input or output.
- CI and automation.
- `--quiet`.
- Version checks.
- Non-interactive subcommands.

The renderer selects full-color, limited-color, monochrome, and narrow variants based on terminal capability.

## 13. Methodical Rebrand

### 13.1 Measured repository baseline

The plan-review scan on 2026-08-08 found the following case-insensitive literal baseline in tracked files:

| Surface | Files with matches | Matches |
|---|---:|---:|
| `internal/` | 1,037 | 6,856 |
| `desktop/` | 307 | 2,728 |
| `docs/` | 79 | 1,567 |
| `site/` | 45 | 544 |
| `release-notes/` | 2 | 379 |
| `scripts/` | 31 | 349 |
| `workers/` | 47 | 247 |
| Repository-root files | 16 | 238 |
| `cmd/` | 18 | 96 |
| `.github/` | 12 | 96 |
| `sdk/` | 13 | 46 |
| `npm/` | 5 | 42 |
| `benchmarks/` | 3 | 17 |
| `.signpath/` | 3 | 14 |
| `tools/` | 4 | 9 |
| **Total** | **1,622** | **13,228** |

This snapshot is evidence of scope, not a fixed completion number. Implementation may expose generated, ignored, binary, or semantic identities that the initial literal scan cannot see. The final gate is zero unauthorized identity, not “13,228 replacements.”

The highest-volume ownership areas are:

| Ownership area | Files | Matches | Primary contract |
|---|---:|---:|---|
| `internal/cli` | 153 | 1,054 | CLI, TUI, commands, help, diagnostics |
| `internal/config` | 54 | 720 | paths, environment, credentials, defaults, serialization |
| `internal/repair` | 25 | 655 | update, recovery, handoff, debris, transactions |
| `desktop/frontend` | 79 | 573 | UI, localization, assets, bridges, snapshots |
| `internal/boot` | 51 | 545 | composition, prompts, extension startup, safe mode |
| `site/src` | 39 | 534 | public website, accounts, downloads, community |
| `internal/agent` | 138 | 469 | sessions, messages, model requests, persistence |
| `internal/control` | 79 | 464 | slash resolution, events, lifecycle, approvals |
| `desktop/cmd` | 15 | 251 | signing, update helpers, Windows resources |
| `desktop/build` | 7 | 245 | Linux/Windows packaging and app identity |
| `internal/i18n` | 6 | 241 | Korean/English catalogs and locale detection |
| `internal/acp` | 20 | 232 | protocol-facing names and user-facing messages |
| `internal/hook` | 10 | 217 | global/project roots and environment |
| `internal/tool` | 57 | 198 | tool metadata, managed paths, runtime messages |
| `internal/skill` | 15 | 192 | built-ins, guide identity, discovery |
| `internal/extension` | 64 | 186 | protocol, sidecars, fixtures, conformance |
| `workers/crash-report` | 30 | 175 | hosted release, crash, registry, auth services |

The initial extension count by file type includes 1,265 Go files, 78 Markdown files, 70 TypeScript files, 30 TSX files, 27 JSON files, 25 MJS files, 20 Astro files, 16 shell files, 15 SVG files, 14 SQL files, 13 YAML/YML files, and platform-specific NSIS, PowerShell, Objective-C, XML, TOML, CSS, Python, and policy files.

### 13.2 Required match ledger

Before changing identity, generate a complete machine-readable ledger outside the distributable source tree. Use tracked-file and working-tree scans so committed, newly generated, ignored build, and untracked implementation outputs are all covered.

Each ledger row contains:

| Field | Meaning |
|---|---|
| Path | Exact current path or generated-artifact coordinate |
| Line or member | Source line, archive member, symbol, schema field, or binary offset |
| Matched identity | Exact old brand, organization, domain, package, service, or locale marker |
| Semantic category | One category from section 13.4 |
| Owning workstream | The detailed plan responsible for the row |
| Disposition | Rename, neutralize, delete, regenerate, preserve legally, or investigate |
| Replacement | Exact new value or profile field |
| Dependency | Change that must land first |
| Verification | Test or inspection proving completion |
| Status | Unclassified, planned, changed, verified, or legal exception |

Required inventory commands begin with:

```bash
git ls-files -z |
  xargs -0 rg -i --count-matches --no-messages --text 'patty'

git ls-files |
  rg -i 'patty|pattycorp|(^|[._/-])(zh|zh-cn|zh-tw)([._/-]|$)'

rg -i --hidden --no-ignore \
  --glob '!.git/**' \
  'patty|pattycorp|patty\.io|urn:patty|@patty|PATTY_'
```

The detailed inventory plan must add product-specific URL, package, signing, app-ID, cookie, keyring, registry, database-binding, user-agent, protocol, and asset scans. Inventory is complete only when:

- Every discovered row has an owner and disposition.
- No row remains `unclassified` or `planned` at its workstream’s completion gate.
- Every generated artifact has been scanned after a clean build.
- Every legal exception matches an exact file, exact string, and documented reason.

### 13.3 Branded path disposition

The initial scan found 28 tracked paths containing the old product identity. Their required dispositions are:

| Current path | Disposition |
|---|---|
| `.patty/commands/review.md` | Move to `.patty/commands/review.md`; update command-root discovery and tests |
| `PATTY.md` | Rename to `PATTY.md`; rewrite as Patty instructions |
| `cmd/patty/main.go` | Move to `cmd/patty/main.go`; preserve CLI entry behavior |
| `cmd/patty/main_test.go` | Move with the entry point and update identity assertions |
| `cmd/patty-launcher/main.go` | Move to `cmd/patty-launcher/main.go`; source names from the profile |
| `cmd/patty-launcher/main_test.go` | Move with the launcher and verify profile-generated names |
| `cmd/patty-legacy-migrator/main.go` | Delete; Patty performs no upstream-data migration |
| `cmd/patty-legacy-migrator/main_test.go` | Delete or replace with clean-install/no-legacy tests |
| `cmd/patty-plugin-example/main.go` | Move to `cmd/patty-plugin-example/main.go` and update manifests |
| `desktop/build/linux/icons/hicolor/16x16/apps/patty-desktop.png` | Regenerate from the Patty profile and new artwork |
| `desktop/build/linux/icons/hicolor/24x24/apps/patty-desktop.png` | Regenerate from the Patty profile and new artwork |
| `desktop/build/linux/icons/hicolor/32x32/apps/patty-desktop.png` | Regenerate from the Patty profile and new artwork |
| `desktop/build/linux/icons/hicolor/48x48/apps/patty-desktop.png` | Regenerate from the Patty profile and new artwork |
| `desktop/build/linux/icons/hicolor/64x64/apps/patty-desktop.png` | Regenerate from the Patty profile and new artwork |
| `desktop/build/linux/icons/hicolor/128x128/apps/patty-desktop.png` | Regenerate from the Patty profile and new artwork |
| `desktop/build/linux/icons/hicolor/256x256/apps/patty-desktop.png` | Regenerate from the Patty profile and new artwork |
| `desktop/build/linux/icons/hicolor/512x512/apps/patty-desktop.png` | Regenerate from the Patty profile and new artwork |
| `desktop/build/linux/icons/hicolor/scalable/apps/patty-desktop.svg` | Regenerate from the Patty profile and inspect SVG text/metadata |
| `desktop/build/linux/io.patty.desktop.update.policy` | Replace with the final owned-domain PolicyKit ID; release-blocked until selected |
| `desktop/build/linux/patty-code.desktop` | Generate a Patty desktop entry from the compiled profile |
| `desktop/frontend/src/components/rehypeSafeKatex.ts` | Rename semantically to `rehypeSafeKatex.ts`; update imports and tests |
| `internal/skill/builtincontent/patty-guide/SKILL.md` | Replace with `patty-guide/SKILL.md`; rewrite product instructions |
| `internal/skill/patty_guide_test.go` | Rename to `patty_guide_test.go`; update expected skill identity |
| `npm/patty/package.json` | Replace with the Patty package directory and package coordinate |
| `npm/patty/bin/patty.js` | Replace with the `patty` launcher and platform-package resolver |
| `patty.example.toml` | Rename to `patty.example.toml`; remove legacy keys and examples |
| `sdk/go/examples/fullsidecar/patty-plugin.json` | Rename to a Patty plugin manifest and update SDK docs/tests |
| `sdk/go/examples/starterextension/patty-plugin.json` | Rename to a Patty plugin manifest and update SDK docs/tests |

No file move is complete until imports, build scripts, packaging manifests, embedded paths, documentation links, tests, and generated archives use the new path.

### 13.4 Inventory categories

Before editing, inventory every Patty Code reference and classify it as:

1. Go module and import path.
2. Shared source symbol.
3. Product/profile identity.
4. Filesystem path or environment variable.
5. Configuration key, file, or default.
6. Database, table, column, migration, or worker identity.
7. Log, metric, event, protocol, or schema identity.
8. Executable, package, installer, service, signing, or release identity.
9. Endpoint, domain, registry, update, crash, forum, or telemetry identity.
10. UI, localization, accessibility, or asset identity.
11. Documentation, example, test, fixture, or snapshot.
12. Legal notice or Git metadata.
13. Upstream organization or maintainer identity used operationally.
14. Binary, image, video, font, embedded metadata, or generated resource.
15. Hosted-service binding, secret name, cookie, OAuth client, email sender, or DNS identity.
16. OS integration identity: bundle, registry, shortcut, mutex, service, keyring, desktop entry, PolicyKit, MIME, or protocol handler.

Inventory tools may enumerate strings mechanically. Engineers must edit each semantic category with its owning contract and tests.

### 13.5 Upstream organization markers

The initial scan found 426 `pattycorp` occurrences across 68 tracked files. They include:

- GitHub CODEOWNERS, issue templates, workflow repositories, acknowledgements, and stale-report automation.
- GoReleaser, SignPath contracts, signing commands, and release workflows.
- Desktop updater owners, release channels, package metadata, and tests.
- NPM package ownership and publication.
- SDK module coordinates and examples.
- Public website accounts, community links, contributor data, release downloads, and headers.
- Crash-report worker release endpoints.

Every occurrence must be classified:

- Operational ownership, URL, package, signing, update, or support coordinates are replaced with Patty-controlled values.
- Upstream authorship that must be retained is moved to the appropriate legal or attribution notice.
- Third-party vendored authorship is preserved in the third party’s own notice.
- No operational system continues to authenticate, publish, download, sign, report, or link through an upstream-owned coordinate.

### 13.6 Platform identity coverage

The platform and packaging plan must resolve every row below:

| Platform | Required identity surfaces |
|---|---|
| Shared CLI | process name, argv help, shell completions, user agent, temp files, locks, PID/port files, diagnostics, config discovery |
| macOS | `.app` name, bundle ID, executable, Info.plist, URL schemes, keychain service, notifications, updater helper, launcher, icons, DMG metadata, signing and notarization |
| Windows | executable/resource names, assembly identity, AppUserModelID, registry keys, shortcuts, Start Menu, uninstall keys, mutexes, named pipes, updater/launcher/guard helpers, portable layout, installer art, SignPath inputs |
| Linux | binary, desktop file, icon names, reverse-DNS app ID, PolicyKit action, packages, MIME/protocol registration, updater/launcher units |
| Go packages | root, desktop, and SDK module paths; imports; generated bindings; examples; tests; module-aware scripts |
| NPM | package scopes, platform packages, `bin` name, metadata keys, provenance fields, registry publication, recovery/finalization scripts |
| Hosted services | worker names, D1/KV/R2 bindings, routes, domains, cookies, CORS/CSP, OAuth clients, email senders, metrics, dashboard links, health checks |

The plan may not mark a platform complete based only on a successful source build. It must inspect the installed result on that platform or a faithful packaging environment.

### 13.7 Persistence, database, and wire coverage

The persistence plan must inventory and decide:

- `.patty` project and user directories, OS application-support directories, config filenames, attachment paths, session paths, memory paths, plugin state, hook roots, cache roots, logs, crash data, update state, and recovery debris.
- `PATTY_*` variables, dotenv behavior, child-process environments, shell integration, test environments, and provider isolation.
- Keyring service/account names, key migration markers, credential-helper protocol, and remote credential storage.
- JSON, JSONL, TOML, SQL, event-wire, extension-protocol, ACP, telemetry, evidence, and provider schema identities.
- Database names, D1 bindings, tables, columns such as `enabled_patty`, indexes, migration comments, registry metadata, deployment commands, and dashboard queries.
- Metrics, tracing resources, log attributes, user agents, HTTP headers, cookies, cache keys, URNs, URL schemes, and correlation IDs.
- Update manifests, version pointers, release unit layouts, signature payloads, checksum/provenance fields, and rollback state.

Because Patty Code is a clean break:

- New Patty state uses only Patty or neutral schema identities.
- No runtime fallback reads upstream paths or fields.
- Upstream migration binaries and compatibility payloads are removed.
- Tests use empty temporary roots and assert that upstream state is ignored.
- GongCode and other profiles wrap state in signed harness-bound envelopes before sensitive decoding.

### 13.8 Hosted-service and network coverage

The hosted-service plan covers, at minimum:

- `workers/accounts`: identity routes, cookies, OAuth, email, service names, domains, tests, and Wrangler bindings.
- `workers/crash-report`: crash ingestion, release distribution, registry, authentication, schema and migration files, metrics, dashboards, package metadata, and bindings.
- `workers/forum`: identity, anti-spam, schemas, routes, domains, bindings, and tests.
- `site`: page titles, metadata, structured data, account/community flows, downloads, release channels, robots, auth redirects, contributor/publication feeds, social cards, and analytics.
- Crash, update, registry, forum, identity, documentation, telemetry, and support endpoint defaults in Go, desktop, scripts, and examples.

Every external coordinate receives one of four dispositions:

1. Replace with an owned Patty endpoint.
2. Derive from the compiled harness profile.
3. Remove because the service is not part of Patty.
4. Block release until the owned endpoint exists.

Silent fallback to an upstream service is prohibited.

### 13.9 Non-text assets and generated outputs

Text search is insufficient. The asset and release plans must inspect:

- PNG, SVG, ICO, ICNS, installer bitmaps, favicons, social cards, screenshots, videos, and theme packs.
- SVG/XML text, image metadata, embedded profiles, filenames, alt text, captions, thumbnails, and archive member names.
- Windows resources and manifests.
- Wails bindings, frontend bundles, source maps, embedded docs, generated JSON schemas, and golden fixtures.
- Go, desktop, helper, and sidecar binaries.
- NPM tarballs, Linux packages, Windows installers/portable archives, macOS application bundles and disk images.
- Update bundles, SBOMs, provenance attestations, release manifests, and signatures.

Required inspection uses the appropriate combination of:

- Filename and archive-member listing.
- `strings -a` for binaries.
- XML/SVG parsing.
- Metadata inspection.
- Visual inspection and OCR for rendered brand artwork.
- Clean-build and clean-package reproduction.

No artifact is released merely because its source directory passed a text scan.

### 13.10 Retired-locale removal coverage

Canonical Korean and English resources are explicitly preserved, including
`README.ko-KR.md`, `internal/i18n/messages_ko.go`,
`desktop/frontend/src/locales/ko.ts`, and `desktop/frontend/src/locales/en.ts`.
Retired-locale removal applies only to the former `zh` resource family:

- `README.zh-*.md` and translated documentation beneath `docs/`.
- `desktop/frontend/src/locales/zh*.ts` and their imports, aliases, and generated bundles.
- `internal/i18n/messages_zh*.go`.
- SDK example documentation with a `README.zh-*.md` name.

Removal also covers non-filename surfaces:

- Locale types, defaults, normalization, detection, and persisted preferences.
- Response-language and reasoning-language policies.
- Retired-locale-specific formatting, token units, pricing, dates, and number formats.
- Retired locale keywords, command arguments, language aliases, and help.
- Documentation generation, language navigation, SEO locale routing, sitemaps, and release-note translation.
- Retired locale catalog parity, snapshots, fixtures, golden prompts, workflow labels, and tests.
- Simplified/traditional fallback behavior in desktop, CLI, embedded docs, agents, and providers.

The replacement sequence is:

1. Add complete Korean catalogs and Korean locale plumbing.
2. Make Korean the default and completeness source.
3. Add or retain complete English parity.
4. Migrate tests and stored default assumptions to Korean/English.
5. Delete retired locale resources and code branches.
6. Run forbidden-locale and forbidden-resource scans.

Deleting retired locale files before Korean parity exists is prohibited because it would create untranslated or broken surfaces.

### 13.11 Dependency order

1. Establish the product-profile schema and generated API.
2. Move current hard-coded identities behind the API without changing behavior.
3. Add profile and module validation tests.
4. Freeze the baseline behavior suite and generate the complete match ledger.
5. Rename all Go module graphs and imports as one controlled, build-green change.
6. Rename shared identity symbols semantically while keeping behavior green.
7. Replace Patty filesystem, environment, configuration, credential, session, cache, lock, and schema identities.
8. Replace event, protocol, metric, log, HTTP, user-agent, and wire identities.
9. Add complete Korean catalogs and Korean-first locale plumbing while English remains functional.
10. Replace Korean built-in command surfaces and multi-keyword resolution.
11. Remove retired locale resources and language branches after Korean/English parity passes.
12. Replace desktop UI identity, TUI layout, launch artwork, assets, examples, docs, snapshots, and accessibility text.
13. Replace platform packaging, OS integration, updater, signing, and release identities.
14. Replace hosted-service, website, worker, database-deployment, domain, and operational identities.
15. Add derived GongCode profile, required-module trust chain, state isolation, and fail-closed tests.
16. Rebuild every supported artifact from a clean checkout and inspect its installed/package contents.
17. Sanitize transitional design and planning documents to use “upstream project,” or remove them from the distributable current tree.
18. Run whole-tree brand, locale, behavior, package, install, and clean-state gates.

Each numbered boundary is a checkpoint. The repository must compile and its proportionate tests must pass before work advances to the next boundary. A failed boundary is repaired before later identity surfaces are changed.

### 13.12 Clean break

The final product contains:

- No Patty Code legacy migrator.
- No Patty Code path fallback.
- No `PATTY_*` environment alias.
- No old command namespace.
- No old package, installer, update, or data compatibility.
- No current schema fields named for Patty Code.

Clean installation tests must begin with empty temporary user and application roots.

### 13.13 Completion accounting

The rebrand is not complete when a search command returns zero in source. It is complete only when all of the following are true:

- The current-tree ledger has zero unresolved rows.
- The 28 branded paths have their recorded disposition.
- Operational `pattycorp` coordinates are gone or profile-controlled; legal authorship is in notices.
- Korean and English parity pass and all retired locale surfaces are removed.
- Root, desktop, and SDK modules build with Patty coordinates.
- macOS, Windows, and Linux packaged identities pass platform inspection.
- Hosted Patty services use only owned or explicitly release-blocked coordinates.
- Databases, metrics, schemas, logs, cookies, URNs, headers, and wire formats contain no unauthorized upstream identity.
- Non-text assets and generated archives pass string, metadata, visual, and member-name inspection.
- Clean installs do not read upstream state.
- GongCode rejects Patty state and unsigned/modified mandatory modules.
- Only exact legal notices and Git history contain the upstream identity.

## 14. License and Attribution

- Patty Code receives its own root `LICENSE`.
- The complete upstream MIT copyright and permission notice moves to `THIRD_PARTY_NOTICES.md` or an equivalently named legal-notices file.
- New Patty Code copyright may be added for original modifications.
- Runtime UI and marketing do not display the Patty Code brand merely because the legal notice exists.
- Git history is retained.

Legal review must confirm the final notice placement before distribution.

## 15. Verification and Acceptance Gates

### 15.1 Brand gate

The scanner treats the following first-party identity families as forbidden unless an exact legal exception applies:

| Identity family | Examples the implementation scanner must cover |
|---|---|
| Product spelling and casing | `Patty Code`, `patty`, uppercase forms, joined/split display variants |
| Filesystem and configuration | `.patty`, `patty.toml`, `PATTY_*`, application-support roots, cache and lock names |
| Code and package coordinates | old Go modules/imports, NPM names/scopes, binary names, command packages, SDK examples |
| Network and protocol | `patty-code.io`, old subdomains, `urn:patty`, URL schemes, user agents, headers, cookies, OAuth IDs |
| OS integration | `io.patty.*`, bundle/app IDs, desktop entries, PolicyKit actions, registry keys, services, keyring names |
| Operational organization | `pattycorp` repositories, owners, signers, publishers, downloads, support links, hosted bindings |
| Generated and visual identity | archive members, binary strings, metadata, icons, screenshots, banners, installer art |

The inventory plan must expand each family into exact case-sensitive and case-insensitive patterns after inspecting the repository’s real forms. A zero result for only the word `patty` is not a passing brand gate.

CI and release verification scan:

- Current tracked source.
- Generated source.
- Frontend bundles and source maps.
- Go and desktop binaries.
- NPM and other package archives.
- Installers and update bundles.
- Database schemas and migrations.
- Documentation and embedded documentation.
- Assets, metadata, accessibility text, snapshots, and fixtures.
- Runtime output from smoke tests.

Allowed exceptions are explicit paths and exact expected strings for:

- `THIRD_PARTY_NOTICES.md`.
- Unmodified third-party legal notices.
- Git object history outside the distributed current tree.

The allowlist must not use broad directory exemptions for first-party code.
Transitional specifications and implementation plans are not permanent exceptions. They must be sanitized or excluded from the final distributable tree before release qualification.

### 15.2 Functional regression gates

- Go unit, integration, and race suites appropriate to each phase.
- CLI/TUI golden and interaction tests.
- Desktop frontend tests, build, and smoke tests.
- Plugin, extension graph, and sidecar contract tests.
- Provider and tool interception tests.
- Config clean-install and override tests.
- Packaging and release workflow tests.
- Cross-platform binary and application-identity inspection.

### 15.3 Localization gates

- Korean key set is complete.
- English key set matches the approved contract.
- No retired locale or language-policy resources remain.
- No unauthorized literal user-facing text remains.
- Korean IME, cell width, wrapping, truncation, and snapshot tests pass.
- Built-in commands resolve through Korean, 초성, English, and keyword inputs without ambiguous execution.

### 15.4 Harness security gates

- A missing required module blocks protected operations.
- A modified module digest is rejected.
- A modified profile is rejected.
- An unsigned derived harness cannot attest as GongCode.
- A Patty state file copied into GongCode is rejected before sensitive decoding.
- A lower-level profile or policy cannot weaken the required baseline.
- Optional Patty modules remain removable without breaking unrelated core behavior.
- GongCode emits no protected model request when mandatory internal services are unavailable.

### 15.5 Evidence

Every implementation phase produces:

- Test output.
- Brand-audit output.
- Generated profile manifest and digest.
- Package-content inventory.
- Before/after behavior notes.
- Known exceptions with owners and expiry.
- Rollback instructions.

## 16. Planning Decomposition

This file is the architecture specification and umbrella execution plan. It is not a substitute for the requested junior-executable plans. After written approval, implementation planning is split into the following complete documents:

| # | Plan document | Required repository ownership |
|---:|---|---|
| 01 | `01-inventory-and-baseline.md` | Literal and semantic inventory, match ledger, behavior baseline, test matrix, build matrix, protected legal exceptions |
| 02 | `02-product-profile-and-module-foundation.md` | New product-profile schema/API/generator, Patty profile, inheritance, validation, capability registration, optional/required module contracts |
| 03 | `03-go-module-and-core-semantic-rebrand.md` | Root/desktop/SDK modules, imports, `cmd/`, neutral shared symbols, URNs, schemas, built-ins, examples, compile checkpoints |
| 04 | `04-storage-config-schema-and-isolation.md` | `internal/config`, credentials/keyring, sessions, memory, hooks, plugins, caches, locks, telemetry, SQL, workers’ schemas, signed harness envelopes |
| 05 | `05-korean-localization-and-legacy-locale-removal.md` | Go and frontend i18n, product docs, response/patty code policy, formatting, Korean parity, English parity, and retired locale cleanup |
| 06 | `06-korean-slash-command-system.md` | Stable command IDs, Korean names, 초성 generation, Korean/English keywords, aliases, collision resolution, CLI/desktop completion/help, all built-ins |
| 07 | `07-tui-redesign-and-launch-art.md` | Bubble Tea TUI layout, transcript, contextual pickers, inline approvals/diffs, Korean IME/width, selected visual direction, profile banner renderer |
| 08 | `08-desktop-site-docs-and-assets.md` | Wails desktop identity/UI, frontend, public site, docs, embedded docs, release notes, accessibility, logos/icons/media/theme assets |
| 09 | `09-packaging-release-signing-and-os-integration.md` | GoReleaser, NPM, GitHub Actions, SignPath, update/repair/launcher helpers, macOS/Windows/Linux packages and installed identities |
| 10 | `10-hosted-services-and-network-identities.md` | Accounts, crash report, registry, forum, workers, databases/bindings, domains, OAuth/cookies/email, CORS/CSP, metrics, endpoints |
| 11 | `11-gongcode-profile-and-mandatory-modules.md` | GongCode profile, trust roots, required module graph, attestation, fail-closed runtime, Control envelope, cross-harness rejection |
| 12 | `12-release-qualification-and-master-plan-reconciliation.md` | Whole-tree and binary audit, clean builds/packages/installs, legal notices, transitional-doc sanitation, GongCode master-plan updates, evidence bundle |

Plans 01–04 establish identity and state foundations. Plans 05–08 change user-facing behavior and presentation. Plans 09–10 change distribution and external systems. Plan 11 builds the first derived secure harness. Plan 12 is the only plan allowed to declare release qualification.

### 16.1 Required structure of every detailed plan

Each plan must provide:

- Purpose, scope, and exclusions.
- Dependencies and required prior gates.
- Measured baseline for its owned surface.
- Exact files and symbols already known from the inventory.
- A prescribed Semble query for semantic discovery and an exact-literal scan for completeness.
- Small ordered tasks, each with one responsible surface and one observable outcome.
- Test-first steps where behavior changes.
- Exact edit intent: what is renamed, neutralized, deleted, generated, or preserved.
- Commands, expected exit status, and expected result.
- Cross-platform considerations.
- Rollback or recovery instructions.
- Definition of done and evidence artifacts.
- Ledger rows closed by the task and rows deliberately handed to another plan.

### 16.2 Junior task template

Every implementation task uses this structure:

1. **Objective:** one concrete behavior or identity outcome.
2. **Preconditions:** prior plan/task IDs and required green gates.
3. **Read first:** exact source, callers, tests, docs, and generated consumers.
4. **Inventory rows:** exact ledger rows owned by the task.
5. **Failing proof:** test, assertion, or inspection demonstrating the old state.
6. **Implementation:** ordered file-level edits with semantic rationale.
7. **Focused validation:** smallest relevant unit/contract tests.
8. **Broader validation:** package, subsystem, or platform suite.
9. **Artifact inspection:** generated/binary/package checks where applicable.
10. **Expected evidence:** files, logs, hashes, screenshots, manifests, or reports.
11. **Rollback:** safe reversal boundary that preserves unrelated work.
12. **Done:** exact statements that must all be true.

Tasks may not say “replace remaining occurrences,” “update related files,” or “run relevant tests” without enumerating the owned paths, discovery command, and expected verification.

### 16.3 Cross-plan gates

| Gate | Required proof |
|---|---|
| G0 Baseline | Current behavior suites recorded; full ledger generated; legal exceptions frozen |
| G1 Profile foundation | Patty profile resolves deterministically; invalid inheritance/module graphs fail |
| G2 Core identity | All Go module graphs compile; core tests pass; no mixed import namespace |
| G3 State identity | Clean Patty state works; upstream state is ignored; harness envelope tests pass |
| G4 Language | Korean completeness and English parity pass; retired locale resources and branches are absent |
| G5 Command UX | Every built-in resolves by Korean, 초성, and keywords; ambiguity never executes |
| G6 TUI/desktop | Korean-first TUI and desktop smoke tests pass at supported widths/platforms |
| G7 Distribution | Every platform package installs with correct names and no upstream operational coordinates |
| G8 Services | Patty-owned endpoints and service bindings pass integration tests; upstream fallback is impossible |
| G9 GongCode | Modified profile/module/state is rejected; mandatory-service outage blocks protected actions |
| G10 Qualification | Whole-tree, generated, binary, package, install, locale, legal, and master-plan gates pass |

## 17. Reconciliation with GongCode Master Plan

The following master-plan decisions remain authoritative:

- GongCode Control is the separate unified administration plane.
- Closed and air-gapped deployments include local governance infrastructure.
- Required services fail closed for protected actions.
- Assurance Boxes remain independently testable enforcement components.
- Coding intelligence, execution authority, and governance evidence stay separated.
- Compliance claims require deployment-specific assessment and certification.

This design adds or changes:

- GongCode derives from Patty Code through a compiled product profile.
- Optional and harness-enforced module tiers are explicit.
- Harness identity and state are cryptographically bound.
- The engineer TUI has no permanent governance navigation.
- Korean slash commands and 초성/keyword resolution are base-platform capabilities.
- Korean flag imagery is allowed by product decision; the master plan’s blanket flag prohibition must be revised while retaining the prohibition on implied endorsement.

## 18. Remaining Release Inputs

The architecture is decided. The following operational inputs may be supplied during execution without reopening it:

- Owned domain for bundle IDs and signing coordinates.
- Patty Code and GongCode final ASCII and graphic assets.
- Release signing authorities and key-custody procedures.
- Production service endpoint domains.
- Formal Korean-government specifications and reviewed control mappings for later GongCode programs.
- Named legal and compliance reviewers.

These inputs block their corresponding production release gates, not the foundational rebrand and architecture work.
