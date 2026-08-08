# Patty Code Platform Rebrand, Korean-First UX, and Harness Architecture

**Status:** Approved design draft  
**Date:** 2026-08-08  
**Repository:** `github.com/patrickrho-patty/patty-code`  
**Base product:** Patty Code  
**First derived harness:** GongCode / 공코드  
**Related authority:** `docs/GongCode_Master_Plan.md`

## 1. Purpose

This specification defines how to turn the current MIT-licensed Reasonix codebase into Patty Code as a clean hard fork, make Korean the primary product language, redesign the terminal experience, and establish a secure profile-and-module architecture from which GongCode and future harnesses can be produced.

This document defines the target design. It does not authorize a bulk search-and-replace. Implementation must be divided into independently verifiable plans and must preserve existing functionality except where this specification explicitly removes or changes behavior.

## 2. Confirmed Decisions

1. Patty Code is a new product and a hard fork. Upstream changes may be merged manually, but upstream compatibility is not a product requirement.
2. The product will not read or migrate Reasonix configuration, sessions, databases, paths, environment variables, or packages.
3. Patty Code is the base harness. GongCode and future public-sector or enterprise products derive from it through product profiles and modules, not repository forks.
4. Product profiles are resolved at build time. A shipped artifact has one immutable product identity and cannot switch brands at runtime.
5. Optional modules are easy to install and remove. Harness-enforced modules are signed, required, and non-disableable through supported runtime controls.
6. GongCode governance is a separate program following the phased scope and architecture in `GongCode_Master_Plan.md`.
7. GongCode Control is a separate administrative web application and service plane. Governance inspection is not an engineer-facing TUI menu.
8. Korean is the default and completeness baseline. English is optional. Chinese support is removed completely.
9. Built-in slash commands use Korean canonical names, full 초성 aliases, and searchable English keywords. Skills and user commands retain their declared names.
10. The TUI is conversation-first. It has no permanent file browser, change inspector, governance inspector, or shortcut rail.
11. The selected visual direction is “한지 작업대”: terminal-native, restrained, Korean, and structurally distinct from Reasonix.
12. Literal Korean flag imagery is permitted in harness artwork. Official seals and claims of government endorsement remain prohibited.
13. Launch artwork appears only for interactive TUI startup and supports color, monochrome, narrow-terminal, `NO_COLOR`, quiet, and non-interactive modes.
14. Harness storage and identity are isolated intrinsically through signed identities and cryptographic binding, not only through different directories.
15. Patty Code has its own root `LICENSE`. The upstream MIT notice is retained in a separate third-party notices file.

## 3. Goals

### 3.1 Product goals

- Remove Reasonix identity from every current product and engineering surface except the legally required upstream notice.
- Preserve mature coding-agent behavior while changing identity, language, presentation, packaging, and extension composition.
- Make Korean complete across CLI, TUI, desktop, documentation, errors, help, onboarding, notifications, and built-in command descriptions.
- Permit a complete English interface for foreign workers without exposing Chinese resources or behavior.
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

- Maintaining automatic compatibility with Reasonix releases, data, configuration, or installation paths.
- Rewriting Git history to remove historical Reasonix references.
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
- Chinese locales, aliases, resources, documentation, tests, and language policies are removed.

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

- Replace the current English/Chinese assumption with a typed Korean/English catalog contract.
- Make Korean the source-of-truth catalog for completeness checks.
- Generate or validate the English key set against Korean.
- Prohibit literal user-facing strings outside approved catalog and formatting layers.
- Add automated missing-key, orphan-key, placeholder-shape, and forbidden-Chinese-resource tests.
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
- Structurally distinct from the current Reasonix presentation.

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

### 12.4 Launch artwork

Artwork is supplied by the compiled product profile. Korean flag imagery is permitted.

The full banner appears once on interactive TUI startup. It does not appear for:

- JSON or machine-readable output.
- Piped input or output.
- CI and automation.
- `--quiet`.
- Version checks.
- Non-interactive subcommands.

The renderer selects full-color, limited-color, monochrome, and narrow variants based on terminal capability.

## 13. Methodical Rebrand

### 13.1 Inventory categories

Before editing, inventory every Reasonix reference and classify it as:

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

Inventory tools may enumerate strings mechanically. Engineers must edit each semantic category with its owning contract and tests.

### 13.2 Dependency order

1. Establish the product-profile schema and generated API.
2. Move current hard-coded identities behind the API without changing behavior.
3. Add profile and module validation tests.
4. Rename the Go module and imports as one controlled mechanical change.
5. Rename shared identity symbols semantically.
6. Replace Patty storage, environment, configuration, wire, and database identities.
7. Replace packaging, release, signing, service, and endpoint identities.
8. Replace UI, localization, documentation, assets, examples, and snapshots.
9. Add derived GongCode profile and isolation tests.
10. Sanitize transitional design and planning documents to use “upstream project,” or remove them from the distributable current tree, so they do not create permanent brand-gate exceptions.
11. Run brand, behavior, package, and clean-install gates.

### 13.3 Clean break

The final product contains:

- No Reasonix legacy migrator.
- No Reasonix path fallback.
- No `REASONIX_*` environment alias.
- No old command namespace.
- No old package, installer, update, or data compatibility.
- No current schema fields named for Reasonix.

Clean installation tests must begin with empty temporary user and application roots.

## 14. License and Attribution

- Patty Code receives its own root `LICENSE`.
- The complete upstream MIT copyright and permission notice moves to `THIRD_PARTY_NOTICES.md` or an equivalently named legal-notices file.
- New Patty Code copyright may be added for original modifications.
- Runtime UI and marketing do not display the Reasonix brand merely because the legal notice exists.
- Git history is retained.

Legal review must confirm the final notice placement before distribution.

## 15. Verification and Acceptance Gates

### 15.1 Brand gate

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
- No Chinese locale or language-policy resources remain.
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

After this design is approved in writing, implementation planning will be split into independently executable documents:

1. Repository inventory and acceptance-baseline plan.
2. Product-profile and module-contract foundation plan.
3. Core semantic rebrand plan.
4. Storage, configuration, schema, and cross-harness isolation plan.
5. Korean-first localization and Chinese-removal plan.
6. Korean slash-command and keyword-resolution plan.
7. TUI redesign and startup artwork plan.
8. Desktop, website, documentation, and asset rebrand plan.
9. Packaging, installer, release, signing, and service-identity plan.
10. GongCode profile, mandatory-module, and Control integration foundation plan.
11. Whole-repository brand audit and release qualification plan.
12. GongCode master-plan reconciliation plan.

Each plan must provide:

- Purpose, scope, and exclusions.
- Dependencies and required prior gates.
- Exact files when known; otherwise a prescribed semantic-discovery query.
- Small tasks with one responsible surface.
- Test-first steps where behavior changes.
- Commands and expected results.
- Cross-platform considerations.
- Rollback or recovery instructions.
- Definition of done and evidence artifacts.

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
