package cli

import (
	"os"
	"testing"
)

func TestFileItemsChosungListing(t *testing.T) {
	dir := t.TempDir()
	writeAt(t, dir, "한국어문서.md", "x")
	writeAt(t, dir, "alpha.go", "y")

	m := newTestChatTUI()

	// jamo query matches via 초성 (proj: ㅎㄱㅇㅁㅅ.md)
	items := m.fileItems(dir + "/ㅎㄱ")
	if !hasLabel(items, "한국어문서.md") {
		t.Fatalf("@ㅎㄱ should offer 한국어문서.md: %v", labels(items))
	}
	if hasLabel(items, "alpha.go") {
		t.Fatalf("@ㅎㄱ must not offer alpha.go: %v", labels(items))
	}

	// jamo query with runes absent from the projection matches nothing
	if got := m.fileItems(dir + "/ㄷㄷ"); len(got) != 0 {
		t.Fatalf("@ㄷㄷ should match nothing: %v", labels(got))
	}

	// literal prefix behavior unchanged
	if got := m.fileItems(dir + "/한"); !hasLabel(got, "한국어문서.md") {
		t.Fatalf("@한 (literal prefix) should still offer 한국어문서.md: %v", labels(got))
	}
	if got := m.fileItems(dir + "/al"); !hasLabel(got, "alpha.go") {
		t.Fatalf("@al should still offer alpha.go: %v", labels(got))
	}
}

// TestFileItemsChosungSearchWalk verifies the top-level @-search walker finds
// a Korean-named file in a subdirectory via 초성 (the fileref.Search path).
func TestFileItemsChosungSearchWalk(t *testing.T) {
	orig, _ := os.Getwd()
	defer os.Chdir(orig)

	dir := t.TempDir()
	writeAt(t, dir, "하위/한국어문서.md", "x")
	writeAt(t, dir, "src/main.go", "y")
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	m := newTestChatTUI()
	items := m.fileItems("ㅎㄱ")

	if !hasLabel(items, "하위/한국어문서.md") {
		t.Fatalf("top-level @ㅎㄱ should offer 하위/한국어문서.md, got %v", labels(items))
	}
	if hasLabel(items, "src/main.go") {
		t.Fatalf("@ㅎㄱ must not offer src/main.go: %v", labels(items))
	}
}
