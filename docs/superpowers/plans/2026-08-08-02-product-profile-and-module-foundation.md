# Plan 02: Product Profile and Module Foundation

**Status:** Draft  
**Date:** 2026-08-08  
**Owner:** Implementation team  
**Dependencies:** Plan 01 baseline complete  
**Gates:** G1 Profile foundation  

## 1. Purpose

Design and implement the product profile schema, generator API, Patty Code base profile, inheritance/resolution mechanism, and module model (optional vs harness-enforced). This architecture eliminates hardcoded product names from shared code.

## 2. Scope

- Product profile YAML schema with validation
- Profile resolution engine (base + derived = resolved profile)
- Generated immutable profile API (Go types, TypeScript bindings)
- Patty Code base profile (`products/patty/product.yaml`, `modules.yaml`)
- Capability registration system
- Optional module lifecycle (install, enable, disable, remove)
- Harness-enforced module contract (non-disableable, signed)
- Integration tests for profile resolution, validation, inheritance

### Exclusions
- Replacing all identity strings in existing code (Plan 03+ handle this)
- GongCode profile creation (Plan 11 handles this)
- Runtime signing/attestation infrastructure (Plan 11)

## 3. Target Source Layout (§5.1)

```text
internal/profile/
    schema.go        # JSON Schema + validation functions
    resolve.go       # base → derived → resolved profile chain
    types.go         # Profile struct with all coordinates
    generated.go     # Hash-pin generation (compile-time digest)
    validation.go    # Override rejection logic
    capabilities.go  # capability registry & enforcement
    testdata/        # sample profiles for testing
    *_test.go
products/
    patty/
        product.yaml         # Base product definition
        modules.yaml         # Default module composition
        assets/              # Logo, icons, banner artwork
    gongcode/                # Future plan-11
        product.yaml
        modules.yaml
        assets/
```

## 4. Profile Type Definition (§§5.3–5.4)

The resolved `Profile` must own all coordinates from §5.3:

```go
type Profile struct {
    // Identity
    HarnessID          string           // "patty" or "gongcode"
    EditionID          string           // semver or hash
    DisplayName        LocalizedString  // {"ko": "Patty Code", "en": "Patty Code"}
    ArtifactName       string           // "patty-code"
    ExecutableName     string           // "patty"
    
    // Paths & Namespaces
    UserRoot           string           // ".patty"
    ConfigFilename     string           // "patty.toml"
    EnvPrefix          string           // "PATTY_"
    DataRoot           string           // platform-appropriate app support dir
    
    // Network
    Website            string
    DocsURL            string
    SupportURL         string
    UpdateURL          string
    RegistryURL        string
    TelemetryURL       string
    APIBaseURL         string
    
    // OS Integration
    DesktopAppID       string           // reverse-DNS bundle ID
    ProtocolHandler    string           // URL scheme (e.g., "patty://")
    KeychainService    string           // OS keyring service name
    DesktopEntryName   string           // Linux .desktop file stem
    
    // Theme & Branding
    BannerArtwork      []byte           // ASCII / emoji / ANSI banner text
    ColorPalette       PaletteRef       // theme tokens
    LaunchArtwork      AssetCoordinate  // profile-sourced image/artwork
    
    // Localization
    DefaultLocale      string           // "ko"
    SupportedLocales   []string         // ["ko", "en"]
    
    // Modules
    RequiredModules    []ModuleRef
    OptionalModules    []ModuleRef
    ProhibitedModules  []string
    
    // Trust
    TrustRoots         []TrustRootRef
    AcceptedPublishers []string
    
    // Security
    SecurityBaseline   BaselineConfig   // degraded-mode behavior, protected ops
    StorageNamespace   string           // wire-format namespace prefix
}

type LocalizedString map[string]string  // e.g. {"ko": "...", "en": "..."}
type PaletteRef struct { /* theme token keys */ }
type AssetCoordinate struct { path string; hash string }
type ModuleRef struct { ID string; version string; signed bool }
type TrustRootRef struct { fingerprint string; url string }
type BaselineConfig struct {
    DegradedMode       DegradedBehavior
    ProtectedOperations []ProtectedOp
}
type ProtectedOp struct {
    Name       string
    FailClosed bool
    Description LocalizedString
}
```

## 5. Profile Inheritance Resolution (§5.2)

Resolution steps:
1. Load `products/patty/product.yaml` → validate against schema → base profile
2. Load derived profile (if extends: patty) → validate overrides
3. Merge: derived fields override base fields; unknown fields rejected
4. Validate: required modules present, no forbidden modules, consistent identities
5. Generate: produce `profile.generated.go` with compiled digest + typed accessors
6. Embed: store canonical SHA-256 digest in binary at compile time

Override rules:
- Derived profile may NOT add new capabilities it didn't inherit from base
- Derived profile may NOT weaken security baselines
- Derived profile MUST include all required modules from base unless explicitly exempted
- Duplicate capability owners across base + derived = validation error

## 6. Module Model (§6)

### Optional Module Interface
```go
type OptionalModule interface {
    ID() string
    Version() string
    Install(path string) error
    Uninstall(path string) error
    Enable() error
    Disable() error
    Status() ModuleStatus
    DependentOn() []string
}

type ModuleStatus int
const (
    Installed ModuleStatus = iota
    Enabled
    Disabled
    Removing
)
```

### Harness-Enforced Module Interface
```go
type EnforcedModule interface {
    OptionalModule
    // Extra methods only enforced modules implement
    Digest() [sha256.Size]byte
    Publisher() string
    Signature() []byte
    FailureMode() EnforcementResult
    IsEnabled() bool // always returns true
    DisableRequest() bool // always returns false
}

type EnforcementResult int
const (
    BlockAll Operations = iota
    AllowExplanation
    AllowReadonly
)
```

## 7. Task List

### T1: Design and implement Profile type + schema validation
- Read: internal/config/schema patterns, any existing YAML loaders
- Write: `internal/profile/types.go` (all structs), `internal/profile/schema.go` (validation)
- Test: `schema_test.go` — valid profile passes, invalid fails
- Expected: 0 compilation errors, full validation pass

### T2: Implement profile resolution engine
- Write: `internal/profile/resolve.go` (base → derived merge)
- Test: `resolve_test.go` — valid inheritance works, bad override rejected
- Expected: Clean inheritance for patty-only; patty→gongcode skeleton ready

### T3: Create Patty Code base product profile
- Write: `products/patty/product.yaml` (all coordinates from §7.1)
- Write: `products/patty/modules.yaml` (default module list)
- Write: minimal placeholder assets directory
- Expected: Profile resolves deterministically to a Profile struct

### T4: Implement generated profile API
- Write: template for `generated.go` (hash-pinned accessor)
- Wire into build: `go generate` runs during pre-build step
- Test: `generated_test.go` — digest matches, accessors return correct values
- Expected: Binary contains profile digest constant

### T5: Implement capability registration system
- Write: `internal/profile/capabilities.go`
- Register replacement slots (provider, tool, system prompt, etc.)
- Test: capability conflicts detected before runtime
- Expected: Neutral capability API, no product-name conditionals

### T6: Implement optional module lifecycle
- Write: `internal/module/registry.go` with install/enable/disable/remove
- Test: module removal leaves harness bootable, data preserved
- Expected: Core remains functional when all optional modules removed

### T7: Implement enforced module contract
- Write: `internal/module/enforced.go` (non-disableable wrapper)
- Test: enforced module cannot be disabled through any exposed API
- Expected: Tamper detection emits factual error, not legal conclusion

### T8: Wire Profile into config loading layer
- Read: internal/config/config.go and config discovery
- Replace hardcoded env prefixes and paths with Profile-derived values
- Test: clean install writes to `.patty/`, not `.reasonix/`
- Expected: No more hardcoded "reasonix" strings in config path resolution

## 8. Cross-Platform Considerations

- macOS: App Support root via `os.UserConfigDirectory()` + `.patty` suffix
- Windows: `%APPDATA%\.patty` via `shfolder` equivalent
- Linux: `$XDG_CONFIG_HOME/.patty` or `~/.config/.patty`
- All roots verified by envelope integrity check before sensitive decoding

## 9. Rollback Instructions

1. Delete `internal/profile/` and `products/` directories
2. Restore previous config default values from git
3. Tests affected: `config_test.go`, `i18n_test.go`, integration tests
4. Rollback preserves all prior behavior since this adds, not modifies, existing code paths

## 10. Definition of Done

- [ ] Profile schema validates correct inputs and rejects incorrect ones
- [ ] Patty profile resolves to fully-populated Profile struct
- [ ] Inheritance chain (patty → gongcode) works; invalid graphs fail
- [ ] Generated profile embeds immutable digest in binary
- [ ] Capability registration rejects conflicts
- [ ] Optional modules removable without breaking core
- [ ] Enforced modules non-disableable through all APIs
- [ ] Config layer uses Profile-derived values exclusively
- [ ] Gate G1 proof: Patty profile resolves; invalid inheritance/module graphs fail