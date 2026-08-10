package profile_test

import (
	"testing"

	"patty/internal/profile"
)

// TestGongCodeResolvesFromPattyBase verifies the derived gongcode harness
// resolves from the patty base: it inherits the base, overrides identity, and
// appends the governance-enforcement modules as harness-required.
func TestGongCodeResolvesFromPattyBase(t *testing.T) {
	base, err := profile.Load("../../products/patty/product.yaml")
	if err != nil {
		t.Fatalf("load patty base: %v", err)
	}
	derived, err := profile.Load("../../products/gongcode/product.yaml")
	if err != nil {
		t.Fatalf("load gongcode derived: %v", err)
	}

	res, err := profile.ResolveDerived(base, derived)
	if err != nil {
		t.Fatalf("resolve derived: %v", err)
	}
	p := res.Resolved
	if p.HarnessID != "gongcode" {
		t.Errorf("HarnessID = %q, want gongcode", p.HarnessID)
	}
	if p.ExecutableName != "gongcode" {
		t.Errorf("ExecutableName = %q, want gongcode", p.ExecutableName)
	}
	if p.UserRoot != ".gongcode" {
		t.Errorf("UserRoot = %q, want .gongcode", p.UserRoot)
	}
	if p.EnvPrefix != "GONGCODE_" {
		t.Errorf("EnvPrefix = %q, want GONGCODE_", p.EnvPrefix)
	}
	if p.DisplayName["ko"] != "공코드" {
		t.Errorf("ko displayName = %q, want 공코드", p.DisplayName["ko"])
	}

	// The base five required modules must be inherited, plus the five
	// governance-enforcement modules appended by the derived profile.
	want := map[string]bool{
		"core.tui": true, "core.i18n": true, "core.agent": true,
		"core.config": true, "core.extension": true,
		"core.identity": true, "core.transport": true, "core.policy": true,
		"core.scanner": true, "core.audit": true,
	}
	got := map[string]bool{}
	for _, m := range p.RequiredModules {
		got[m.ID] = true
	}
	for id := range want {
		if !got[id] {
			t.Errorf("resolved gongcode profile missing required module %q", id)
		}
	}
	// The enforcement modules must all be signed per the derived profile.
	for _, m := range p.RequiredModules {
		if id := m.ID; id == "core.identity" || id == "core.transport" || id == "core.policy" || id == "core.scanner" || id == "core.audit" {
			if !m.Signed {
				t.Errorf("enforcement module %q should require signing", id)
			}
		}
	}
}

// TestGongCodeEnforcementFailClosed verifies that the gongcode governance
// modules are registered as enforced (non-disableable) and that integrity
// reports them present, matching the G9 gate requirement that a mandatory
// module cannot be toggled off through any API.
func TestGongCodeEnforcementFailClosed(t *testing.T) {
	reg := profile.NewRegistry()
	enforced := []string{"core.identity", "core.transport", "core.policy", "core.scanner", "core.audit"}
	for _, id := range enforced {
		if err := reg.Register(&profile.Module{ID: id, Version: "1.0.0", Enabled: true}, true); err != nil {
			t.Fatal(err)
		}
	}
	// All enforced modules are present and the registry is ready.
	res := reg.CheckIntegrity(nil)
	if !res.AllEnforcedPresent {
		t.Fatalf("expected all enforced modules present, missing=%v", res.MissingModules)
	}
	if res.Readiness != "ready" {
		t.Errorf("Readiness = %q, want ready", res.Readiness)
	}
	// None of the enforced modules can be disabled or removed.
	for _, id := range enforced {
		if err := reg.Disable(id); err == nil {
			t.Errorf("expected enforced module %q disable to fail", id)
		}
		if err := reg.Remove(id); err == nil {
			t.Errorf("expected enforced module %q remove to fail", id)
		}
	}
}

// TestGongCodeStorageNamespaceIsolated verifies the gongcode harness uses its
// own userRoot/storage namespace rather than the patty one (cross-harness state
// isolation from the profile layer).
func TestGongCodeStorageNamespaceIsolated(t *testing.T) {
	base, err := profile.Load("../../products/patty/product.yaml")
	if err != nil {
		t.Fatal(err)
	}
	derived, err := profile.Load("../../products/gongcode/product.yaml")
	if err != nil {
		t.Fatal(err)
	}
	res, err := profile.ResolveDerived(base, derived)
	if err != nil {
		t.Fatal(err)
	}
	p := res.Resolved
	if p.StorageNamespace != "gongcode" {
		t.Errorf("StorageNamespace = %q, want gongcode (isolated from patty)", p.StorageNamespace)
	}
	if p.UserRoot == base.UserRoot {
		t.Errorf("gongcode UserRoot %q should differ from patty %q", p.UserRoot, base.UserRoot)
	}
}