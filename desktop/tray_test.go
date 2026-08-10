package main

import "testing"

func TestTrayMenuLabelsFollowLocale(t *testing.T) {
	zh := trayMenuLabels("ko-KR")
	if zh.openTitle != "열기" || zh.quitTitle != "종료" {
		t.Fatalf("zh labels = %#v", zh)
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
