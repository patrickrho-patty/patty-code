//go:build profile_sovereign

// Sovereign update path (ADR G3): no vendor/GitHub fetch is compiled into
// the binary. The signed offline advisory is the primary channel; the
// operator UX is `patcode update import <advisory-file> --key <pubkey>`.
package cli

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
	"unicode"

	"github.com/spf13/pflag"

	"patty/internal/sovereign"
	"patty/internal/tier"
)

const maxAdvisoryBytes = 1 << 20 // 1 MiB; signed advisories are KB-range.

// ErrUpgradeUnavailable is returned by upgradeCommand in sovereign builds
// (ADR G3): the online update fetch is compiled out; the offline advisory
// channel via `patcode update import` is the supported update path.
var ErrUpgradeUnavailable = errors.New("upgrade: online update fetch is not available in this build profile — apply signed offline updates via 'patcode update import <advisory-file> --key <pubkey>'")

func upgradeCommand(args []string, version string) int {
	tier.AssertDisallowed(tier.CapOnlineUpdate)
	_ = version
	if len(args) > 0 && args[0] == "import" {
		return runUpdateImport(args[1:])
	}
	if len(args) > 0 && (args[0] == "-h" || args[0] == "--help" || args[0] == "help") {
		fmt.Fprint(os.Stdout, sovereignUpgradeHelp)
		return 0
	}
	fmt.Fprintln(os.Stderr, ErrUpgradeUnavailable)
	return 1
}

const sovereignUpgradeHelp = `Usage of upgrade:

Apply signed offline updates in this build profile.

Usage:
  patcode update import <advisory-file> --key <pubkey-file>

Online update fetch is not compiled into this build profile; the signed
offline advisory is the update channel.
`

// runUpdateImport verifies a signed offline UpdateAdvisory against the
// issuing source's ed25519 public key and prints its audit digest. Staging
// the payload for the installer is the follow-up installer work; this
// command is the verification gate.
// errHelpRequested marks an explicit --help invocation: the usage text has
// already been printed to stdout, so the caller exits 0 without re-printing.
var errHelpRequested = errors.New("help requested")

func runUpdateImport(args []string) int {
	advisoryPath, keyPath, err := parseUpdateImportArgs(args)
	if err != nil {
		if errors.Is(err, errHelpRequested) {
			return 0
		}
		fmt.Fprintln(os.Stderr, "usage: patcode update import <advisory-file> --key <pubkey-file>")
		return 1
	}
	adv, err := readUpdateAdvisory(advisoryPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "update import:", err)
		return 1
	}
	if err := validateUpdateAdvisory(adv); err != nil {
		fmt.Fprintln(os.Stderr, "update import:", err)
		return 1
	}
	pub, err := loadUpdatePublicKey(keyPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "update import:", err)
		return 1
	}
	return reportImport(adv, sovereign.VerifyAdvisorySignature(pub, &adv), adv.IsExpired(nowUnixMilli()))
}

// parseUpdateImportArgs pulls the advisory path and --key flag out of
// `patcode update import`'s positional + flag args.
func parseUpdateImportArgs(args []string) (advisoryPath, keyPath string, err error) {
	fs := pflag.NewFlagSet("update import", pflag.ContinueOnError)
	keyFlag := fs.String("key", "", "path to a hex-encoded ed25519 public key of the issuing source")
	if err = fs.Parse(args); err != nil {
		if errors.Is(err, pflag.ErrHelp) {
			fmt.Fprintln(os.Stdout, "usage: patcode update import <advisory-file> --key <pubkey-file>")
			fs.SetOutput(os.Stdout)
			fs.PrintDefaults()
			return "", "", errHelpRequested
		}
		return "", "", err
	}
	rest := fs.Args()
	if len(rest) != 1 || *keyFlag == "" {
		return "", "", errors.New("missing advisory path or --key flag")
	}
	return rest[0], *keyFlag, nil
}

// readUpdateAdvisory opens the advisory file, enforces the 1 MiB size
// cap, and decodes its JSON.
func readUpdateAdvisory(path string) (sovereign.UpdateAdvisory, error) {
	f, err := os.Open(path)
	if err != nil {
		return sovereign.UpdateAdvisory{}, err
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, maxAdvisoryBytes+1))
	if err != nil {
		return sovereign.UpdateAdvisory{}, err
	}
	if len(raw) > maxAdvisoryBytes {
		return sovereign.UpdateAdvisory{}, fmt.Errorf("advisory too large (>%d bytes)", maxAdvisoryBytes)
	}
	var adv sovereign.UpdateAdvisory
	if err := json.Unmarshal(raw, &adv); err != nil {
		return sovereign.UpdateAdvisory{}, fmt.Errorf("malformed advisory: %w", err)
	}
	return adv, nil
}

// loadUpdatePublicKey reads the hex-encoded ed25519 public key from
// path, strips every Unicode whitespace rune (NBSP, line separator,
// etc.), hex-decodes it, and verifies the resulting length is exactly
// 32 bytes.
func loadUpdatePublicKey(path string) ([]byte, error) {
	keyHex, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	pub, err := hex.DecodeString(stripWhitespace(string(keyHex)))
	if err != nil {
		return nil, fmt.Errorf("--key must be hex: %w", err)
	}
	if len(pub) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("--key must decode to %d bytes (ed25519 public key), got %d", ed25519.PublicKeySize, len(pub))
	}
	return pub, nil
}

// validateUpdateAdvisory rejects structurally invalid advisories before
// any verification work: an empty AdvisoryID or Version would otherwise
// verify fine (the signature still binds) and print a blank audit line.
func validateUpdateAdvisory(adv sovereign.UpdateAdvisory) error {
	if strings.TrimSpace(adv.AdvisoryID) == "" {
		return errors.New("advisory has an empty advisory_id")
	}
	if strings.TrimSpace(adv.Version) == "" {
		return errors.New("advisory has an empty version")
	}
	return nil
}

// reportImport prints the advisory's audit digest and signature/expiry
// verdict, then returns the process exit code. IDs and versions are
// sanitized to printable runes: an advisory file is operator-supplied
// input, and raw ANSI/control sequences would inject into the terminal.
func reportImport(adv sovereign.UpdateAdvisory, sigOK, expired bool) int {
	fmt.Printf("advisory %s  version=%s\n", printable(adv.AdvisoryID), printable(adv.Version))
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

// printable maps control characters (including ANSI escape sequences)
// to '?' so untrusted strings cannot tamper with the operator's terminal.
func printable(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsPrint(r) {
			return r
		}
		return '?'
	}, s)
}

// stripWhitespace removes every Unicode-space rune (NBSP, line
// separator, etc.) so a key file containing copy-paste noise from a
// rendered HTML page does not produce a confusing "invalid hex" error
// down the line. ASCII-only trimming would silently accept U+00A0 and
// the like and then fail at hex.DecodeString.
func stripWhitespace(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, s)
}

// nowUnixMilli returns the current wall-clock time in milliseconds since
// the Unix epoch. It is a package-level variable so the expiry test can
// swap it directly from upgrade_sovereign_test.go (same package).
var nowUnixMilli = func() int64 { return time.Now().UnixMilli() }
