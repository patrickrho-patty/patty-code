package main

import "testing"

func TestTrayMenuLabelsFollowLocale(t *testing.T) {
	koLabels := trayMenuLabels("ko-KR")
	if koLabels.openTitle != "열기" || koLabels.quitTitle != "종료" {
		t.Fatalf("ko labels = %#v", koLabels)
	}

	en := trayMenuLabels("en")
	if en.openTitle != "Open" || en.quitTitle != "Quit" {
		t.Fatalf("en labels = %#v", en)
	}

	other := trayMenuLabels("fr")
	if other.openTitle != en.openTitle || other.quitTitle != en.quitTitle {
		t.Fatalf("unknown locale should fall back to English, got %#v", other)
	}
}
