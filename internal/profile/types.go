// Package profile provides the product profile system: schema validation,
// resolution of base/derived profiles, capability registration, and module
// lifecycle management. The Profile type owns every coordinate that varies
// across derived harnesses (Patty Code, GongCode, etc.).
package profile

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// Locale is a two-letter language tag (ko, en).
type Locale string

const (
	LocaleKo Locale = "ko"
	LocaleEn Locale = "en"
)

// LocalizedString maps locale tags to display values. The canonical source
// is always the Korean ("ko") key; English ("en") is an optional secondary
// translation.
type LocalizedString map[string]string

// String returns the Korean value if present, falling back to English, then
// the empty string.
func (l LocalizedString) String() string {
	if v, ok := l[string(LocaleKo)]; ok {
		return v
	}
	if v, ok := l[string(LocaleEn)]; ok {
		return v
	}
	return ""
}

// MustExist panics if the given locale key is not present in the map.
// Use only during initialization where a missing value indicates a bug.
func (l LocalizedString) MustExist(key string) string {
	if v, ok := l[key]; ok {
		return v
	}
	panic(fmt.Sprintf("localized string missing key %q", key))
}

// PaletteRef holds theme-token references for a product's visual identity.
type PaletteRef struct {
	Name       string // palette identifier
	Foreground string // hex color or ANSI fallback
	Background string
	AccentBlue string // 청 accent
	AccentRed  string // 홍 accent
	Monochrome []int  // monochrome terminal ANSI codes
}

// AssetCoordinate identifies artwork supplied by the compiled profile.
type AssetCoordinate struct {
	Path   string // filesystem path within the binary embed
	Digest string // SHA-256 hex digest of the asset content
}

// ModuleRef declares a required or optional module dependency.
type ModuleRef struct {
	ID      string // unique module identifier
	Version string // semver constraint
	Signed  bool   // enforced modules must be signed
}

// TrustRootRef defines a certificate fingerprint accepted as publisher trust.
type TrustRootRef struct {
	Fingerprint string // DER fingerprint hex
	URL         string // revocation / certificate URL
}

// DegradedBehavior describes how the harness operates when mandatory
// services are unavailable.
type DegradedBehavior int

const (
	DegradedBlockAll         DegradedBehavior = iota // no protected operations
	DegradedAllowExplanation                         // read-only explanations only
	DegradedAllowReadonly                            // read with visible warning
)

// ProtectedOp describes an operation that fails closed when its prerequisites
// are missing or unhealthy.
type ProtectedOp struct {
	Name        string          // stable internal name
	Description LocalizedString // user-facing explanation
	FailClosed  bool            // default: block when unavailable
}

// BaselineConfig holds security and degradation settings set by the profile.
type BaselineConfig struct {
	DegradedMode        DegradedBehavior
	ProtectedOperations []ProtectedOp
}

// Profile owns every coordinate that varies across derived harness products.
// It is resolved at build time from a base profile and one or more derived
// overrides; the resolved result is immutable at runtime.
type Profile struct {
	// Identity
	HarnessID      string          // e.g. "patty", "gongcode"
	EditionID      string          // semver or git-hash
	DisplayName    LocalizedString // {"ko": "Patty Code", "en": "Patty Code"}
	ArtifactName   string          // distributable artifact slug
	ExecutableName string          // CLI binary name

	// Paths & Namespaces
	UserRoot       string // dot-directory name, e.g. ".patty"
	ConfigFilename string // config file stem, e.g. "patty.toml"
	EnvPrefix      string // environment variable prefix, e.g. "PATTY_"
	DataRoot       string // platform-appropriate application support root

	// Network
	WebsiteURL   string
	DocsURL      string
	SupportURL   string
	UpdateURL    string
	RegistryURL  string
	TelemetryURL string
	APIBaseURL   string

	// OS Integration
	DesktopAppID     string // reverse-DNS bundle ID (macOS/Windows/Linux)
	ProtocolHandler  string // URL scheme, e.g. "patty://"
	KeychainService  string // OS keyring service account
	DesktopEntryName string // Linux .desktop file stem

	// Theme & Branding
	BannerArtwork AssetCoordinate // ASCII/emoji banner text asset
	ColorPalette  PaletteRef      // theme tokens
	LaunchArtwork AssetCoordinate // launch splash/image asset

	// Localization
	DefaultLocale    Locale
	SupportedLocales []string // ordered preference list

	// Modules
	RequiredModules   []ModuleRef
	OptionalModules   []ModuleRef
	ProhibitedModules []string // module IDs never loaded

	// Trust
	TrustRoots         []TrustRootRef
	AcceptedPublishers []string

	// Security
	SecurityBaseline BaselineConfig
	StorageNamespace string // wire-format namespace prefix

	// Endpoints for governance/control-plane where applicable
	GovernanceURL    string
	AuditURL         string
	ScannerURL       string
	ModelApprovalURL string

	// Internal
	resolvedDigest [sha256.Size]byte // SHA-256 of the canonical YAML representation
	extendsBase    string            // parent profile ID, empty if this is the base
}

// CanonicalDigest returns the SHA-256 digest computed from the profile's
// canonical YAML serialization at resolve time. Callers should compare this
// against the embedded digest in the binary to detect tampering.
func (p *Profile) CanonicalDigest() [sha256.Size]byte {
	return p.resolvedDigest
}

// DigestHex returns the digest as a hex string.
func (p *Profile) DigestHex() string {
	return hex.EncodeToString(p.resolvedDigest[:])
}

// Extends returns the parent profile ID, or empty string if this is the base.
func (p *Profile) Extends() string {
	return p.extendsBase
}

// HasCapability checks whether this profile declares the given capability ID.
func (p *Profile) HasCapability(id string) bool {
	for _, m := range p.RequiredModules {
		if m.ID == id {
			return true
		}
	}
	for _, m := range p.OptionalModules {
		if m.ID == id {
			return true
		}
	}
	return false
}
