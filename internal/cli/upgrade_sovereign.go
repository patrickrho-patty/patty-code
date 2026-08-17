//go:build profile_sovereign

// Sovereign update path (ADR G3): no vendor/GitHub fetch is compiled into
// the binary. The signed offline advisory is the primary channel; the
// operator UX is `patcode update import <advisory-file> --key <pubkey>`.
package cli

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/pflag"

	"patty/internal/sovereign"
)

func upgradeCommand(args []string, version string) int {
	_ = version
	if len(args) > 0 && args[0] == "import" {
		return runUpdateImport(args[1:])
	}
	if len(args) > 0 && (args[0] == "-h" || args[0] == "--help" || args[0] == "help") {
		fmt.Fprint(os.Stdout, sovereignUpgradeHelp)
		return 0
	}
	fmt.Fprintln(os.Stderr, "upgrade: online update fetch is not available in this build profile — apply signed offline updates via `patcode update import <advisory-file> --key <pubkey>`")
	return 1
}

const sovereignUpgradeHelp = `Apply signed offline updates in this build profile.

Usage:
  patcode update import <advisory-file> --key <pubkey-file>

Online update fetch is not compiled into this build profile; the signed
offline advisory is the update channel.
`

// runUpdateImport verifies a signed offline UpdateAdvisory against the
// issuing source's ed25519 public key and prints its audit digest. Staging
// the payload for the installer is the follow-up installer work; this
// command is the verification gate.
func runUpdateImport(args []string) int {
	fs := pflag.NewFlagSet("update import", pflag.ContinueOnError)
	keyPath := fs.String("key", "", "path to a hex-encoded ed25519 public key of the issuing source")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	rest := fs.Args()
	if len(rest) != 1 || *keyPath == "" {
		fmt.Fprintln(os.Stderr, "usage: patcode update import <advisory-file> --key <pubkey-file>")
		return 1
	}
	raw, err := os.ReadFile(rest[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "update import: %v\n", err)
		return 1
	}
	var adv sovereign.UpdateAdvisory
	if err := json.Unmarshal(raw, &adv); err != nil {
		fmt.Fprintf(os.Stderr, "update import: malformed advisory: %v\n", err)
		return 1
	}
	keyHex, err := os.ReadFile(*keyPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "update import: %v\n", err)
		return 1
	}
	pub, err := hex.DecodeString(string(trimSpaceAll(string(keyHex))))
	if err != nil {
		fmt.Fprintf(os.Stderr, "update import: --key must be hex: %v\n", err)
		return 1
	}
	sigOK := sovereign.VerifyAdvisorySignature(pub, &adv)
	expired := adv.IsExpired(nowUnixMilli())
	fmt.Printf("advisory %s  version=%s\n", adv.AdvisoryID, adv.Version)
	fmt.Printf("digest  %x\n", adv.Digest())
	fmt.Printf("signature: %v  expired: %v\n", sigOK, expired)
	if !sigOK {
		fmt.Fprintln(os.Stderr, "update import: REJECTED — signature verification failed")
		return 1
	}
	if expired {
		fmt.Fprintln(os.Stderr, "update import: REJECTED — advisory expired")
		return 1
	}
	fmt.Println("update import: advisory verified OK")
	return 0
}

func trimSpaceAll(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] != ' ' && s[i] != '\n' && s[i] != '\r' && s[i] != '\t' {
			out = append(out, s[i])
		}
	}
	return string(out)
}

// nowUnixMilli is swappable for tests.
var nowUnixMilli = func() int64 { return time.Now().UnixMilli() }
