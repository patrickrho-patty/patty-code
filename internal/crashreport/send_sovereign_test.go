//go:build profile_sovereign

package crashreport

import (
	"context"
	"testing"

	"patty/internal/netclient"
)

func TestSovereignCrashSendFailsClosed(t *testing.T) {
	err := Send(context.Background(), Report{}, netclient.ProxySpec{})
	if err != ErrSendUnavailable {
		t.Fatalf("Send = %v, want ErrSendUnavailable", err)
	}
}
