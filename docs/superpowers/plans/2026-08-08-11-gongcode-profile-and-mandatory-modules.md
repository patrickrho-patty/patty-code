# Plan 11: GongCode Profile and Mandatory Modules

**Status:** Draft  
**Date:** 2026-08-08  
**Owner:** Implementation team  
**Dependencies:** Plan 02 complete (G1 gate); Plan 04 (state isolation) partially done  
**Gates:** G9 GongCode  

## 1. Purpose

Create the first derived harness profile (GongCode / 공코드) that extends Patty Code, defines required enforcement modules, establishes the trust chain infrastructure, and verifies cross-harness state rejection.

## 2. Scope

- Derived product profile: `products/gongcode/product.yaml` + `modules.yaml`
- Required module graph for GongCode enforcement
- Cryptographic identity binding between device and harness
- mTLS transport envelope structure
- Attestation result format for control-plane submission
- Cross-harness envelope rejection tests

### Exclusions
- Full government control implementation (Plan 12/Phase 3)
- Control Tower web application (separate product)
- Production signing key procurement

## 3. Task List

### T1: Create GongCode base profile
- `products/gongcode/product.yaml`: extends patty, overrides coordinates
- HarnessID: gongcode, ExecutableName: gongcode
- Korean display name: 공코드
- Additional governance/control URLs from profile

### T2: Define required module graph
- core.identity — device-bound cryptographic identity
- core.transport — mTLS transport headers/request signing
- core.policy — policy evaluation engine
- core.scanner — code/security scanner integration
- core.audit — immutable audit correlation
- Add these to gongcode/modules.yaml as harness-enforced

### T3: Implement signed launcher verification
- Launcher binary contains embedded certificate
- On startup, verifies its own signature and embedded profile digest
- Rejects if rebuilt without official signing keys
- Fails closed on verification failure

### T4: Implement device-bound identity
- Generate/store hardware-backed or OS-level credential
- Bound to HarnessID via cryptographic proof
- Persists across updates but not across harness copies
- New method on Profile: `IdentityBound()` → bool

### T5: Implement transport envelope
- HTTP request wrapper adds auth header, request signature, metadata
- Envelope includes: harness ID, device fingerprint, session correlation, timestamp
- Approved endpoint enforcement: only requests to Profile-defined endpoints allowed
- Signed payload prevents tampering in transit

### T6: Implement attestation result format
```json
{
  "harness": "gongcode",
  "edition": "1.0.0-abc123",
  "profileDigest": "<sha256>",
  "moduleIntegrity": { "status": "ok" | "tampered" | "missing" },
  "identityBound": true,
  "timestamp": "...",
  "signatures": [...]
}
```

### T7: Cross-harness isolation tests
- Copy a .patty state file into .gongcode directory → rejected at load
- Copy .gongcode module into patty environment → rejected
- Each harness's envelope MAC validation fails for foreign data

### T8: Fail-closed enforcement tests
- Disable a mandatory module → protected operations blocked
- Modify a module digest → attestation reports tampered
- Tamper-proof enforcement through exposed APIs

## 4. Definition of Done

- [ ] GongCode profile resolves from base patty + derived overrides
- [ ] Required modules non-disableable through all APIs
- [ ] Device-bound identity verified at startup
- [ ] Transport envelope signs every outgoing request
- [ ] Attestation result correctly reflects integrity status
- [ ] Cross-harness state copy rejected by envelope validation
- [ ] Modified module digest detected and reported
- [ ] Gate G9 proof: modified profile/module/state rejected; mandatory-service outage blocks protected actions