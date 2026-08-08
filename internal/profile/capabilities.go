// Package profile — capability registration and enforcement.
package profile

import (
	"fmt"
	"sync"
)

// CapabilityType identifies the kind of capability a module provides.
type CapabilityType int

const (
	CapabilityProvider   CapabilityType = iota // provider request/response interception
	CapabilityTool                             // tool before/after hooks
	CapabilitySystemPrompt                     // system prompt / context composition
	CapabilityPayloadValidation                // payload blocking
	CapabilityReplacementSlot                  // last-mile replacement for core path
	CapabilityAttestation                      // security attestation
	CapabilityEgress                           // egress gating
	CapabilityAudit                            // audit correlation
)

func (t CapabilityType) String() string {
	switch t {
	case CapabilityProvider:
		return "provider"
	case CapabilityTool:
		return "tool"
	case CapabilitySystemPrompt:
		return "system_prompt"
	case CapabilityPayloadValidation:
		return "payload_validation"
	case CapabilityReplacementSlot:
		return "replacement_slot"
	case CapabilityAttestation:
		return "attestation"
	case CapabilityEgress:
		return "egress"
	case CapabilityAudit:
		return "audit"
	default:
		return fmt.Sprintf("capability_%d", int(t))
	}
}

// Capability declares a replaceable or extensible point in the harness.
type Capability struct {
	ID             string           // unique identifier
	Type           CapabilityType   // category
	Description    LocalizedString  // user-facing description
	Mandatory      bool             // missing this = fail closed
	Priority       int              // deterministic priority (higher wins)
	SupportedLocales []string      // locales that register this capability
}

// CapReg manages all registered capabilities across product profiles.
type CapReg struct {
	mu     sync.RWMutex
	caps   map[string]*Capability // id → capability
	owners map[string][]string    // capability → module IDs that provide it
}

// NewCapReg creates an empty capability registry.
func NewCapReg() *CapReg {
	return &CapReg{
		caps:   make(map[string]*Capability),
		owners: make(map[string][]string),
	}
}

// Register adds a capability to the registry. Returns an error if the ID
// already exists with a different owner or if mandatory fields are empty.
func (r *CapReg) Register(cap *Capability, ownerID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if cap.ID == "" {
		return fmt.Errorf("capability ID required")
	}

	if existing, ok := r.caps[cap.ID]; ok {
		for _, o := range r.owners[cap.ID] {
			if o == ownerID {
				return fmt.Errorf("capability %q already registered by this module", cap.ID)
			}
		}
		return fmt.Errorf("capability %q conflict: claimed by %s and %s", cap.ID, existingOwner(r, existing), ownerID)
	}

	r.caps[cap.ID] = cap
	r.owners[cap.ID] = append(r.owners[cap.ID], ownerID)
	return nil
}

func existingOwner(r *CapReg, cap *Capability) string {
	for _, o := range r.owners[cap.ID] {
		return o
	}
	return "<unknown>"
}

// Unregister removes a capability provided by the given module.
func (r *CapReg) Unregister(capID, ownerID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	cap, ok := r.caps[capID]
	if !ok {
		return fmt.Errorf("capability %q not found", capID)
	}

	owners := r.owners[capID]
	newOwners := make([]string, 0, len(owners))
	found := false
	for _, o := range owners {
		if o == ownerID {
			found = true
		} else {
			newOwners = append(newOwners, o)
		}
	}
	if !found {
		return fmt.Errorf("owner %q does not provide capability %q", ownerID, capID)
	}

	if cap.Mandatory && len(newOwners) == 0 {
		return fmt.Errorf("cannot unregister mandatory capability %q from all modules", capID)
	}

	r.owners[capID] = newOwners
	if len(newOwners) == 0 {
		delete(r.caps, capID)
		delete(r.owners, capID)
	}

	return nil
}

// Get returns the capability with the given ID.
func (r *CapReg) Get(id string) (*Capability, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, ok := r.caps[id]
	return c, ok
}

// Owners returns the module IDs that provide the given capability.
func (r *CapReg) Owners(id string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	owners := r.owners[id]
	cp := make([]string, len(owners))
	copy(cp, owners)
	return cp
}

// ForProfile registers all default capabilities declared in the profile's
// required and optional modules. This is called during profile resolution.
func (r *CapReg) ForProfile(p *Profile) error {
	for _, m := range p.RequiredModules {
		capMap := defaultCapabilitiesForModule(m.ID)
		for _, cap := range capMap {
			if err := r.Register(cap, m.ID); err != nil {
				return err
			}
		}
	}
	for _, m := range p.OptionalModules {
		capMap := defaultCapabilitiesForModule(m.ID)
		for _, cap := range capMap {
			// Optional module conflicts are warnings, not fatal
			_ = r.Register(cap, m.ID)
		}
	}
	return nil
}

// defaultCapabilitiesForModule maps known module IDs to their default
// capability slots. This is the centralized source — no scattered conditionals.
func defaultCapabilitiesForModule(moduleID string) map[string]*Capability {
	m := make(map[string]*Capability)

	switch {
	case moduleID == "core.tui":
		m["tui.render"] = &Capability{
			ID: "tui.render", Type: CapabilityReplacementSlot, Priority: 100,
			Description: LocalizedString{"ko": "TUI 렌더링", "en": "TUI rendering"},
			Mandatory: true, SupportedLocales: []string{"ko", "en"},
		}
		m["tui.keymap"] = &Capability{
			ID: "tui.keymap", Type: CapabilityReplacementSlot, Priority: 90,
			Description: LocalizedString{"ko": "TUI 키맵", "en": "TUI keymap"},
			Mandatory: true, SupportedLocales: []string{"ko", "en"},
		}
	case moduleID == "core.i18n":
		m["i18n.locale"] = &Capability{
			ID: "i18n.locale", Type: CapabilityReplacementSlot, Priority: 200,
			Description: LocalizedString{"ko": "로케일 감지 및 번역", "en": "Locale detection and translation"},
			Mandatory: true, SupportedLocales: []string{"ko", "en"},
		}
	case moduleID == "core.agent":
		m["agent.compose"] = &Capability{
			ID: "agent.compose", Type: CapabilitySystemPrompt, Priority: 150,
			Description: LocalizedString{"ko": "시스템 프롬프트 구성", "en": "System prompt composition"},
			Mandatory: true, SupportedLocales: []string{"ko", "en"},
		}
	case moduleID == "core.config":
		m["config.resolve"] = &Capability{
			ID: "config.resolve", Type: CapabilityReplacementSlot, Priority: 300,
			Description: LocalizedString{"ko": "설정 로드 및 검증", "en": "Config loading and validation"},
			Mandatory: true, SupportedLocales: []string{"ko", "en"},
		}
	case moduleID == "core.extension":
		m["ext.intercept_provider"] = &Capability{
			ID: "ext.intercept_provider", Type: CapabilityProvider, Priority: 50,
			Description: LocalizedString{"ko": "provider 요청 인터셉트", "en": "Provider request interception"},
			Mandatory: true, SupportedLocales: []string{"ko", "en"},
		}
		m["ext.intercept_tool"] = &Capability{
			ID: "ext.intercept_tool", Type: CapabilityTool, Priority: 50,
			Description: LocalizedString{"ko": "도구 훅", "en": "Tool hooks"},
			Mandatory: true, SupportedLocales: []string{"ko", "en"},
		}
	case moduleID == "feature.autoresearch":
		m["autoresearch.sweep"] = &Capability{
			ID: "autoresearch.sweep", Type: CapabilityTool, Priority: 40,
			Description: LocalizedString{"ko": "자동 연구 스위프", "en": "Auto-research sweep"},
			Mandatory: false, SupportedLocales: []string{"ko", "en"},
		}
	}

	return m
}