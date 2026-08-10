package sftpfs

import (
	"strings"
	"testing"
)

func TestDetectKind(t *testing.T) {
	cases := []struct {
		name string
		in   []byte
		want Kind
	}{
		{"ascii", []byte("hello world\n"), KindText},
		{"utf8", []byte("héllo 세계"), KindText},
		{"empty", []byte{}, KindText},
		{"nul", []byte("abc\x00def"), KindBinary},
		{"invalid-utf8", []byte{0xff, 0xfe, 0xfd, 0xfc}, KindBinary},
		{"long-text", []byte(strings.Repeat("a", 20000)), KindText},
	}
	for _, c := range cases {
		if got := DetectKind(c.in); got != c.want {
			t.Errorf("DetectKind(%s) = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestDetectKindTruncatedRune(t *testing.T) {
	body := append([]byte(strings.Repeat("a", sniffLen-1)), []byte("세")[0])
	if got := DetectKind(body); got != KindText {
		t.Errorf("truncated trailing rune classified as %v, want text", got)
	}
}
