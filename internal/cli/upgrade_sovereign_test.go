//go:build profile_sovereign

package cli

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"patty/internal/sovereign"
)

func TestUpdateImportVerifiesSignedAdvisory(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	adv := sovereign.UpdateAdvisory{
		AdvisoryID: "adv-1",
		Version:    "1.2.3",
		Payload:    []byte("bundle-bytes"),
		IssuedAt:   1,
		NotAfter:   0, // no expiry
	}
	adv.Signature = ed25519.Sign(priv, adv.SigningBytes())
	dir := t.TempDir()
	advPath := filepath.Join(dir, "advisory.json")
	raw, _ := json.Marshal(adv)
	if err := os.WriteFile(advPath, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	keyPath := filepath.Join(dir, "pub.hex")
	if err := os.WriteFile(keyPath, []byte(hex.EncodeToString(pub)), 0o600); err != nil {
		t.Fatal(err)
	}
	if code := upgradeCommand([]string{"import", advPath, "--key", keyPath}, "1.0.0"); code != 0 {
		t.Fatalf("valid advisory import exited %d, want 0", code)
	}
	tampered := adv
	tampered.Signature = []byte("bad")
	raw2, _ := json.Marshal(tampered)
	badPath := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(badPath, raw2, 0o600); err != nil {
		t.Fatal(err)
	}
	if code := upgradeCommand([]string{"import", badPath, "--key", keyPath}, "1.0.0"); code == 0 {
		t.Fatal("tampered advisory must exit non-zero")
	}
}

func TestSovereignUpgradeRefusesOnline(t *testing.T) {
	if code := upgradeCommand([]string{"--check"}, "1.0.0"); code == 0 {
		t.Fatal("online upgrade must fail closed in sovereign builds")
	}
}

func TestParseUpdateImportArgsMissingFlag(t *testing.T) {
	if _, _, err := parseUpdateImportArgs([]string{"advisory.json"}); err == nil {
		t.Fatal("missing --key flag must surface an error")
	}
}

func TestParseUpdateImportArgsParseError(t *testing.T) {
	if _, _, err := parseUpdateImportArgs([]string{"--unknown"}); err == nil {
		t.Fatal("unknown flag must surface an error")
	}
}

func TestReadUpdateAdvisoryMissingFile(t *testing.T) {
	if _, err := readUpdateAdvisory(filepath.Join(t.TempDir(), "nope.json")); err == nil {
		t.Fatal("missing file must surface an error")
	}
}

func TestReadUpdateAdvisoryBadJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(path, []byte("{not-json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readUpdateAdvisory(path); err == nil {
		t.Fatal("malformed JSON must surface an error")
	}
}

func TestLoadUpdatePublicKeyBadHex(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.hex")
	if err := os.WriteFile(path, []byte("zz-not-hex"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadUpdatePublicKey(path); err == nil {
		t.Fatal("non-hex --key must surface an error")
	}
}

func TestLoadUpdatePublicKeyWrongLength(t *testing.T) {
	path := filepath.Join(t.TempDir(), "short.hex")
	if err := os.WriteFile(path, []byte(hex.EncodeToString([]byte{1, 2, 3})), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadUpdatePublicKey(path); err == nil {
		t.Fatal("non-32-byte --key must surface an error")
	}
}

func TestLoadUpdatePublicKeyStripsUnicodeWhitespace(t *testing.T) {
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "key.hex")
	if err := os.WriteFile(path, []byte(strings.Join([]string{
		hex.EncodeToString(pub[:16]),
		"\u00a0", // NBSP copy-pasted from a rendered HTML page
		hex.EncodeToString(pub[16:]),
	}, "")), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := loadUpdatePublicKey(path)
	if err != nil {
		t.Fatalf("loadUpdatePublicKey: %v", err)
	}
	if !equalBytes(got, pub) {
		t.Fatalf("loaded key differs from pub after whitespace strip")
	}
}

func TestRunUpdateImportExpiredAdvisory(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	adv := sovereign.UpdateAdvisory{
		AdvisoryID: "adv-1",
		Version:    "1.2.3",
		Payload:    []byte("bundle-bytes"),
		IssuedAt:   1,
		NotAfter:   100,
	}
	adv.Signature = ed25519.Sign(priv, adv.SigningBytes())
	dir := t.TempDir()
	advPath := filepath.Join(dir, "advisory.json")
	if err := os.WriteFile(advPath, mustMarshal(adv), 0o600); err != nil {
		t.Fatal(err)
	}
	keyPath := filepath.Join(dir, "pub.hex")
	if err := os.WriteFile(keyPath, []byte(hex.EncodeToString(pub)), 0o600); err != nil {
		t.Fatal(err)
	}
	defer setNowUnixMilli(t, func() int64 { return 200 })()
	if code := upgradeCommand([]string{"import", advPath, "--key", keyPath}, "1.0.0"); code == 0 {
		t.Fatal("expired advisory must exit non-zero")
	}
}

func mustMarshal(v any) []byte {
	raw, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return raw
}

func equalBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// setNowUnixMilli replaces nowUnixMilli for the lifetime of a test and
// restores it on cleanup.
// setNowUnixMilli replaces nowUnixMilli for the lifetime of a test and
// returns a restore function the test should defer. It is undefined
// outside the test build because no production code path is allowed to
// drift the wall clock — the swap exists purely to verify IsExpired
// behaviour deterministically.
func setNowUnixMilli(t testing.TB, fn func() int64) func() {
	t.Helper()
	prev := nowUnixMilli
	nowUnixMilli = fn
	return func() { nowUnixMilli = prev }
}
