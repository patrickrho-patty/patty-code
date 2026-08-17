//go:build profile_sovereign

package releaseasset

import (
	"context"
	"net/http"
	"testing"
)

func TestSovereignDownloadCLIFailsClosed(t *testing.T) {
	if _, err := DownloadCLI(context.Background(), http.DefaultClient, "1.0.0", "linux", "amd64"); err != ErrDownloadUnavailable {
		t.Fatalf("DownloadCLI err = %v, want ErrDownloadUnavailable", err)
	}
}
