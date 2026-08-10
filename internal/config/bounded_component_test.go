package config

import (
	"strings"
	"testing"
)

func TestWorkspaceSlugShortPathUnchanged(t *testing.T) {
	setRuntimeGOOS(t, "linux")
	got := WorkspaceSlug("/Users/me/proj")
	if got != "-Users-me-proj" {
		t.Fatalf("WorkspaceSlug short path = %q, want -Users-me-proj", got)
	}
}

func TestWorkspaceSlugBoundsDeepPaths(t *testing.T) {
	deep := "/data/" + strings.Repeat("deeply-nested-workspace-segment/", 12) + "proj"
	slug := WorkspaceSlug(deep)
	if len(slug) > 255 {
		t.Fatalf("slug length = %d, exceeds 255-byte component limit", len(slug))
	}
	other := "/data/" + strings.Repeat("deeply-nested-workspace-segment/", 12) + "proj2"
	if WorkspaceSlug(other) == slug {
		t.Fatal("distinct deep paths share one slug")
	}
	if WorkspaceSlug(deep) != slug {
		t.Fatal("slug is not deterministic")
	}
}

func TestBoundFilenameComponentRuneBoundary(t *testing.T) {
	long := strings.Repeat("작업공간", 100)
	got := BoundFilenameComponent(long, 255)
	if len(got) > 255 {
		t.Fatalf("bounded component length = %d, want <= 255", len(got))
	}
	if strings.Contains(got, "�") {
		t.Fatalf("bounded component contains replacement char: %q", got)
	}
}
