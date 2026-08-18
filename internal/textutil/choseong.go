package textutil

import "strings"

// Hangul syllable decomposition constants (Unicode Hangul Syllables block).
const (
	hangulSBase  = 0xAC00 // 가
	hangulSLast  = 0xD7A3 // 힣
	hangulVCount = 21
	hangulTCount = 28
	hangulNCount = hangulVCount * hangulTCount // 588 syllables per leading consonant
)

// hangulLeading is the canonical order of leading consonants (초성), indexed
// by (syllable - hangulSBase) / hangulNCount.
const hangulLeading = "ㄱㄲㄴㄷㄸㄹㅁㅂㅃㅅㅆㅇㅈㅉㅊㅋㅌㅍㅎ"

var hangulLeadingRunes = []rune(hangulLeading)

// HangulLeadingJamo returns the leading consonant (초성) of a precomposed
// Hangul syllable, or 0 when r is not a syllable.
func HangulLeadingJamo(r rune) rune {
	if r < hangulSBase || r > hangulSLast {
		return 0
	}
	return hangulLeadingRunes[(r-hangulSBase)/hangulNCount]
}

// ChoseongOf maps a Hangul syllable string to its leading-jamo (초성) spelling,
// e.g. "모델변경" → "ㅁㄷㅂㄱ". Non-Hangul runes pass through unchanged so a "/"
// prefix and any Latin text survive.
func ChoseongOf(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if l := HangulLeadingJamo(r); l != 0 {
			b.WriteRune(l)
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// HasJamo reports whether s contains any Hangul initial consonant (U+3131–U+314E,
// ㄱ..ㅎ). Queries without jamo always take the literal matcher, so English-only
// users never pay for chosung matching.
func HasJamo(s string) bool {
	for _, r := range s {
		if r >= 'ㄱ' && r <= 'ㅎ' {
			return true
		}
	}
	return false
}

// ChoseongMatch reports whether query matches candidate's 초성 spelling: an
// empty query matches all; a jamo-free query never matches (callers keep their
// literal matchers); otherwise each jamo run of query must appear in order as a
// subsequence of ChoseongOf(candidate) and each non-jamo run as a subsequence
// of the case-folded candidate.
func ChoseongMatch(candidate, query string) bool {
	if query == "" {
		return true
	}
	q := strings.ToLower(query)
	if !HasJamo(q) {
		return false
	}
	c := strings.ToLower(candidate)
	proj := []rune(ChoseongOf(c))
	raw := []rune(c)
	qr := []rune(q)
	// proj and raw are index-parallel rune arrays (ChoseongOf is 1:1 per rune),
	// so one shared cursor enforces global order across jamo and non-jamo runs.
	i := 0
	for qi := 0; qi < len(qr); {
		if isJamoRune(qr[qi]) {
			j := qi
			for j < len(qr) && isJamoRune(qr[j]) {
				j++
			}
			next, ok := subseqAfter(proj, i, qr[qi:j])
			if !ok {
				return false
			}
			i, qi = next, j
		} else {
			j := qi
			for j < len(qr) && !isJamoRune(qr[j]) {
				j++
			}
			next, ok := subseqAfter(raw, i, qr[qi:j])
			if !ok {
				return false
			}
			i, qi = next, j
		}
	}
	return true
}

func isJamoRune(r rune) bool { return r >= 'ㄱ' && r <= 'ㅎ' }

// subseqAfter returns the index just after the first occurrence of seg as a
// subsequence of s starting at from, or (0, false) when it does not occur.
func subseqAfter(s []rune, from int, seg []rune) (int, bool) {
	ti := from
	for ti < len(s) {
		if s[ti] == seg[0] {
			seg = seg[1:]
			if len(seg) == 0 {
				return ti + 1, true
			}
		}
		ti++
	}
	return 0, false
}
