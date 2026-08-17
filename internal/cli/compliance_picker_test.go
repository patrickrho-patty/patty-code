package cli

import (
	"testing"

	"patty/internal/i18n"
)

func TestCompliancePickerOpensAndInitializesItems(t *testing.T) {
	previousLanguage := i18n.CurrentLanguage()
	defer i18n.DetectLanguage(previousLanguage)
	i18n.DetectLanguage("ko")

	m := newTestChatTUI()
	m.openCompliancePicker()

	p := m.quickPick
	if p == nil {
		t.Fatal("compliance picker did not open")
	}
	if p.kind != quickPickerCompliance {
		t.Fatalf("picker kind = %q, want compliance", p.kind)
	}
	if len(p.items) != 4 {
		t.Fatalf("picker items count = %d, want 4", len(p.items))
	}
	wantIDs := []string{"all", "pipa", "kisa", "csap"}
	for i, want := range wantIDs {
		if p.items[i].ID != want {
			t.Errorf("item %d ID = %q, want %q", i, p.items[i].ID, want)
		}
	}
}
