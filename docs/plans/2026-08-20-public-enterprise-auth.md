# Authentication Plan — Public & Enterprise Paths (patty-code + pccp)

Status: PLAN — to be broken into Linear tasks
Date: 2026-08-20
Depends on: DARI-only tier amendment (ADR 2026-08-16, amended 2026-08-19); pccp enrollment/seat machinery (live)

---

## 0. Ground truth (verified)

**Harness (patty-code):**
- Every build profile is DARI-only; generic providers excluded at boot (`internal/tier`, `internal/boot/boot.go:2262`).
- Identity = Ed25519 private key + COSE-Sign1 PPC credential (`internal/dariproto/proof.go:261`, `internal/provider/dari/dari.go:280-400`).
- DARI handshake complete: HELLO → AUTH_CHALLENGE → AUTH_PROOF (transcript-bound) → AUTH_ACK → leased sessions.
- **Missing:** any first-run login UX. `patcode setup` is still the legacy provider/keys flow. No keypair generation, no OAuth client, no enrollment redemption.

**Relay (pccp):**
- Enrollment: `POST /api/harnesses/enroll` — modes `sso` (operator session) and `code` (one-time enrollment code, harnesses B3). Seat limits (`MaxUserSeats`, `MaxHarnessSeats`) enforced **inside the enrollment transaction**, 402 on breach. Version-floor rings reject old binaries at enrollment. PPC issued by control-plane CA, 90-day validity.
- Console auth today: local login + MFA, per-org SAML + OIDC + SCIM (`internal/api/server.go:240-275`, `internal/sso/flows.go`). SSO config is **server-authoritative** — browsers never supply IdP endpoints/keys.
- **Missing:** a first-party "public" issuer trust; self-service enrollment grant endpoint; any OAuth front door for harnesses.

**Identity provider:**
- Keycloak is **deployed and live at `https://login.patty.io`** (company IdP today). Reuse the instance; do not deploy another.

**Reference flows studied:**
- Claude Code (`~/projects/claude-code-source-build/source/src/services/oauth/`): PKCE S256 + loopback listener on OS-assigned port; manual-paste fallback; refresh rotation; scope expansion re-prompt; `login_hint`/`login_method` params.
- Codex (`~/projects/codex/codex-rs/login/`): same PKCE in Rust (`pkce.rs`); real RFC 8628 device-code flow with phishing-warning prompt (`device_code_auth.rs`); token persisted to `auth.json` **+ OS keyring**; agent-identity JWT layered above user auth.

---

## 1. Public path — the spine

Design law: **OAuth is bootstrap-only.** The token's only job is to mint an enrollment; the PPC is the only model credential. No bearer token ever touches inference. This is what keeps one trust model (DARI) instead of forking into two.

### 1.1 Keycloak realm decision

**New realm `patty` on the existing login.patty.io instance.** Issuer: `https://login.patty.io/realms/patty`.

- NOT a client in the corporate realm: public self-signup must never share a user store, password policy, or brute-force surface with employee accounts.
- NOT a new instance: same Keycloak, realm isolation is the standard multi-tenancy boundary.
- Realm settings: self-registration ON, email verification, consumer password policy, brute-force detection, Google identity brokering (subscribers signed up with Google on the site), optional Apple later.
- **Website SSO continuity (the friction-killer):** patty.io's customer login uses this same realm, so the browser session from subscription checkout is already valid when the CLI opens the authorize URL → instant redirect, no consent screen (first-party client), no typing. This is the single highest-leverage UX decision and it is architectural, not cosmetic.
- Corporate realm untouched. Optional later: identity brokering corporate→public for staff.

### 1.2 Client & scopes

- Public client **`patcode`**: Standard flow + PKCE (S256), no client secret; redirect `http://localhost` (RFC 8252 — loopback port ignored); **Device Authorization Grant enabled** on the same client for headless.
- Custom client scope **`harness-enroll`** — carried in the access token; pccp requires it on the self-service enrollment endpoint. No other scopes needed at v1 (`openid email profile`).
- pccp gets a **first-party issuer trust config** (issuer URL + JWKS fetch) distinct from per-org customer SSO. Server-authoritative, Patty-operated, not org-editable.

### 1.3 Harness first-run flow (the 5-second experience)

```
patcode (fresh binary, no identity)
  → "Login required. Press Enter to log in…"          (one keypress; never surprise-open)
  → generate Ed25519 harness keypair locally           (key never leaves device)
  → PKCE S256 + loopback listener :os-port/callback
     ├─ GUI: open browser → authorize URL (login_hint if download token present,
     │        login_method=google if that's how they subscribed)
     │        tab shows "authorizing patty-code… ✓ return to terminal"
     ├─ clipboard copy of URL as fallback when open fails
     └─ headless: device-code flow + QR in terminal (verification_uri_complete
          — scan with the phone, tap once; no typing, no paste)
  → terminal flips to "✓ Logged in as <email> — provisioning" THE MOMENT the
    callback lands (listener resolves a promise — no polling spinner)
  → harness exchanges code (grant_type=authorization_code)  [device flow polls]
  → POST pccp /api/me/harness-enroll-grant  (Bearer access token, scope harness-enroll)
      server: validate token vs realm JWKS → active coding plan? → harness seat free?
      → one-time enrollment grant bound to user+harness pubkey
  → POST /api/harnesses/enroll {mode:"code", enrollment_code, public_key, …}
      existing seat transaction → Device+Harness graph → PPC (90d)
  → store identity: macOS Keychain / Windows Credential Manager / Linux secret-service
      (Codex pattern; NOT a bare file; file fallback only where no keychain)
  → discard OAuth tokens (bootstrap-only)
  → final line: "✓ Ready — plan: Pro · model: qwen3.6 · type your first task"
```

Edge cases to specify in tasks:
- Timeout → "press Enter to retry, r for QR code" (never a dead end)
- Plan lapsed → "Your plan ended — renew at patty.io/billing", harness degrades to local-only (never cliff-edge data loss)
- Seat full → "3/3 devices registered — manage at patty.io/devices" with deep link; revoking there frees the seat
- Download-link `login_hint`: website appends a short-lived signed hint token to the binary download URL → installer/first-run uses it to prefill email (Claude's login_hint pattern)
- Binary re-run on a second machine: same flow, second harness under the same user, seat count enforced

### 1.4 Renewal & lifecycle

- PPC validity 90 days → renewal is a **silent background re-enroll** while the subscription is active (server re-checks plan at renewal; no browser involved).
- Subscription cancelled → renewal denied → model calls stop with the human error; local data intact.
- Lost device → user revokes that harness on the web dashboard → next revocation epoch invalidates the PPC; the on-device key is useless.
- OAuth is touched exactly once per machine, ever — the one dimension we beat both Claude and Codex (they surface refresh failures to users; we have no long-lived model token to fail).

### 1.5 What this needs built

| # | Piece | Repo |
|---|---|---|
| P1 | Keycloak realm `patty` + client `patcode` + scope + Google broker + website SSO wiring | Keycloak/patty.io |
| P2 | `patcode login` (or setup DARI branch): keypair gen, PKCE loopback, device-code fallback, QR, keychain storage | patty-code |
| P3 | pccp first-party issuer trust + `POST /api/me/harness-enroll-grant` (plan + seat check → one-time code) | pccp |
| P4 | Enrollment redemption through the existing `mode:"code"` path + identity file/keychain layout + renewal loop | patty-code |
| P5 | Desktop app equivalent (same realm, system browser) | patty-code desktop |
| P6 | Web dashboard: device list, revoke, plan status | patty.io |

---

## 2. Enterprise path

Two planes, deliberately separate:

- **Console plane (admins):** pccp console auth — already built (local+MFA, per-org SAML/OIDC/SCIM). Enterprise decides how *humans reach the console*.
- **Harness plane (developers):** identical in every enterprise — enroll → PPC → DARI. Only *who issues the enrollment* differs.

### 2.1 Enterprise auth options (config-time choice per org)

**Option A — Patty-hosted auth ("startup mode").** Org has no IdP (or doesn't want one wired). We provision their users:
- Small orgs: users in the **`patty` public realm** as an org-scoped group? NO — keep planes clean: console-local users (already supported: `POST /api/users`, CSV import) with MFA. No Keycloak dependency for enterprises at v1.
- Larger dedicated tenants (later, per contract): Keycloak **Organizations** (v26+) or realm-per-org on login.patty.io. Decide per contract; not in v1 scope.
- Admins issue enrollment codes from the console (`POST /users/{id}/enrollment-code` — already live).

**Option B — Customer IdP federation ("enterprise mode").** Org brings LDAP / AD / SAML / OIDC (Auth0, Okta, Entra):
- Already implemented on the console plane: per-org `OrganizationSSOConfig` (SAML metadata, OIDC issuer/JWKS, SCIM user sync). Server-authoritative — validated configs only.
- Admin logs in via their IdP → mints console JWT → issues enrollment codes / grants harnesses exactly like Option A.
- Developer machines never see the corporate IdP. Their harness authenticates with the PPC only.

**Option C — admin-only configuration by Patty staff (config-as-code).** Per product decision: enterprise IdP onboarding is **NOT a self-serve wizard**. Variations (IdP metadata quirks, attribute mappings, SCIM schemas) are endless; a wizard would be a compatibility minefield.
- Format: `config.toml` (or JSON) snippet per org — provider, mode, issuer, client id, secret **ref** (never inline secrets — `client_secret_ref` pattern already in the struct), SAML metadata, attribute mappings.
- Patty employees (us) apply it via an internal admin path; validation stays server-side (`internal/sso` validators), every change audited.
- A library of per-IdP templates (Okta / Entra / Google Workspace / generic SAML / generic OIDC) with known-good defaults lives next to the schema.

### 2.2 Why public dictates enterprise (the shared spine)

Whatever the enterprise's console plane, the harness plane is byte-identical: enrollment code → PPC → DARI handshake → leased sessions → revocation epochs. The public path forces us to build the pieces enterprises then reuse for free:
- enrollment-code redemption client (P2/P4) = same client enterprises' devs run
- device/dashboard management (P6) = what enterprise admins see per-org
- renewal/revocation lifecycle = identical
- keychain identity storage = identical

Enterprise-only extras (console-side, mostly built): SCIM, audit exports, forced-version rings (C2), org policy profiles.

### 2.3 Enterprise build list

| # | Piece | Repo | Status |
|---|---|---|---|
| E1 | IdP template library + config schema + staff-run apply path | pccp | mostly exists (`internal/sso`); add templates + staff flow |
| E2 | Enrollment UX doc for enterprise devs (`patcode login --org <sso>` → same PKCE/device flow → org-scoped grant) | patty-code | same client as P2 with org param |
| E3 | Keycloak Organizations / realm-per-org evaluation for dedicated tenants | pccp+Keycloak | later, per contract |

---

## 3. Task breakdown (proposed Linear slices)

1. **AUTH-P1** Keycloak realm/client/scope + website SSO wiring (ops + web)
2. **AUTH-P2** `patcode login` — PKCE loopback + device-code + QR + keychain (patty-code)
3. **AUTH-P3** pccp issuer trust + self-service enrollment-grant endpoint (pccp)
4. **AUTH-P4** Enrollment redemption + renewal loop + degraded-mode errors (patty-code)
5. **AUTH-P5** Desktop login parity (patty-code desktop)
6. **AUTH-P6** Web device dashboard (patty.io)
7. **AUTH-E1** Enterprise IdP templates + staff config path (pccp)
8. **AUTH-E2** Org-scoped login variant + enterprise onboarding doc (patty-code)

Order: P1 ∥ P2 → P3 → P4 → (P5, P6) ; E1/E2 parallel after P2.

---

## 4. Non-goals / open decisions

- No self-serve enterprise IdP wizard (deliberate — staff-configured only).
- No OAuth token ever used for inference (architectural law).
- No second Keycloak instance.
- Open: realm name bikeshed (`patty` vs `customers`); Apple sign-in timing; whether website moves fully onto the realm now or OIDC-federates into it.
