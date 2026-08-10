// Package assets embeds the compiled-in product artwork for the patty harness.
// The banner and launch artwork are resolved from the product profile and
// served to the TUI launch renderer at startup.
package assets

import (
	"embed"
	"fmt"
	"strings"
	"sync"
)

//go:embed banner.txt logo.svg
var embedded embed.FS

// Lookup returns the named asset's bytes. Names are the profile-relative paths
// (e.g. "assets/banner.txt") as declared in product.yaml.
func Lookup(name string) ([]byte, error) {
	name = strings.TrimPrefix(name, "/")
	clean := strings.TrimPrefix(name, "assets/")
	if !strings.HasPrefix(clean, "banner.") && !strings.HasPrefix(clean, "logo.") {
		return nil, fmt.Errorf("assets: unknown artwork %q", name)
	}
	raw, err := embedded.ReadFile(clean)
	if err != nil {
		return nil, fmt.Errorf("assets: read %q: %w", name, err)
	}
	return raw, nil
}

var (
	bannerOnce sync.Once
	bannerText string
)

// Banner returns the 한지 작업대 launch banner text. The embedded asset is
// read and normalized once per process; callers (e.g. the TUI banner renderer)
// may invoke it repeatedly without re-reading the embed FS.
func Banner() string {
	bannerOnce.Do(func() {
		raw, err := Lookup("assets/banner.txt")
		if err != nil {
			return
		}
		bannerText = strings.TrimRight(string(raw), "\n")
	})
	return bannerText
}
