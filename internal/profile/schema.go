// Package profile — schema validation for product profiles.
package profile

import (
	"encoding/json"
	"fmt"
	"slices"
)

// SchemaError records a single validation failure on a profile field.
type SchemaError struct {
	Path  string // dot-separated path into the profile (e.g., "defaultLocale")
	Msg   string // human-readable explanation
	Value interface{}
}

func (e SchemaError) Error() string {
	return fmt.Sprintf("profile.%s: %s (value: %v)", e.Path, e.Msg, e.Value)
}

// ValidateErrors is a collection of validation errors.
type ValidateErrors []SchemaError

func (errs ValidateErrors) Error() string {
	if len(errs) == 0 {
		return "<nil>"
	}
	var msg string
	for i, e := range errs {
		if i > 0 {
			msg += "\n"
		}
		msg += "  " + e.Error()
	}
	return msg
}

// Validate checks that p has valid values for all fields. It returns a
// ValidateErrors containing every violation found.
func (p *Profile) Validate() ValidateErrors {
	var errs ValidateErrors

	errs = append(errs, validateIdentity(p)...)
	errs = append(errs, validatePaths(p)...)
	errs = append(errs, validateNetwork(p)...)
	errs = append(errs, validateLocalization(p)...)
	errs = append(errs, validateModules(p)...)
	errs = append(errs, validateTrust(p)...)
	errs = append(errs, validateSecurity(p)...)
	errs = append(errs, validateDisplay(p)...)

	return errs
}

func validateIdentity(p *Profile) ValidateErrors {
	var errs ValidateErrors
	if p.HarnessID == "" {
		errs = append(errs, SchemaError{"harnessID", "required, must be non-empty", nil})
	}
	if p.EditionID == "" {
		errs = append(errs, SchemaError{"editionID", "required, must be non-empty", nil})
	}
	if p.DisplayName == nil || p.DisplayName[string(LocaleKo)] == "" {
		errs = append(errs, SchemaError{"displayName.ko", "Korean display name is required", nil})
	}
	if p.ExecutableName == "" {
		errs = append(errs, SchemaError{"executableName", "required, must be non-empty", nil})
	}
	if p.ArtifactName == "" {
		errs = append(errs, SchemaError{"artifactName", "required, must be non-empty", nil})
	}
	return errs
}

func validatePaths(p *Profile) ValidateErrors {
	var errs ValidateErrors
	if p.UserRoot == "" {
		errs = append(errs, SchemaError{"userRoot", "required, dot-directory name", nil})
	}
	if p.ConfigFilename == "" {
		errs = append(errs, SchemaError{"configFilename", "required", nil})
	}
	if p.EnvPrefix == "" {
		errs = append(errs, SchemaError{"envPrefix", "required, uppercase with trailing underscore", nil})
	}
	if p.StorageNamespace == "" {
		errs = append(errs, SchemaError{"storageNamespace", "required, wire-format prefix", nil})
	}
	return errs
}

func validateNetwork(p *Profile) ValidateErrors {
	if p.WebsiteURL != "" && !isValidURL(p.WebsiteURL) {
		return ValidateErrors{{"websiteURL", "must be a valid URL", p.WebsiteURL}}
	}
	return nil
}

func isValidURL(s string) bool {
	_, err := json.Marshal(s)
	return err == nil
}

func validateLocalization(p *Profile) ValidateErrors {
	var errs ValidateErrors
	if p.DefaultLocale != LocaleKo && p.DefaultLocale != LocaleEn {
		errs = append(errs, SchemaError{"defaultLocale", "must be 'ko' or 'en'", p.DefaultLocale})
	}
	if len(p.SupportedLocales) == 0 {
		errs = append(errs, SchemaError{"supportedLocales", "must list at least one supported locale", nil})
	} else if p.DefaultLocale != "" {
		found := false
		for _, l := range p.SupportedLocales {
			if l == string(p.DefaultLocale) {
				found = true
				break
			}
		}
		if !found {
			errs = append(errs, SchemaError{"defaultLocale", "not in supportedLocales list", p.DefaultLocale})
		}
	}
	return errs
}

func validateModules(p *Profile) ValidateErrors {
	var errs ValidateErrors
	seenMod := make(map[string]bool)
	for _, m := range p.RequiredModules {
		if m.ID == "" {
			errs = append(errs, SchemaError{"requiredModules[*].id", "module ID required", nil})
		}
		if seenMod[m.ID] {
			errs = append(errs, SchemaError{fmt.Sprintf("requiredModules[%s]", m.ID), "duplicate module ID", nil})
		}
		seenMod[m.ID] = true
	}
	for _, m := range p.OptionalModules {
		if m.ID == "" {
			errs = append(errs, SchemaError{"optionalModules[*].id", "module ID required", nil})
		}
		if seenMod[m.ID] {
			errs = append(errs, SchemaError{fmt.Sprintf("optionalModules[%s]", m.ID), "duplicate module ID; also declared in requiredModules", nil})
		}
		seenMod[m.ID] = true
	}
	return errs
}

func validateTrust(p *Profile) ValidateErrors {
	var errs ValidateErrors
	for i, tr := range p.TrustRoots {
		if tr.Fingerprint == "" {
			errs = append(errs, SchemaError{fmt.Sprintf("trustRoots[%d].fingerprint", i), "required", nil})
		}
	}
	return errs
}

func validateSecurity(p *Profile) ValidateErrors {
	if p.SecurityBaseline.DegradedMode < DegradedBlockAll || p.SecurityBaseline.DegradedMode > DegradedAllowReadonly {
		return ValidateErrors{{"securityBaseline.degradedMode", "invalid value", int(p.SecurityBaseline.DegradedMode)}}
	}
	return nil
}

func validateDisplay(p *Profile) ValidateErrors {
	if p.BannerArtwork.Path == "" {
		return ValidateErrors{{"bannerArtwork.path", "required, path within binary embed", nil}}
	}
	return nil
}

// MergeResult holds the output of resolving a derived profile against a base.
type MergeResult struct {
	Resolved  *Profile
	Warnings  []string
	Errors    ValidateErrors
	Overrides map[string]string // path → override value for audit
}

// ResolveDerived merges a derived profile into its base and validates the result.
// The base profile must already pass Validate(). The derived profile may have
// nil fields (meaning "use base value"). Unknown/forbidden overrides cause errors.
func ResolveDerived(base, derived *Profile) (*MergeResult, error) {
	if err := base.Validate(); err != nil {
		return nil, fmt.Errorf("base profile invalid: %w", err)
	}
	if err := deriveFieldChecks(base, derived); err != nil {
		return nil, fmt.Errorf("derived profile check failed: %w", err)
	}

	merged := base.clone()
	applied := make(map[string]string)

	// Apply overrides from derived to merged copy
	if derived.HarnessID != "" {
		merged.HarnessID = derived.HarnessID
		applied["harnessID"] = derived.HarnessID
	}
	if derived.EditionID != "" {
		merged.EditionID = derived.EditionID
		applied["editionID"] = derived.EditionID
	}
	if derived.DisplayName != nil {
		merged.DisplayName = mergeLocalStrings(merged.DisplayName, derived.DisplayName)
		applied["displayName"] = "merged local strings"
	}
	if derived.ArtifactName != "" {
		merged.ArtifactName = derived.ArtifactName
		applied["artifactName"] = derived.ArtifactName
	}
	if derived.ExecutableName != "" {
		merged.ExecutableName = derived.ExecutableName
		applied["executableName"] = derived.ExecutableName
	}
	if derived.UserRoot != "" {
		merged.UserRoot = derived.UserRoot
		applied["userRoot"] = derived.UserRoot
	}
	if derived.ConfigFilename != "" {
		merged.ConfigFilename = derived.ConfigFilename
		applied["configFilename"] = derived.ConfigFilename
	}
	if derived.EnvPrefix != "" {
		merged.EnvPrefix = derived.EnvPrefix
		applied["envPrefix"] = derived.EnvPrefix
	}
	if derived.DataRoot != "" {
		merged.DataRoot = derived.DataRoot
		applied["dataRoot"] = derived.DataRoot
	}
	if derived.WebsiteURL != "" {
		merged.WebsiteURL = derived.WebsiteURL
		applied["websiteURL"] = derived.WebsiteURL
	}
	if derived.DocsURL != "" {
		merged.DocsURL = derived.DocsURL
		applied["docsURL"] = derived.DocsURL
	}
	if derived.SupportURL != "" {
		merged.SupportURL = derived.SupportURL
		applied["supportURL"] = derived.SupportURL
	}
	if derived.UpdateURL != "" {
		merged.UpdateURL = derived.UpdateURL
		applied["updateURL"] = derived.UpdateURL
	}
	if derived.RegistryURL != "" {
		merged.RegistryURL = derived.RegistryURL
		applied["registryURL"] = derived.RegistryURL
	}
	if derived.TelemetryURL != "" {
		merged.TelemetryURL = derived.TelemetryURL
		applied["telemetryURL"] = derived.TelemetryURL
	}
	if derived.APIBaseURL != "" {
		merged.APIBaseURL = derived.APIBaseURL
		applied["apiBaseURL"] = derived.APIBaseURL
	}
	if derived.DesktopAppID != "" {
		merged.DesktopAppID = derived.DesktopAppID
		applied["desktopAppID"] = derived.DesktopAppID
	}
	if derived.ProtocolHandler != "" {
		merged.ProtocolHandler = derived.ProtocolHandler
		applied["protocolHandler"] = derived.ProtocolHandler
	}
	if derived.KeychainService != "" {
		merged.KeychainService = derived.KeychainService
		applied["keychainService"] = derived.KeychainService
	}
	if derived.DesktopEntryName != "" {
		merged.DesktopEntryName = derived.DesktopEntryName
		applied["desktopEntryName"] = derived.DesktopEntryName
	}
	if derived.ColorPalette.Name != "" {
		merged.ColorPalette = derived.ColorPalette
		applied["colorPalette"] = "replaced"
	}
	if derived.DefaultLocale != "" {
		merged.DefaultLocale = derived.DefaultLocale
		applied["defaultLocale"] = string(derived.DefaultLocale)
	}
	if len(derived.SupportedLocales) > 0 {
		merged.SupportedLocales = slices.Clone(derived.SupportedLocales)
		applied["supportedLocales"] = "replaced"
	}
	if len(derived.ProhibitedModules) > 0 {
		merged.ProhibitedModules = slices.Clone(derived.ProhibitedModules)
		applied["prohibitedModules"] = "replaced"
	}
	if len(derived.TrustRoots) > 0 {
		merged.TrustRoots = slices.Clone(derived.TrustRoots)
		applied["trustRoots"] = "replaced"
	}
	if len(derived.AcceptedPublishers) > 0 {
		merged.AcceptedPublishers = slices.Clone(derived.AcceptedPublishers)
		applied["acceptedPublishers"] = "replaced"
	}
	if derived.GovernanceURL != "" {
		merged.GovernanceURL = derived.GovernanceURL
		applied["governanceURL"] = derived.GovernanceURL
	}
	if derived.AuditURL != "" {
		merged.AuditURL = derived.AuditURL
		applied["auditURL"] = derived.AuditURL
	}
	if derived.ScannerURL != "" {
		merged.ScannerURL = derived.ScannerURL
		applied["scannerURL"] = derived.ScannerURL
	}
	if derived.ModelApprovalURL != "" {
		merged.ModelApprovalURL = derived.ModelApprovalURL
		applied["modelApprovalURL"] = derived.ModelApprovalURL
	}
	if derived.StorageNamespace != "" {
		merged.StorageNamespace = derived.StorageNamespace
		applied["storageNamespace"] = derived.StorageNamespace
	}

	// Inheritance: append modules from derived (they don't replace base's)
	merged.RequiredModules = append(merged.RequiredModules, derived.RequiredModules...)
	merged.OptionalModules = append(merged.OptionalModules, derived.OptionalModules...)
	if len(derived.RequiredModules) > 0 {
		applied["requiredModules"] = "extended"
	}
	if len(derived.OptionalModules) > 0 {
		applied["optionalModules"] = "extended"
	}

	merged.extendsBase = base.HarnessID

	// Validate the merged result
	allErrs := merged.Validate()
	if len(allErrs) > 0 {
		return &MergeResult{
			Resolved:  merged,
			Warnings:  nil,
			Errors:    allErrs,
			Overrides: applied,
		}, fmt.Errorf("merged profile validation failed: %w", allErrs)
	}

	return &MergeResult{
		Resolved:  merged,
		Warnings:  nil,
		Errors:    nil,
		Overrides: applied,
	}, nil
}

// clone creates a shallow copy of the profile. Module slices are cloned so
// mutations to the returned profile don't affect the source.
func (p *Profile) clone() *Profile {
	cp := *p
	cp.DisplayName = make(LocalizedString, len(p.DisplayName))
	for k, v := range p.DisplayName {
		cp.DisplayName[k] = v
	}
	cp.SupportedLocales = slices.Clone(p.SupportedLocales)
	cp.RequiredModules = slices.Clone(p.RequiredModules)
	cp.OptionalModules = slices.Clone(p.OptionalModules)
	cp.ProhibitedModules = slices.Clone(p.ProhibitedModules)
	cp.TrustRoots = slices.Clone(p.TrustRoots)
	cp.AcceptedPublishers = slices.Clone(p.AcceptedPublishers)
	cp.SecurityBaseline.ProtectedOperations = slices.Clone(p.SecurityBaseline.ProtectedOperations)
	return &cp
}

func mergeLocalStrings(base, override LocalizedString) LocalizedString {
	result := make(LocalizedString, len(base))
	for k, v := range base {
		result[k] = v
	}
	for k, v := range override {
		result[k] = v
	}
	return result
}

// forbiddenOverrides lists derived-profile fields that cannot weaken the
// security baseline. Product-isolation fields (userRoot, storageNamespace,
// dataRoot) are intentionally overridable so a derived harness owns its own
// state; only the security baseline is fixed by the base.
var forbiddenOverrides = []string{
	"securityBaseline.degradedMode",
}

func deriveFieldChecks(base, derived *Profile) error {
	for _, f := range forbiddenOverrides {
		switch f {
		case "securityBaseline.degradedMode":
			// Higher enum values are more permissive (weaker security).
			// Forbidding a weaker mode means derived must not exceed the base.
			if derived.SecurityBaseline.DegradedMode > base.SecurityBaseline.DegradedMode {
				return fmt.Errorf("forbidden override: degradedMode weakened from %v to %v", base.SecurityBaseline.DegradedMode, derived.SecurityBaseline.DegradedMode)
			}
		}
	}
	return nil
}
