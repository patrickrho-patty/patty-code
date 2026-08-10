# Plan 04: Storage, Config Schema, and Cross-Harness Isolation

**Status:** Draft  
**Date:** 2026-08-08  
**Owner:** Implementation team  
**Dependencies:** Plan 02 complete (G1 gate); Plan 03 partially done  
**Gates:** G3 State identity  

## 1. Purpose

Replace all Patty Code-specific filesystem paths, environment variables, configuration keys, credential names, session/cache/lock identifiers, and wire-format namespaces with values derived from the ProductProfile. Ensure harness state isolation via signed envelopes.

## 2. Scope

- `internal/config` path resolution — env vars, config filenames, home dirs
- Credential/keyring service/account naming
- Session/memory/plugin/hook/cache/lock data roots
- Database, D1 schema identities
- Event-wire protocol fields, telemetry URNs, user-agent strings
- Signed harness envelope format for sensitive persisted data
- Clean-install tests that verify no upstream paths are read

### Exclusions
- i18n catalog (Plan 05)
- TUI text changes (Plan 07)
- Desktop UI assets (Plan 08)
- Packaging names (Plan 09)

## 3. Task List

### T1: Rewrite `internal/config/config.go` path discovery
- Replace hardcoded `.patty`, `PATTY_HOME`, `patty.toml`
- Use Profile-derived UserRoot, EnvPrefix, ConfigFilename
- Test: clean install writes to `~/.patty/`, not `~/.patty/`

### T2: Update credentials store naming
- Keychain service name → derived from Profile.KeychainService
- File-store filename → `{env_prefix}_credentials.json`

### T3: Rename cache/lock/session roots
- Cache root: `<data_root>/.cache/patty`
- Lock prefix: `patty-` instead of `patty-code-`
- Session namespace: bounded by HarnessID in filename

### T4: Replace database/D1 identities
- Table/column names referencing patty code → patty-neutral or profile-derived
- Migration comments updated

### T5: Replace event-wire protocol identities
- Event types, correlation IDs, user-agent prefixes
- Metrics resource labels

### T6: Implement signed harness envelope format
- Fields: schema version, harness ID, edition ID, profile digest, integrity MAC
- Validation before decoding any sensitive content
- Rejected: copied state from another harness
- Test: envelope validation rejects cross-harness copies

### T7: Update hook/root/plug-in/discovery paths
- Plugin package roots use Profile.UserRoot
- Skill content paths neutral

### T8: Add clean-install smoke tests
- Ephemeral temp directory as home
- Start harness, verify it uses `.patty/`, not `.patty/`
- Verify `PATTY_*` env vars have no effect

## 4. Definition of Done

- [ ] No hardcoded `.patty` paths in config resolution
- [ ] No hardcoded `PATTY_` env var usage
- [ ] All storage paths derived from Profile
- [ ] Envelope rejects cross-harness copied state
- [ ] Clean-install tests pass with empty roots
- [ ] Gate G3 proof: clean Patty state works; upstream state ignored; envelope tests pass