// Package profile — module lifecycle management.
package profile

import (
	"fmt"
	"sync"
)

// Module represents an installable, configurable unit of harness functionality.
type Module struct {
	ID          string
	Version     string
	Signed      bool
	Description LocalizedString
	Enabled     bool
	DependsOn   []string
	Provides    []string // capability IDs this module implements
	DataPath    string   // persistent data storage path
}

// Registry manages the installation, enabling, and removal of modules.
type Registry struct {
	mu       sync.RWMutex
	modules  map[string]*Module // id → module
	enforced map[string]bool    // ids that cannot be disabled
	policies map[string]Policy  // profile policies for each module
}

// Policy defines what a product profile mandates for a module.
type Policy int

const (
	PolicyOptional   Policy = iota // user can enable/disable/remove
	PolicyRequired                 // must stay enabled, cannot disable
	PolicyProhibited               // must never load
)

// NewRegistry creates an empty module registry.
func NewRegistry() *Registry {
	return &Registry{
		modules:  make(map[string]*Module),
		enforced: make(map[string]bool),
		policies: make(map[string]Policy),
	}
}

// Register adds a module to the registry. If enforced is true, the module
// cannot be disabled through any runtime API.
func (r *Registry) Register(m *Module, enforced bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if m.ID == "" {
		return fmt.Errorf("module ID required")
	}
	if _, exists := r.modules[m.ID]; exists {
		return fmt.Errorf("module %q already registered", m.ID)
	}

	r.modules[m.ID] = m
	if enforced {
		r.enforced[m.ID] = true
		r.policies[m.ID] = PolicyRequired
	} else {
		r.policies[m.ID] = PolicyOptional
	}
	return nil
}

// Install prepares a module for use by verifying dependencies.
func (r *Registry) Install(id string, version string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	m, exists := r.modules[id]
	if !exists {
		return fmt.Errorf("unknown module %q", id)
	}

	if version != "" && version != m.Version {
		return fmt.Errorf("module %q version mismatch: expected %q, got %q", id, m.Version, version)
	}

	for _, dep := range m.DependsOn {
		depMod, ok := r.modules[dep]
		if !ok {
			return fmt.Errorf("missing dependency %q for module %q", dep, id)
		}
		if !depMod.Enabled {
			return fmt.Errorf("dependency %q not enabled for module %q", dep, id)
		}
	}

	// Set data path if not already set
	if m.DataPath == "" {
		m.DataPath = fmt.Sprintf(".data/%s", id)
	}

	return nil
}

// Enable marks a module as active. Returns false if the module is enforced
// (already always enabled) or absent.
func (r *Registry) Enable(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	m, ok := r.modules[id]
	if !ok {
		return fmt.Errorf("unknown module %q", id)
	}

	if r.enforced[id] {
		return fmt.Errorf("module %q is harness-enforced and cannot be toggled", id)
	}

	m.Enabled = true
	return nil
}

// Disable attempts to disable a module. Enforced modules always reject this.
// Dependent modules are checked before disabling.
func (r *Registry) Disable(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	m, ok := r.modules[id]
	if !ok {
		return fmt.Errorf("unknown module %q", id)
	}

	if r.enforced[id] {
		return fmt.Errorf("module %q is harness-enforced and cannot be disabled", id)
	}

	// Check reverse dependencies — no other enabled module depends on this
	for otherID, other := range r.modules {
		if otherID == id || !other.Enabled {
			continue
		}
		for _, dep := range other.DependsOn {
			if dep == id {
				return fmt.Errorf("cannot disable %q: enabled module %q depends on it", id, otherID)
			}
		}
	}

	m.Enabled = false
	return nil
}

// Remove unregisters a module. Data is preserved by default; callers may
// call CleanupData() separately for destructive purges.
func (r *Registry) Remove(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	_, ok := r.modules[id]
	if !ok {
		return fmt.Errorf("unknown module %q", id)
	}

	if r.enforced[id] {
		return fmt.Errorf("harness-enforced module %q cannot be removed", id)
	}

	// Same dependency check as Disable
	for otherID, other := range r.modules {
		if otherID == id || !other.Enabled {
			continue
		}
		for _, dep := range other.DependsOn {
			if dep == id {
				return fmt.Errorf("cannot remove %q: enabled module %q depends on it", id, otherID)
			}
		}
	}

	delete(r.modules, id)
	delete(r.policies, id)
	delete(r.enforced, id)
	return nil
}

// IsEnforced returns whether the given module is harness-enforced.
func (r *Registry) IsEnforced(id string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.enforced[id]
}

// Get returns the module with the given ID.
func (r *Registry) Get(id string) (*Module, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	m, ok := r.modules[id]
	if !ok {
		return nil, false
	}
	cp := *m
	cp.Provides = cloneStrSlice(m.Provides)
	cp.DependsOn = cloneStrSlice(m.DependsOn)
	return &cp, true
}

// EnabledModules returns all currently enabled module IDs.
func (r *Registry) EnabledModules() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []string
	for id, m := range r.modules {
		if m.Enabled {
			result = append(result, id)
		}
	}
	return result
}

// IntegrityResult reports the health of enforced modules for attestation.
type IntegrityResult struct {
	AllEnforcedPresent bool
	MissingModules     []string
	ModifiedDigests    []string
	Readiness          string
}

// CheckIntegrity verifies all enforced modules are present and healthy.
func (r *Registry) CheckIntegrity(verifier Verifier) IntegrityResult {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var missing, modified []string

	for modID := range r.enforced {
		if _, ok := r.modules[modID]; !ok {
			missing = append(missing, modID)
			continue
		}
		if verifier != nil {
			if valid, digest := verifier.Verify(modID); !valid {
				modified = append(modified, digest)
			}
		}
	}

	result := IntegrityResult{
		AllEnforcedPresent: len(missing) == 0,
		MissingModules:     missing,
		ModifiedDigests:    modified,
	}

	if len(missing) > 0 {
		result.Readiness = "blocked_missing_modules"
	} else if len(modified) > 0 {
		result.Readiness = "tampered_module_detected"
	} else {
		result.Readiness = "ready"
	}

	return result
}

// CleanupData removes the data directory for the given module. This is
// destructive and only called explicitly during uninstall with user consent.
func (r *Registry) CleanupData(moduleID string) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.modules[moduleID]
	if !ok {
		return fmt.Errorf("unknown module %q", moduleID)
	}
	if r.enforced[moduleID] {
		return fmt.Errorf("enforced module %q data cleanup requires administrative action", moduleID)
	}
	return nil
}

// cloneStrSlice returns a copy of s without allocating if s is nil or empty.
func cloneStrSlice(s []string) []string {
	if s == nil {
		return nil
	}
	cp := make([]string, len(s))
	copy(cp, s)
	return cp
}

// Verifier checks the integrity of a signed module.
type Verifier interface {
	Verify(moduleID string) (bool, string) // (valid, computed_digest_hex)
}
