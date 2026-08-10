// Package profile — YAML loading.
package profile

import (
	"crypto/sha256"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// unmarshalDegradedMode maps the YAML string ("blockAll", "allowExplanation",
// "allowReadonly") onto the DegradedBehavior enum.
func unmarshalDegradedMode(s string) (DegradedBehavior, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "blockall", "block_all":
		return DegradedBlockAll, nil
	case "allowexplanation", "allow_explanation":
		return DegradedAllowExplanation, nil
	case "allowreadonly", "allow_readonly":
		return DegradedAllowReadonly, nil
	default:
		return DegradedBlockAll, fmt.Errorf("profile: unknown degradedMode %q", s)
	}
}

// yamlBaseline mirrors BaselineConfig with YAML-friendly field names.
type yamlBaseline struct {
	DegradedMode        string        `yaml:"degradedMode"`
	ProtectedOperations []ProtectedOp `yaml:"protectedOperations"`
}

// UnmarshalYAML lets the yaml.v3 package decode securityBaseline.
func (b *BaselineConfig) UnmarshalYAML(value *yaml.Node) error {
	var yb yamlBaseline
	if err := value.Decode(&yb); err != nil {
		return err
	}
	mode, err := unmarshalDegradedMode(yb.DegradedMode)
	if err != nil {
		return err
	}
	b.DegradedMode = mode
	b.ProtectedOperations = yb.ProtectedOperations
	return nil
}

// yamlProfile mirrors Profile with only the exported, YAML-settable fields so
// the unexported bookkeeping (resolvedDigest, extendsBase) stays private to the
// package. Field names use yaml tags matching products/<harness>/product.yaml.
type yamlProfile struct {
	HarnessID        string           `yaml:"harnessID"`
	EditionID        string           `yaml:"editionID"`
	DisplayName      LocalizedString  `yaml:"displayName"`
	ArtifactName     string           `yaml:"artifactName"`
	ExecutableName   string           `yaml:"executableName"`
	UserRoot         string           `yaml:"userRoot"`
	ConfigFilename   string           `yaml:"configFilename"`
	EnvPrefix        string           `yaml:"envPrefix"`
	DataRoot         string           `yaml:"dataRoot"`
	WebsiteURL       string           `yaml:"websiteURL"`
	DocsURL          string           `yaml:"docsURL"`
	SupportURL       string           `yaml:"supportURL"`
	UpdateURL        string           `yaml:"updateURL"`
	RegistryURL      string           `yaml:"registryURL"`
	TelemetryURL     string           `yaml:"telemetryURL"`
	APIBaseURL       string           `yaml:"apiBaseURL"`
	DesktopAppID     string           `yaml:"desktopAppID"`
	ProtocolHandler  string           `yaml:"protocolHandler"`
	KeychainService  string           `yaml:"keychainService"`
	DesktopEntryName string           `yaml:"desktopEntryName"`
	BannerArtwork    AssetCoordinate  `yaml:"bannerArtwork"`
	ColorPalette     PaletteRef       `yaml:"colorPalette"`
	LaunchArtwork    AssetCoordinate  `yaml:"launchArtwork"`
	DefaultLocale    Locale           `yaml:"defaultLocale"`
	SupportedLocales []string         `yaml:"supportedLocales"`
	RequiredModules  []ModuleRef      `yaml:"requiredModules"`
	OptionalModules  []ModuleRef      `yaml:"optionalModules"`
	ProhibitedMods   []string         `yaml:"prohibitedModules"`
	TrustRoots       []TrustRootRef   `yaml:"trustRoots"`
	AcceptedPub      []string         `yaml:"acceptedPublishers"`
	SecurityBaseline BaselineConfig   `yaml:"securityBaseline"`
	StorageNamespace string           `yaml:"storageNamespace"`
	GovernanceURL    string           `yaml:"governanceURL"`
	AuditURL         string           `yaml:"auditURL"`
	ScannerURL       string           `yaml:"scannerURL"`
	ModelApprovalURL string           `yaml:"modelApprovalURL"`
	Extends          string           `yaml:"extends"`
}

// Load reads and validates a product profile from a YAML file at path. It
// computes and stores the canonical digest from the raw bytes.
func Load(path string) (*Profile, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("profile: read %s: %w", path, err)
	}
	return Parse(raw)
}

// Parse decodes, validates, and digests a product profile from raw YAML.
func Parse(raw []byte) (*Profile, error) {
	var yp yamlProfile
	if err := yaml.Unmarshal(raw, &yp); err != nil {
		return nil, fmt.Errorf("profile: parse yaml: %w", err)
	}
	p := &Profile{
		HarnessID:        yp.HarnessID,
		EditionID:        yp.EditionID,
		DisplayName:      yp.DisplayName,
		ArtifactName:     yp.ArtifactName,
		ExecutableName:   yp.ExecutableName,
		UserRoot:         yp.UserRoot,
		ConfigFilename:   yp.ConfigFilename,
		EnvPrefix:        yp.EnvPrefix,
		DataRoot:         yp.DataRoot,
		WebsiteURL:       yp.WebsiteURL,
		DocsURL:          yp.DocsURL,
		SupportURL:       yp.SupportURL,
		UpdateURL:        yp.UpdateURL,
		RegistryURL:      yp.RegistryURL,
		TelemetryURL:     yp.TelemetryURL,
		APIBaseURL:       yp.APIBaseURL,
		DesktopAppID:     yp.DesktopAppID,
		ProtocolHandler:  yp.ProtocolHandler,
		KeychainService:  yp.KeychainService,
		DesktopEntryName: yp.DesktopEntryName,
		BannerArtwork:    yp.BannerArtwork,
		ColorPalette:     yp.ColorPalette,
		LaunchArtwork:    yp.LaunchArtwork,
		DefaultLocale:    yp.DefaultLocale,
		SupportedLocales: yp.SupportedLocales,
		RequiredModules:  yp.RequiredModules,
		OptionalModules:  yp.OptionalModules,
		ProhibitedModules: yp.ProhibitedMods,
		TrustRoots:       yp.TrustRoots,
		AcceptedPublishers: yp.AcceptedPub,
		SecurityBaseline: yp.SecurityBaseline,
		StorageNamespace: yp.StorageNamespace,
		GovernanceURL:    yp.GovernanceURL,
		AuditURL:         yp.AuditURL,
		ScannerURL:       yp.ScannerURL,
		ModelApprovalURL: yp.ModelApprovalURL,
	}
	p.extendsBase = yp.Extends
	p.resolvedDigest = sha256.Sum256(raw)
	if errs := p.Validate(); len(errs) > 0 {
		return nil, fmt.Errorf("profile: validation: %w", errs)
	}
	return p, nil
}
