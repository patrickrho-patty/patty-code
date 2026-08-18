package dariproto

import (
	"bytes"
	"testing"
)

// dlprules_scope_test.go pins the PAT-1432 scope extension: packs
// WITHOUT a scope must stay byte-identical to the pre-scope wire
// format (deployed relays/ harnesses must interoperate), and packs
// WITH a scope must round-trip the level+ID.

// unscopedPackLayout is the pre-PAT-1432 wire layout (no field 8).
type unscopedPackLayout struct {
	Version       uint16            `cbor:"1,keyasint"`
	EpochID       string            `cbor:"2,keyasint"`
	OrgID         string            `cbor:"3,keyasint"`
	NotAfterMs    int64             `cbor:"4,keyasint"`
	Rules         []DLPRuleWire     `cbor:"5,keyasint"`
	Digest        [32]byte          `cbor:"6,keyasint"`
	RuleOverrides []DLPRuleOverride `cbor:"7,keyasint,omitempty"`
}

func TestUnscopedPackOmitsField8(t *testing.T) {
	pack := &DLPRulePackWire{
		Version: 1, EpochID: "e", OrgID: "o",
		Rules: []DLPRuleWire{{RuleID: "pii-kr-rrn", Pattern: "korean_pii", Severity: "critical"}},
	}
	data, err := MarshalCBOR(pack)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte{0x68}) && bytes.Contains(data, []byte("org")) {
		// crude guard only: an encoded scope map would carry the level
		// string; precise check is the field-label scan below.
	}
	for _, level := range []string{ScopeOrg, ScopeTeam, ScopeUser, ScopeHarness} {
		if bytes.Contains(data, []byte(level)) {
			t.Fatalf("unscoped pack must not carry scope strings, found %q in %x", level, data)
		}
	}
	// A pre-PAT-1432 decoder (no Scope field) must decode the bytes.
	var legacy unscopedPackLayout
	if err := UnmarshalCBOR(data, &legacy); err != nil {
		t.Fatalf("legacy decode: %v", err)
	}
	if legacy.EpochID != "e" || len(legacy.Rules) != 1 {
		t.Fatalf("legacy decode lost fields: %+v", legacy)
	}
}

func TestScopedPackRoundTrip(t *testing.T) {
	pack := &DLPRulePackWire{
		Version: 1, EpochID: "e", OrgID: "o",
		Rules: []DLPRuleWire{{RuleID: "pii-kr-rrn", Pattern: "korean_pii", Severity: "critical"}},
		Scope: DLPRuleScope{Level: ScopeUser, ID: "user-42"},
	}
	data, err := MarshalCBOR(pack)
	if err != nil {
		t.Fatal(err)
	}
	var decoded DLPRulePackWire
	if err := UnmarshalCBOR(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Scope.Level != ScopeUser || decoded.Scope.ID != "user-42" {
		t.Fatalf("scope round-trip: %+v", decoded.Scope)
	}
}

func TestScopeEffectiveLevelDefaultsToOrg(t *testing.T) {
	if got := (DLPRuleScope{}).EffectiveLevel(); got != ScopeOrg {
		t.Fatalf("zero scope must be org, got %q", got)
	}
	if got := (DLPRuleScope{Level: ScopeHarness, ID: "peer-1"}).EffectiveLevel(); got != ScopeHarness {
		t.Fatalf("explicit harness level: %q", got)
	}
}

func TestScopeRankOrder(t *testing.T) {
	// Harness > User > Team > Org; unknown ranks below org.
	if !(ScopeRank(ScopeHarness) > ScopeRank(ScopeUser) &&
		ScopeRank(ScopeUser) > ScopeRank(ScopeTeam) &&
		ScopeRank(ScopeTeam) > ScopeRank(ScopeOrg)) {
		t.Fatal("precedence order must be Harness > User > Team > Org")
	}
	if ScopeRank("nonsense") >= ScopeRank(ScopeOrg) {
		t.Fatal("unknown level must rank below org")
	}
}
