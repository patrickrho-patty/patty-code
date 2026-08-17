# ADR: Harness Build-Time Profile Gating (public / enterprise / sovereign)

**Status:** Accepted (phase 0 + G4 + G2/G3 implemented; G1/G5–G8 and root-repo items tracked as follow-up plans)
**Date:** 2026-08-16
**Deciders:** Patrick
**Repos:** patty-code-pccp (primary), pccp (relay/CP alignment)

## Context

Patty ships one harness binary (`patcode`) that must serve three deployment
tiers with materially different trust postures:

- **public** — individual subscribers; OAuth login; auto-update; BYOK model
  presets; vendor telemetry (opt-in); billing surfaces visible.
- **enterprise** — org-managed; SAML/OIDC login against the org IdP;
  org-controlled update rings; full governance console (freezes, changeboard,
  policy, audit); no vendor telemetry without org policy.
- **sovereign / government** — air-gapped or on-prem; AD/Kerberos/smart-card
  auth; **offline media updates only**; **no vendor telemetry or crash
  upload**; **no public-internet code/dependency fetch**; procurement audits
  the binary's network surface.

Today the tier concept exists only as dead code and env escape hatches:
root-repo `DeploymentProfile`/`GetProfile()` (internal/config/profiles.go)
has zero consumers; `Organization.Profile` is write-only; the DARI-only
provider policy is bypassable via a runtime env var; air-gap is opt-in
config. A government binary built today *contains* OAuth client scaffolding,
47 public-cloud model presets, GitHub update fetchers, and telemetry code —
compiled in, merely dormant.

**Requirement:** one codebase; the bifurcation must be **compile-time**, not
runtime flags, because (a) sovereign builds must *physically lack* external
endpoints and auth mechanisms (attack surface, procurement, FIPS/CC-style
certification of a specific binary), (b) the public download should not ship
enterprise/SAML machinery, and (c) a compile-time gate cannot be
runtime-undone by env vars or compromised config.

## Decision

1. **One repository, one core, three build profiles.** Profiles are Go build
   tag sets producing distinct binaries from the same source:
   `-tags profile_public`, `-tags profile_enterprise` (default),
   `-tags profile_sovereign`.
2. **A `profile` package owns the truth.** Each build links exactly one
   `profile.Default` (compile-time constant). `profile.Lock()` semantics:
   features excluded by the linked profile **cannot be enabled at runtime**
   — config/env attempting it fails closed with a clear error.
3. **Feature modules register behind build tags.** Auth providers, update
   channels, telemetry, provider catalogs, and CLI commands are `//go:build`
   files. The agent core (loop, DARI connector, tools, local DLP, session
   machinery) is unconditionally compiled — **the governed data plane is
   profile-agnostic** (the protocol doesn't change per tier; what changes is
   who authenticates and what the binary may touch).
4. **CI builds and tests all three profiles on every change.** This is the
   non-negotiable tax of compile-time gating: unexercised gates rot
   silently. `make build-public|enterprise|sovereign` +
   `make test-profiles` run the core suite under each tag set.
5. **Gates are justified, minimal, and inventoried.** Every `//go:build`
   boundary must map to a row in the gate inventory below ("this code must
   not exist in that binary"). We gate *few things well*; anything that is
   merely *hidden* rather than *forbidden* belongs to runtime profile
   defaults (root-repo `GetProfile()`), not build tags.

## Profile semantics (summary)

| Dimension | public | enterprise | sovereign |
|---|---|---|---|
| Login (`patcode login`) | OAuth (Google/GitHub/email) | SAML/OIDC → org IdP | AD/Kerberos/smart-card (local) |
| Auto-enroll after login | invisible keygen+enroll | org-issued or SSO-authorized enroll | operator-provisioned |
| Update channel | vendor (crash.patty.io → GitHub) | org ring (relay-pushed) | **signed offline advisory only** |
| Telemetry | opt-in ping | org-policy only | **compiled out** |
| Crash upload | consent-based | org-policy | **send disabled; local inspect only** |
| Model providers | DARI relay + BYOK generic presets | DARI relay (org catalog) | DARI relay (on-prem) only — presets absent |
| MCP/plugin sources | public registry, GitHub, PyPI | org allowlist mirror | internal mirror only |
| Billing/balance UI | visible (subscriber) | chargeback (org) | absent (offline entitlement file) |
| Governance payload | thin (plan-tier policy) | full (freezes, changeboard, acks…) | full + air-gap dial policy **default-on** |
| `patcode bot` | available | available | excluded |
| serve auth_mode=none | allowed (loopback default) | warn | **refused** |

## Gate inventory (harness)

Each row: what is excluded/enabled per build and why. "—" = compiled in all
profiles. Evidence cites current code the gate replaces or wraps.

### G1 Authentication providers
- OAuth public flow — `public` only (new module `internal/auth/oauthpub`).
- SAML/OIDC enterprise enrollment — `enterprise` (+sovereign optionally
  against on-prem IdP) (`internal/auth/samlent`).
- AD/Kerberos local — `sovereign` (`internal/auth/adgov`).
- Interface: `auth.LoginProvider` chosen by `profile.Default` at link time.
- *Why:* sovereign must not contain OAuth client IDs; public must not ship
  SAML metadata handling.

### G2 Telemetry & crash reporting — `sovereign` excludes
- `internal/telemetry` daily ping/counters (client.go:22 endpoint
  crash.patty.io) — sovereign build links a no-op implementation.
- `patcode report send` + `internal/crashreport` upload — sovereign keeps
  local list/show/delete, `send` fails closed.
- Desktop crash poster (desktop/crash_app.go) — same gate.
- *Why:* stack traces and usage signals must not leave an enclave; presence
  of the endpoint is itself a procurement finding.

### G3 Update channel — `sovereign` excludes network fetch
- `patcode upgrade` pointer fetch (upgrade.go:32–38) + GitHub fallback +
  self-replace — public/enterprise only.
- Sovereign: `UpdateAdvisory` verification (internal/sovereign/airgap.go:48–86)
  promoted to the primary path + a new `patcode update import <file>` operator
  command (gap — advisory machinery exists, import UX does not).
- Remote CLI asset download (internal/releaseasset/cli.go:25) — excluded.
- *Why:* air-gapped networks forbid fetching code from vendor/GitHub.

### G4 Provider catalog — `sovereign` excludes generics
- 47 public-cloud presets (internal/config/provider_presets.go) +
  legacy deepseek defaults (load.go:346–348) — `public` only (BYOK is a
  public-tier feature); enterprise builds filter by org catalog at runtime;
  sovereign builds **omit the files**.
- `PATTY_ALLOW_GENERIC=1` env bypass of DARI-only policy
  (provider.go:1167–1172) becomes a build-time property: `public` build may
  register generics; enterprise/sovereign builds contain no generic provider
  constructors. *This is the #1 existing "already wants to be a gate" item.*
- `models_url` auto-fetch (internal/config/fetch.go:29) — sovereign: internal
  mirrors only (dial policy enforces; gate removes the default hint).
- Balance fetching `GET /user/balance` (internal/billing) — public only.
- *Why:* foreign LLM endpoints in a gov binary are both a security surface
  and a licensing/procurement problem.

### G5 Plugin/extension/dependency sources — `sovereign` internal-mirror only
- MCP public registry (internal/mcpregistry, registry.modelcontextprotocol.io),
  GitHub install sources (internal/installsource), PyPI resolution
  (internal/plugin/launcher_lock.go:37) — public: as-is; enterprise: org
  allowlist; sovereign: no compiled-in default endpoints — config must name
  internal mirrors, and the air-gap dial policy is the runtime backstop.
- *Why:* supply-chain ingress control (§15.3 planned feature).

### G6 Bot runtime — `sovereign` excludes
- `patcode bot` command + internal/botruntime + desktop bot bridge. The
  loopback control server (127.0.0.1:37913, token-gated) is safe everywhere,
  but chat-platform integrations are not a sovereign use case.
- *Why:* surface reduction.

### G7 CLI command surface
- `report`, `upgrade`, `bot`, `mcp` (public registry browse), `plugin` —
  conditioned per gates G2–G6; commands either excluded from help or fail
  closed with "not available in this build profile".
- `task`, `review`, `session`, `hook` — enterprise-forward; remain compiled
  (harmless) until a concrete sovereign/public exclusion is justified.

### G8 Enforcement posture baked in
- Air-gap dial policy (`[sovereign]` config → SetDialPolicy, boot.go:2098–2100)
  becomes **default-on in the sovereign build** (config can only *widen*
  the allowlist, never disable the policy). Env hatches
  (`PATTY_AIRGAP*`) are sovereign-excluded.
- `serve` refuses `auth_mode="none"` in sovereign; enterprise warns.
- *Why:* compile-time posture cannot be env-undone — that is the entire
  point of this ADR.

### Explicitly NOT gated (runtime defaults instead)
- Governance payload depth (freezes/changeboard/acks) — driven by relay
  policy per org, not by binary.
- Console menu visibility (below) — server-side by `Organization.Profile`.
- Plan-tier quotas (public) — relay-side policy packs.
- Anything merely cosmetic.

## Console visibility (root repo, runtime — not build tags)

Driven by `Organization.Profile` gaining read-side consumers (today
write-only, server.go:814–828):

| Console area | public | enterprise | sovereign |
|---|---|---|---|
| Dashboard, Harnesses, Projects, Repos | ✓ | ✓ | ✓ |
| Portal (subscription/billing/devices) | ✓ | — | — |
| Users, Fleet, Analytics, Code Explorer, Sessions(transcripts), Provenance, Communications, Sandboxes, Enterprise Features, Audit | — (self-service subset: own sessions metadata, own devices) | ✓ | ✓ (audit maximal) |
| Policy, Security, Compliance, Tools | — | ✓ | ✓ |
| SRE / Subscriber Mgmt / Model Infra / SCC (ops console) | Patty-internal only | internal | customer-local CP (on-prem install) |

The `liveFeatures` §33 tracker extends with a per-tier visibility column so
"honest status" stays honest per audience.

## Root-repo alignment items

1. Wire `GetProfile()`/`DeploymentProfile` (internal/config/profiles.go —
   currently dead) as the CP/relay seed of tier defaults: retention
   (30d public redacted / 90d ent / 365d sovereign), update_mode, audit level,
   KoreanPIIDetection, RequireVPN/MDM.
2. `Organization.Profile` read-side consumers for the console table above.
3. Payments (§29.9 placeholder) surfaces public-only.
4. SCIM: ship the endpoint or remove the source string (currently
   half-present, users_lifecycle.go:326).
5. PIA signed-config `requireSignature` already keys on
   `profile == "production" || "sovereign"` (loader.go:111) — align constant
   names with the harness profile package.

## Code-structure impact (concrete layout)

### Naming collision, resolved

`internal/profile` **already exists** — it is the *edition* profile system
(Patty Code vs GongCode product metadata: editions, branding, capabilities).
The deployment-tier concept gets its own package: **`internal/tier`** —
reads better than `buildprofile` at call sites and cannot be confused with
either the edition profiles or the DARI wire-protocol profiles
(`internal/dari/profiles.go`, F.13 negotiation — orthogonal).

### Package layout (current → proposed)

```
# TODAY — everything compiles into every binary
cmd/patcode/main.go          ← blank-imports ONLY dari + builtin tools, but
                                generic providers/presets/telemetry/crashreport
                                link transitively via config/boot/cli
internal/
  provider/{dari,openai,anthropic,responses}/   all in tree; generics registered
                                but blocked by env-var policy (PATTY_ALLOW_GENERIC)
  config/provider_presets.go  ← 47 public-cloud presets, ALL builds
  telemetry/                  ← vendor endpoint (crash.patty.io), runtime opt-in
  crashreport/                ← upload path, consent-gated at runtime
  billing/                    ← DeepSeek balance API, all builds
  mcpregistry/ installsource/ ← public registry + GitHub endpoints, all builds
  bot/ botruntime/            ← all builds
  profile/                    ← EDITION profiles (Patty vs GongCode) — unrelated
```

```
# PROPOSED
internal/
  tier/                       ← NEW: deployment-tier package
    tier.go                   // type Tier; Public/Enterprise/Sovereign; Allows(cap)
    lock.go                   // Lock(cap) fail-closed checks (compile-time truth)
    tags_public.go            //go:build profile_public     → Default = Public
    tags_enterprise.go        //go:build !profile_public && !profile_sovereign
    tags_sovereign.go         //go:build profile_sovereign  → Default = Sovereign
  auth/                       ← NEW (G1)
    auth.go                   // LoginProvider interface (unconditional)
    oauthpub/…                //go:build profile_public
    samlent/…                 //go:build profile_enterprise || profile_sovereign
    adgov/…                   //go:build profile_sovereign
  provider/
    dari/                     // UNCONDITIONAL — the governed data plane
    openai/ anthropic/ responses/
                              //go:build profile_public (G4: generics compile only
                                into public builds; their Register init() calls
                                vanish from other binaries)
  config/
    provider_presets.go       //go:build profile_public   (G4)
    sovereign_defaults.go     //go:build profile_sovereign (air-gap default-on)
  telemetry/
    client.go                 //go:build !profile_sovereign  (G2)
    noop.go                   //go:build profile_sovereign — same API, does nothing
  crashreport/
    upload.go                 //go:build !profile_sovereign  (G2)
  update/
    online.go                 //go:build !profile_sovereign  (G3)
    advisory_import.go        //go:build profile_sovereign — new operator command
  billing/                    //go:build profile_public      (G4)
  mcpregistry/ installsource/ // default endpoints behind tier checks + G5 tags
  bot/ botruntime/            //go:build !profile_sovereign  (G6)
```

### Mechanics that make this fit THIS codebase

1. **Provider registration is already interface-based.**
   `provider.Register(kind, New)` with `init()` per package
   (internal/provider/*/…go). Because `cmd/patcode/main.go` blank-imports
   only `provider/dari`, generics register only when transitively linked —
   adding `//go:build profile_public` to their files means enterprise and
   sovereign builds **do not compile them at all**. Minimal diff, no
   registration registry changes.

2. **Env checks become `tier` seam checks.** Today's
   `PATTY_ALLOW_GENERIC` gate in `provider.New()` (provider.go:1167–1172)
   becomes `if !tier.Default.Allows(tier.CapGenericProviders)`. In a
   sovereign build the constant is compile-time false — the branch is
   provably dead code, and the env hatch is deleted because nothing reads
   it anymore.

3. **No-op twin pattern for pervasive interfaces.** Telemetry and
   crashreport are called from many sites; instead of tagging every call
   site, each package grows a `noop.go` twin with the identical API
   (`//go:build profile_sovereign`). Call sites are untouched; the linker
   resolves to the no-op in sovereign builds.

4. **`boot.Build` gains one early step: `tier.Lock()`** — asserted right
   after config load. Sovereign binary + config attempting to enable an
   excluded capability → hard startup error. This is the fail-closed
   backstop for anything that parses from config even though its
   implementation is tagged out.

5. **Single `cmd/patcode`, three binaries via `-tags`.** No new command
   directories; `make build-public|build-enterprise|build-sovereign` pass
   the tag sets. `make test-profiles` runs the core suite under each.

### What does NOT move

`agent/`, `dariproto/`, `daricollab/`, `dlp/`, `governed/`, `workflow/`,
`tool/`, `control/`, and the whole session/provenance machinery — the
core. The diff footprint of profile-gating is ~10 packages at the edges,
zero in the middle. (This is the ADR's "the data plane is
profile-agnostic" principle made concrete.)

### Diff estimate

| Item | Size |
|---|---|
| `internal/tier` package + tests | ~150 LOC |
| Build tags on existing files | ~15 files, one-line headers |
| Noop twins (telemetry, crashreport) | 2 small files |
| `tier.Lock` boot hook | ~20 LOC |
| Makefile + CI matrix | ~100 LOC |
| Env hatches retired (ALLOW_GENERIC, PATTY_AIRGAP) | deletions |
| G1 auth providers | new feature code regardless of this ADR |

## Consequences

- **+** Sovereign binaries provably lack external endpoints (grep-able,
  auditor-friendly). Public download excludes enterprise auth and gov
  machinery. Enterprise posture can't be env-downgraded.
- **+** The DARI data plane stays identical across tiers — one protocol, one
  conformance suite.
- **−** Build/test matrix triples: CI must compile and run core tests under
  all three tag sets on every PR (accepted tax; the alternative is silent
  gate rot).
- **−** Cross-profile bug classes: code compiled only under one tag gets no
  exercise unless CI covers it — mitigated by the same matrix rule.
- **Migration:** today's env escape hatches (`PATTY_ALLOW_GENERIC`,
  `PATTY_AIRGAP*`) retire into build properties; runtime `[sovereign]`
  config remains as *refinement* (allowlist contents), not as the on-switch.
- **Docs:** `[sovereign]` must be documented in patty.example.toml and the
  config renderer (currently undocumented).

## Implementation plan (ordered)

0. **Code-structure groundwork** — create `internal/tier` (resolving the
   `internal/profile` edition-package collision documented above), wire the
   Makefile/CI matrix, add `tier.Lock` to boot. Matrix first, gates second —
   never gate what CI can't see.
1. G4 (provider catalog + generic-provider constructors) — highest-value,
   removes the `PATTY_ALLOW_GENERIC` env hatch.
2. G2/G3 (telemetry, crash send, upgrade fetch) — sovereign no-ops +
   `patcode update import`.
3. G1 auth provider interface + OAuth public flow (pairs with the public
   login/enrollment design — OAuth above, enrollment below, per the auth
   ADR discussion).
4. G5–G8 enforcement posture + command surfacing.
5. Root-repo alignment items (profile seeding, console visibility,
   SCIM/payments).

## Deviations from the original ADR

These three deliberate deviations honored the intent of the ADR while
lowering execution risk. All of them preserve the G-row outcome; the
mechanics differ.

- **G2 telemetry — egress-twin vs whole-file noop.** The ADR proposed a
  full `client.go` no-op twin (`//go:build profile_sovereign`). The
  implementation kept the live `client.go` unconditional and added a
  `noop_egress.go` egress-only twin (`Send`/`Flush` no-ops) so the
  counter-bucket code paths stay reachable from any profile. Rationale:
  a whole-file noop would leave dangling `init()` symbols and the
  `Enabled` gate would have to be reconstructed by every consumer. The
  twin keeps the public API uniform; only the outbound transport
  vanishes.
- **`openaiapi` leaf package extraction (G4 prep).** The ADR did not
  mention `internal/openaiapi` as a separate leaf. During G4 we
  extracted a shared helper package from `provider/openai` because
  `internal/cli` and `internal/config` import the response-shaping
  helpers and would otherwise have been transitively gated behind the
  `profile_public` tag. The leaf is `//go:build !profile_sovereign` for
  the Anthropic-translation helper and unconditional for the shape
  parser; downstream consumers are unchanged.
- **Exported-surface drift in `openaiapi` (G4).** Six capitalization-only
  renames (`ID` → `Id`, `URL` → `Url`, etc.) landed during the leaf
  extraction to match the rest of the harness. The ADR was silent on
  identifier style; the rename keeps linter noise out of the public
  build, but downstream importers (none in-tree) would have to follow.
  This is acceptable inside this monorepo.
