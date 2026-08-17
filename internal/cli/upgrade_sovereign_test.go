//go:build profile_sovereign

package cli

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
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
