# Plan 09: Packaging, Release, Signing, and OS Integration

**Status:** Draft  
**Date:** 2026-08-08  
**Owner:** Implementation team  
**Dependencies:** Plan 03 (module rename) complete; Plan 07 (TUI assets) partially done  
**Gates:** G7 Distribution  

## 1. Purpose

Replace every platform-specific packaging identity — GoReleaser, NPM, GitHub Actions, SignPath, update helpers, macOS/Windows/Linux package names — with Patty Code coordinates.

## 2. Scope

- `.goreleaser.yaml` binary names, artifact slugs, metadata
- `cmd/patty-*` → `cmd/patty-*` (entry point renames from plan-03)
- Desktop build configs: NSIS installer names, AppImage spec, DMG settings
- GitHub release workflows: asset names, channel metadata
- PolicyKit desktop entry file
- Windows resource files, macOS Info.plist entries
- Service unit files for Linux

### Exclusions
- Actual production signing keys (need release inputs §18)
- Bundle IDs pending owned domain

## 3. Task List

### T1: Update GoReleaser config
- Binary name: `patty` → `patty`, `patty-code-launcher` → `patty-launcher`
- Artifact slug format: `patty-code-{version}-{os}-{arch}`
- Remove legacy-migrator goreleaser target

### T2: Rename launcher command
- `cmd/patty-launcher/main.go` → `cmd/patty-launcher/main.go`
- Reads executable name from Profile at runtime
- Embedded profile digest verification

### T3: Delete legacy migrator binaries
- Remove `cmd/patty-legacy-migrator/` entirely
- Remove migration code paths from `internal/repair/`

### T4: Update macOS packaging
- `.app` bundle name: `Patty Code.app` → `Patty Code.app`
- Info.plist: CFBundleName, CFBundleDisplayName from Profile
- URL scheme: `patty://` → `patty://`
- Keychain service string updated

### T5: Update Windows packaging
- NSIS installer title/name: "Patty Code" → "Patty Code"
- Application User Model ID (AppUserModelID)
- Registry key updates
- Mutex/named pipe prefixes changed to patty

### T6: Update Linux packaging
- `.desktop` file: name, icon reference from Profile
- PolicyKit action file (`io.patty.policy`)
- Package metadata in deb/rpm spec files
- Systemd service file names if applicable

### T7: Update update helper identities
- Update checker endpoint URLs derived from Profile.UpdateURL
- Channel metadata uses Profile.EditionID
- Rollback state paths use Profile.UserRoot

## 4. Definition of Done

- [ ] All platform packages install with correct Patty names
- [ ] No upstream operational coordinates remain in packaging
- [ ] Legacy migrator fully removed
- [ ] Launcher reads names from Profile
- [ ] Gate G7 proof: every platform package installs with correct names and no upstream operational coordinates