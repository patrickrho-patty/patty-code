# Chosung Search Expansion (PAT-1398) Implementation Plan

> **REQUIRED SUB-SKILL:** Use the executing-plans skill to implement this plan task-by-task.

**Goal:** Extract Hangul 초성 (initial-consonant) decomposition into a shared `internal/textutil` package and thread one runtime matcher (`ChoseongMatch`) through every harness search/filter surface — slash palette, `@`-file references (including the workspace walker), MCP resources, generic select menus, quick-picker overlays, skill picker, arg completion, shell completion — with zero behavior change for non-Korean queries.

**Architecture:** One shared package owns the decomposition math (`HangulLeadingJamo`, `ChoseongOf`) and one additive matcher (`ChoseongMatch`, plus the `HasJamo` branch predicate). Every surface keeps its existing literal matcher untouched and simply ORs in `ChoseongMatch`; the matcher returns `false` fast for queries without Hangul jamo, so English-only users see identical behavior and no performance cost. The `select.go` filter functions are the hub for all generic menus — one edit covers sessions, language, providers, models, and every picker built on them.

**Tech Stack:** Go (harness CLI), standard library only. `golang.org/x/text/unicode/norm` already used in `internal/textutil` — no new dependencies.

---

## Canonical matcher spec (the hub — read this before every task)

New file `internal/textutil/choseong.go` defines exactly four exported symbols. All surfaces read from here; no surface may implement its own jamo logic.

```go
HangulLeadingJamo(r rune) rune       // leading consonant of a precomposed syllable; 0 if not a syllable
ChoseongOf(s string) string          // per-rune projection: syllables → leading jamo, everything else unchanged
HasJamo(s string) bool               // any rune in U+3131..U+314E (ㄱ..ㅎ)
ChoseongMatch(candidate, query) bool // "" → true; !HasJamo(query) → false; else segment match (below)
```

`ChoseongMatch` semantics:

1. Empty query → `true` (same "matches everything" contract as every existing filter).
2. Query has no Hangul jamo → `false` immediately (the issue's "비한국어 패스스루" — English users never pay for chosung matching, and existing literal matchers are untouched).
3. Otherwise, split the query at script boundaries into **jamo runs** and **non-jamo runs**. Both `candidate` and `query` are case-folded first. Each jamo run must appear as a subsequence of `ChoseongOf(candidate)`; each non-jamo run must appear as a subsequence of the case-folded candidate. Runs match in order within their own space. This single rule covers: pure 초성 (`ㅇㅊ`), slash-prefixed 초성 (`/ㅇㅊ`), mixed queries (`ㅋㅋlogin` → ㅋㅋ in the projection + login in the candidate), mixed with uppercase Latin (`ㅋㅋLOGIN`), and literal Korean (`압축` is a non-jamo run matching the raw candidate).

Behavior consequences (deliberate, documented):

- **D1 — Slash palette widens from prefix-only to subsequence for chosung.** Today `TestFuzzyFilterSlashChosungMatchesOnlyInitialPrefix` locks chosung to an initial prefix of a precomputed alias. With the shared matcher, `/ㅇㅊ` subsequence-matches any label whose 초성 contains ㅇㅊ in order. This is a deliberate consistency decision (one matcher everywhere, matching the issue's "모든 표면에 동일 필터 적용"); the test is rewritten to the new contract in Task 3. Single-jamo queries match broadly and the user narrows — exactly the issue's "모호한 자음" stance.
- **D2 — Double consonants stay distinct by construction.** The leading table is `ㄱㄲㄴㄷㄸㄹㅁㅂㅃㅅㅆㅇㅈㅉㅊㅋㅌㅍㅎ`; `ChoseongOf` emits the exact leading consonant, so a `ㄱ` query never matches a `ㄲ`-initial candidate. Add tests anyway (Task 1).
- **D3 — No widening of literal matching.** `ChoseongMatch` returns `false` for jamo-free queries, so OR-ing it into a prefix filter (files, arg completion) leaves English prefix behavior byte-identical.

---

## Task 0: Baseline & branch

**Files:** none

**Step 1: Record the baseline**

Run: `go build ./... && go test ./internal/... 2>&1 | tail -5`
Expected: build succeeds, tests pass. Note the pass count — every later task must not regress it.

**Step 2: Create the feature branch**

```bash
git checkout -b feat/chosung-search
```

**Step 3: Commit** (nothing to commit yet — this task just establishes the branch)

---

## Task 1: `internal/textutil/choseong.go` + tests (the foundation)

**Files:**
- Create: `internal/textutil/choseong.go`
- Create: `internal/textutil/choseong_test.go`

**Step 1: Write the failing test**

`internal/textutil/choseong_test.go` — table-driven, covering every edge case in the issue plus the invariants:

- `HangulLeadingJamo`: 가→ㄱ, 까→ㄲ, 한→ㅎ, 힣→ㅎ; non-syllable → 0 for ASCII (`'A'`), bare jamo (`'ㅎ'`, U+314E — outside the syllable block), and compat jamo (`'ᄀ'`, U+1100).
- `ChoseongOf`: "모델변경"→"ㅁㄷㅂㄱ", "/압축"→"/ㅇㅊ", "한/글"→"ㅎ/ㄱ" (path separator preserved), "README.md"→"README.md" (passthrough), ""→"".
- `HasJamo`: "ㅇㅊ"→true, "/ㅇㅊ"→true, "login"→false, "압축"→false (syllables are not jamo), ""→false.
- `ChoseongMatch`:
  - "" query → true
  - "login" vs "ㅋㅋlogin" → false (no-jamo query never matches — D3)
  - "ㅇㅊ" vs "압축" → true (full chosung)
  - "/ㅇㅊ" vs "/압축" → true (slash + chosung, the palette case)
  - "ㅇㅊ" vs "/압축 (compact)" → true (label with parens)
  - "ㅁㄷ" vs "모델변경" → true (partial prefix)
  - "ㄷㅂㄱ" vs "모델변경" → true (partial subsequence, not just prefix)
  - "ㅋㅋlogin" vs "ㅋㅋlogin" → true (mixed)
  - "ㅋㅋlogin" vs "login" → false (mixed query, missing jamo part)
  - "ㅋㅋLOGIN" vs "ㅋㅋlogin" → true (mixed + case folding)
  - "LOGIN" vs "ㅋㅋlogin" → false (D3: pure-Latin query never chosung-matches)
  - "ㄱ" vs "까치" → false (double consonant: 까→ㄲ, query ㄱ must not match — D2)
  - "ㄲ" vs "까치" → true
  - "ㄱ" vs "가나다" → true
  - "LOGIN" vs "ㅋㅋlogin" → true (case folding in mixed queries)
  - "ㅇㅊ" vs "apple" → false (no Hangul in candidate)

**Step 2: Run to verify it fails**

Run: `go test ./internal/textutil/ -run Choseong -v`
Expected: compile error / `undefined: ChoseongOf`.

**Step 3: Write the implementation**

`internal/textutil/choseong.go` (complete):

```go
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

// ChoseongMatch reports whether query matches candidate's 초성 spelling.
//
// An empty query matches everything (same contract as the existing filters). A
// query without Hangul jamo never matches — callers keep their literal matchers
// and English behavior is unchanged. Otherwise the query is split at script
// boundaries: each jamo run must appear as a subsequence of ChoseongOf(candidate)
// and each non-jamo run as a subsequence of the case-folded candidate, in order
// within each space. This covers pure 초성 ("ㅇㅊ"), slash-prefixed 초성 ("/ㅇㅊ")
// and mixed queries ("ㅋㅋlogin").
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
	pi, ri := 0, 0
	for qi := 0; qi < len(qr); {
		if isJamoRune(qr[qi]) {
			j := qi
			for j < len(qr) && isJamoRune(qr[j]) {
				j++
			}
			next, ok := subseqAfter(proj, pi, qr[qi:j])
			if !ok {
				return false
			}
			pi, qi = next, j
		} else {
			j := qi
			for j < len(qr) && !isJamoRune(qr[j]) {
				j++
			}
			next, ok := subseqAfter(raw, ri, qr[qi:j])
			if !ok {
				return false
			}
			ri, qi = next, j
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
```

**Step 4: Run to verify it passes**

Run: `go test ./internal/textutil/ -run Choseong -v`
Expected: all pass.

**Step 5: Commit**

```bash
git add internal/textutil/choseong.go internal/textutil/choseong_test.go
git commit -m "feat(textutil): chosung decomposition and matching (PAT-1398)"
```

---

## Task 2: Refactor existing inline implementations onto textutil

Behavior-neutral — the existing tests are the safety net. Two call sites today:

- `internal/cli/composer_selection.go:480-500` — local `hangulLeading`/`hangulLeadingRunes`/`hangulLeadingJamo`.
- `internal/cli/slash_registry.go:80-93` — local `chosungOf`.

**Files:**
- Modify: `internal/cli/composer_selection.go` (delete lines 486, 489-500; keep 480-485 and `composerHangulBackspaceReplacement`)
- Modify: `internal/cli/slash_registry.go` (delete `chosungOf`; `populateChosung` calls `textutil.ChoseongOf`)
- Test: `internal/cli/slash_registry_test.go` (existing — must stay green)

**Step 1: Refactor `composer_selection.go`**

- Add `"patty/internal/textutil"` to imports.
- Delete `hangulLeading` (line 486), `hangulLeadingRunes` (489-491), and `hangulLeadingJamo` (493-500). Keep `hangulSBase`, `hangulVCount`, `hangulTCount`, `hangulNCount` (the backspace decomposition math uses them); `hangulSLast` becomes unused — delete it too.
- `composerHangulBackspaceReplacement` (line 502): `leading := hangulLeadingJamo(r)` → `leading := textutil.HangulLeadingJamo(r)`.

**Step 2: Refactor `slash_registry.go`**

- Add `"patty/internal/textutil"` to imports.
- Delete `chosungOf` (80-93).
- `populateChosung` (74-78): `specs[i].chosung = chosungOf(specs[i].ko)` → `specs[i].chosung = textutil.ChoseongOf(specs[i].ko)`.

**Step 3: Verify**

Run: `go test ./internal/cli/ -run 'TestCanonicalSlashCommand|TestChosung|TestBuiltinSlash' && go build ./...`
Expected: pass (pure refactor — same output, same tests). Also run the composer tests: `go test ./internal/cli/ -run Composer`.

**Step 4: Commit**

```bash
git add internal/cli/composer_selection.go internal/cli/slash_registry.go
git commit -m "refactor(cli): use textutil for hangul leading-jamo (PAT-1398)"
```

---

## Task 3: `complete.go` — slash palette, `@`-references, MCP resources

**Files:**
- Modify: `internal/cli/complete.go` — `aliasMatches` (496-510), delete `hasHangulInitialJamo` (512-519), `fileItems` (615-665), `resourceItems` (735-756)
- Modify: `internal/fileref/search.go` — `Search` (77-170) match predicates, `pathSegmentContains` (176-183)
- Test: `internal/cli/complete_test.go` — rewrite `TestFuzzyFilterSlashChosungMatchesOnlyInitialPrefix` (738) to the D1 contract; add `@`-ref and resource chosung tests
- Test: `internal/fileref/search_test.go` — add chosung cases

**Step 1: Update `aliasMatches` (delete `hasHangulInitialJamo`)**

```go
// aliasMatches reports whether query matches any of an item's alternate
// spellings as a case-folded prefix or subsequence, or via 초성 matching.
func aliasMatches(it compItem, query string) bool {
	for _, a := range it.aliases {
		la := strings.ToLower(a)
		if strings.HasPrefix(la, query) {
			return true
		}
		if textutil.HasJamo(query) {
			if textutil.ChoseongMatch(a, query) {
				return true
			}
			continue // jamo queries match aliases only via chosung, not subsequence
		}
		if subsequenceMatch(la, query) {
			return true
		}
	}
	return false
}
```

Rationale: the precomputed `spec.chosung` alias (e.g. `/ㅇㅊ`) is the Korean-capable candidate — `ChoseongMatch` against it keeps chosung working in **both** locales (today's en-locale chosung relies on the alias too). `hasHangulInitialJamo` is deleted; `textutil.HasJamo` replaces it.

**Step 2: Rewrite the contract test**

Replace `TestFuzzyFilterSlashChosungMatchesOnlyInitialPrefix` with a test asserting the D1 contract. It runs in the default (en) locale — labels are `/compact` etc. and chosung matching works through the aliases in both locales:

```go
func TestFuzzyFilterSlashChosungSubsequence(t *testing.T) {
	m := newTestChatTUI()
	m.input.SetValue("/ㅇㅊ")
	m.updateCompletion()

	if !m.completion.active {
		t.Fatal("menu should open for Korean initial-consonant query /ㅇㅊ")
	}
	if !hasLabel(m.completion.items, "/compact") { // 압축 → ㅇㅊ
		t.Fatalf("/ㅇㅊ should match /compact: %v", labels(m.completion.items))
	}
	if hasLabel(m.completion.items, "/copy") { // 복사 → ㅂㅅ
		t.Fatalf("/ㅇㅊ must not match /copy: %v", labels(m.completion.items))
	}
}
```

**Step 3: Add chosung to `fileItems` (the `@`-reference surface)**

```go
	if !strings.HasPrefix(name, fsFrag) && !textutil.ChoseongMatch(name, fsFrag) {
		continue
	}
```

(`ChoseongMatch` returns `false` for jamo-free fragments, so English prefix behavior is byte-identical — D3. The path separator edge case is handled by construction: `splitPathToken` already splits dir/frag, and `ChoseongOf` never decomposes `/`.)

**Step 4: Make the top-level `@`-search walker chosung-aware (`fileref.Search`)**

The `@`-search results come from `fileref.Search` (`internal/fileref/search.go:77`), which matches **literal** substrings only (`strings.Contains(nameLower, query)` for basenames, `pathSegmentContains` for path segments, and the same for directories). It also enforces `minQueryLen = 2` (line 59) and rejects queries containing `/` or `\` (line 79). A post-filter in `fileItems` cannot work — for a jamo query like `ㅎㄱ` the walker returns *nothing to filter*. The walker's match predicates must become chosung-aware, in the same additive style as every other surface:

```go
// inside Search, for files:
switch {
case strings.Contains(nameLower, query):
	basenameHits = append(basenameHits, SearchResult{Path: rel})
case chosungMatch(name, query):
	basenameHits = append(basenameHits, SearchResult{Path: rel})
case pathSegmentContains(rel, query):
	segmentHits = append(segmentHits, SearchResult{Path: rel})
case chosungMatch(rel, query):
	segmentHits = append(segmentHits, SearchResult{Path: rel})
}

// inside Search, for directories (mirror the literal dir check):
if strings.Contains(strings.ToLower(name), query) || chosungMatch(name, query) {
	dirHits = append(dirHits, SearchResult{Path: rel, IsDir: true})
}
```

With the shared helper next to `Search`:

```go
// chosungMatch applies the 초성 matcher only when the query actually contains
// jamo; literal queries keep today's exact behavior and cost.
func chosungMatch(candidate, query string) bool {
	return textutil.HasJamo(query) && textutil.ChoseongMatch(candidate, query)
}
```

Semantics preserved: `minQueryLen = 2` still applies (single-jamo walk queries are rejected, consistent with today — the current-directory listing in `fileItems` still handles them), and the slash rejection still applies (the `@`-token frag never contains `/` after `splitPathToken`). The `pathSegmentContains` chosung variant matches per segment (`ChoseongOf` never decomposes `/`, and segment-scoped matching mirrors the literal behavior). `fileItems` and `searchFileRefs` need **no changes** — they pass the frag through unchanged.

**Step 5: Add chosung to `resourceItems` (MCP resources)**

Apply the same OR pattern to the strings already being prefix-checked (`ref` at top level, `r.URI` per-server):

```go
		case server == "":
			if !strings.HasPrefix(ref, frag) && !textutil.ChoseongMatch(ref, frag) {
				continue
			}
		case r.Server == server:
			if !strings.HasPrefix(r.URI, frag) && !textutil.ChoseongMatch(r.URI, frag) {
				continue
			}
```

**Step 6: Add `@`-ref tests**

In `complete_test.go`, follow the existing fixtures exactly:

- **Current-directory listing** (the `readDir` path — this is what the plan's manual demo exercises): copy the `TestFileItemsOneLevel` pattern (`complete_test.go:291`) — `t.TempDir()` + `writeAt(t, dir, "한국어문서.md", "x")` + `m.fileItems(dir + "/ㅎㄱ")`. Assert the file is offered for `ㅎㄱ`, and that `ㄷㄷ` (runes absent from the projection `ㅎㄱㅇㅁㅅ.md`) is not. Absolute-path tokens contain `/`, so the top-level walk branch does not fire — this test isolates the listing path. Also assert `@한` (literal prefix) still offers it and `@한국` (literal) does.
- **Workspace walk** (the `fileref.Search` path): copy the `TestFileItemsSearchesBasenameAtTopLevel` pattern (`complete_test.go:344`) — chdir into a temp workspace root, `m.ctrl = control.New(control.Options{SessionDir: t.TempDir(), WorkspaceRoot: workspace})` (pattern from `TestFileItemsSubdirUsesWorkspaceRoot`, line 320), a Korean-named file in a **subdirectory**, token `ㅎㄱ`. Assert the file is offered.
- Fixture references: `newTestChatTUI` lives in `chat_render_test.go:16`; `hasLabel`/`labels` in `complete_test.go:605/613`.

**Step 7: Verify**

Run: `go test ./internal/cli/ -run 'TestFuzzyFilterSlash|TestFileItems|TestAtItems|TestResource' -v && go test ./internal/fileref/ -v`
Expected: pass, including the rewritten contract test and the new fileref chosung cases.

**Step 8: Commit**

```bash
git add internal/cli/complete.go internal/cli/complete_test.go
git commit -m "feat(cli): chosung matching in completion surfaces (PAT-1398)"
```

---

## Task 4: `select.go` — the generic-menu hub (sessions, language, providers, models, pickers)

**Files:**
- Modify: `internal/cli/select.go` — `filterMenuItems` (58-72), `filterIndices` (481-496)
- Test: `internal/cli/select_test.go`

**Step 1: Write the failing test**

```go
func TestFilterMenuItemsChosung(t *testing.T) {
	items := []menuItem{
		{name: "패키지 배포 수정", desc: "session from today"},
		{name: "README", desc: "docs"},
	}
	if got := filterMenuItems(items, "ㅂㅍ"); len(got) != 1 || got[0].name != "패키지 배포 수정" {
		t.Fatalf("ㅂㅍ should match the Korean item, got %+v", got)
	}
	if got := filterMenuItems(items, "login"); len(got) != 0 {
		t.Fatalf("jamo-free query must not chosung-match, got %+v", got)
	}
	if got := filterMenuItems(items, "README"); len(got) != 1 {
		t.Fatalf("literal matching unchanged, got %+v", got)
	}
	// name-only match must survive the name/desc OR
	if got := filterMenuItems(items, "docs"); len(got) != 1 || got[0].name != "README" {
		t.Fatalf("desc literal matching unchanged, got %+v", got)
	}
}
```

**Step 2: Run to verify it fails**

Run: `go test ./internal/cli/ -run TestFilterMenuItemsChosung -v`
Expected: FAIL (no chosung branch yet).

**Step 3: Implement — add the chosung OR to both filters**

`filterMenuItems`:

```go
	lq := strings.ToLower(query)
	var out []menuItem
	for _, it := range items {
		if strings.Contains(strings.ToLower(it.name), lq) ||
			strings.Contains(strings.ToLower(it.desc), lq) ||
			textutil.ChoseongMatch(it.name, query) ||
			textutil.ChoseongMatch(it.desc, query) {
			out = append(out, it)
		}
	}
```

`filterIndices`: the identical OR inside its loop.

This one edit is the hub for the **non-TUI generic menus**: the CLI `/resume` session picker (`cli.go:1608`), language selector, provider/model selectors, setup manager, and custom-add-method menus — all built on `selectOne`/`selectMany`. (The TUI's searchable overlays — `/model`, `/provider`, `/resume` in the chat UI — do **not** route through `select.go`; they use `quickPicker` and are wired in Task 4b.)

**Step 4: Verify**

Run: `go test ./internal/cli/ -run 'TestFilterMenuItems|TestFilterIndices' -v`
Expected: pass.

**Step 5: Commit**

```bash
git add internal/cli/select.go internal/cli/select_test.go
git commit -m "feat(cli): chosung matching in generic select menus (PAT-1398)"
```

---

## Task 4b: `quick_picker.go` — TUI searchable overlays (`/model`, `/provider`, `/compliance`, output style, TUI `/resume`)

**Files:**
- Modify: `internal/cli/quick_picker.go` — `filteredItems` (50-64)
- Test: `internal/cli/quick_picker_test.go`

**Step 1: Write the failing test**

Follow the existing `quick_picker_test.go` fixture: a `quickPicker` with items whose label/description contains Korean (e.g. a `/model` item labeled `모델변경`). Assert `filteredItems()` with query `ㅁㄷ` returns it; a jamo-free query behaves exactly as today.

**Step 2: Run to verify it fails**

Run: `go test ./internal/cli/ -run TestQuickPicker -v`
Expected: FAIL.

**Step 3: Implement**

```go
func (p *quickPicker) filteredItems() []quickPickerItem {
	if p == nil || strings.TrimSpace(p.query) == "" {
		if p == nil {
			return nil
		}
		return p.items
	}
	query := strings.ToLower(strings.TrimSpace(p.query))
	out := make([]quickPickerItem, 0, len(p.items))
	for _, item := range p.items {
		haystack := strings.ToLower(item.Label + " " + item.Description + " " + item.Status)
		if strings.Contains(haystack, query) || textutil.ChoseongMatch(haystack, p.query) {
			out = append(out, item)
		}
	}
	return out
}
```

This one edit covers the searchable TUI overlays that the issue's session/history and MCP-tool search surfaces actually use in the chat UI: `quickPickerModel` (model.go:71), `quickPickerProvider` (provider.go:59, 122), the compliance picker (compliance_picker.go:37), output-style picker (output_style_picker.go:32), and the TUI `/resume` overlay, whose search delegates to a `quickPicker` (resume_picker.go:66-86) over `sessionPickerLabel` haystacks.

**Step 4: Verify** — `go test ./internal/cli/ -run 'TestQuickPicker|TestResume' -v` → pass.

**Step 5: Commit**

```bash
git add internal/cli/quick_picker.go internal/cli/quick_picker_test.go
git commit -m "feat(cli): chosung matching in quick-picker overlays (PAT-1398)"
```

---

## Task 5: `skill_picker.go` — skill search

**Files:**
- Modify: `internal/cli/skill_picker.go` — `filteredSkills` (367-380)
- Test: `internal/cli/skill_picker_test.go`

**Step 1: Write the failing test**

Follow the existing `skill_picker_test.go` fixture (construct a picker with `skills` containing a `skill.Skill` whose `SlashName()` is Korean, e.g. `스킬설정`). Assert `filteredSkills()` with query `ㅅㅅㅅ` returns it; a jamo-free query still behaves as today.

**Step 2: Run to verify it fails**

Run: `go test ./internal/cli/ -run TestFilteredSkills -v`
Expected: FAIL.

**Step 3: Implement**

```go
func (p *skillPicker) filteredSkills() []skill.Skill {
	if p.query == "" {
		return p.skills
	}
	q := strings.ToLower(p.query)
	var out []skill.Skill
	for _, s := range p.skills {
		if strings.Contains(strings.ToLower(s.SlashName()), q) ||
			strings.Contains(strings.ToLower(s.Plugin), q) ||
			strings.Contains(strings.ToLower(s.Description), q) ||
			textutil.ChoseongMatch(s.SlashName(), p.query) ||
			textutil.ChoseongMatch(s.Plugin, p.query) ||
			textutil.ChoseongMatch(s.Description, p.query) {
			out = append(out, s)
		}
	}
	return out
}
```

**Step 4: Verify** — `go test ./internal/cli/ -run TestFilteredSkills -v` → pass.

**Step 5: Commit**

```bash
git add internal/cli/skill_picker.go internal/cli/skill_picker_test.go
git commit -m "feat(cli): chosung matching in skill picker (PAT-1398)"
```

---

## Task 6: `control/slash.go` — arg completion (`/mcp`, `/skill`, `/plugins`, `/model`, …)

**Files:**
- Modify: `internal/control/slash.go` — `filterSlash` (346-360)
- Test: `internal/control/slash_test.go`

**Step 1: Write the failing test**

Using the existing `slash_test.go` fixture: a `SlashItem{Label: "모델변경"}` must be kept for current token `ㅁㄷ`; a jamo-free token behaves exactly as today (prefix).

**Step 2: Run to verify it fails** — `go test ./internal/control/ -run TestFilterSlash -v` → FAIL.

**Step 3: Implement**

```go
	for _, it := range items {
		if !strings.HasPrefix(strings.ToLower(it.Label), lp) &&
			!textutil.ChoseongMatch(it.Label, cur) {
			continue
		}
		...
```

This covers the plugin marketplace (`/plugins`), MCP server/tool completion (`/mcp`), skills (`/skill`), model/provider/theme/language/memory/effort arg lists — all through the one arg-completion chokepoint.

**Step 4: Surface audit for picker overlays — expected outcome (verified during plan review)**

The following were checked line-by-line during plan review and need **no** chosung wiring: `mcp_import_picker.go`, `mcp_manager.go`, `rewind.go`, `copy_picker.go` (no search/filter logic); `compliance_picker.go` and `output_style_picker.go` (route through `quickPicker`, covered by Task 4b); the `/plugins` marketplace surface is text output (`control/slash.go:404-429`) plus arg completion — no browse UI in the CLI. Run this grep as a cheap confirmation that nothing was missed:

Run: `grep -rnE "HasPrefix|Contains" internal/cli/chat_tui.go | grep -iE "query|frag|filter|search"`
Expected: only `runSlashCommand`'s `/mcp__` prefix dispatch (line 4963) — no list-filtering site.

**Step 5: Verify** — `go test ./internal/control/ ./internal/cli/ -run 'TestFilterSlash|TestMCP|TestPlugin'` → pass.

**Step 6: Commit**

```bash
git add internal/control/slash.go internal/control/slash_test.go internal/cli/chat_tui.go
git commit -m "feat(control): chosung matching in arg completion (PAT-1398)"
```

---

## Task 7: `shell_completion.go` — shell-level command completion

**Files:**
- Modify: `internal/cli/shell_completion.go` — `filterCompletionPrefix` (599-605)
- Test: `internal/cli/shell_completion_test.go` (exists — extend it)

**Step 1-3: Write test (fails) → implement**

Same OR pattern: `filterCompletionPrefix` gains `|| textutil.ChoseongMatch(value, prefix)` (only reached when the prefix contains jamo, per D3). This is the CLI binary's own shell-completion surface — the same "모든 검색/필터 표면" mandate, one line.

**Step 4: Verify** — `go test ./internal/cli/ -run Completion` → pass.

**Step 5: Commit**

```bash
git add internal/cli/shell_completion.go
git commit -m "feat(cli): chosung matching in shell completion (PAT-1398)"
```

---

## Task 8: Full verification, quality loop, manual pass

**Files:** none (verification only)

**Step 1: Full suite + vet**

Run: `go build ./... && go vet ./internal/... && go test ./internal/... 2>&1 | tail -10`
Expected: clean build, vet clean, all tests pass with no regressions vs the Task 0 baseline.

**Step 2: Team-protocol loop**

Run the three-stage loop once (code-review → structure-code → improve-codebase), fix every finding, re-run each stage until clean. The review should specifically check: no surface left with literal-only filtering, no duplicated jamo logic outside `textutil`, the matcher spec in this document matches the shipped code.

**Step 3: Manual TTY validation (the harness is a CLI — no browser surface)**

Build and run the harness locally, then drive these scenarios as Patrick:

1. Slash palette: type `/ㅁㄷ` → `/model` (`모델변경`) appears; type `/ㅇㅊ` → `/compact` (`압축`) appears; confirm `/copy` does not.
2. `@`-reference: in a temp workspace with `한국어문서.md` at the **workspace root** (one directory level), type `@ㅎㄱ` → the file is offered; type `@한` (literal) → still offered; type `@ㄷㄷ` (runes absent from the projection `ㅎㄱㅇㅁㅅ.md`) → not offered. For a subdirectory file, verify via the workspace-walk test (Task 3 Step 6) rather than manually.
3. `/resume` picker search: with a Korean-topic session in the list, press `/` in the picker and type `ㅂㅍ` → the session filters in.
4. `/skill` search and `/mcp` arg completion: verify with a Korean-named skill and MCP server.
5. English regression: a Latin query behaves exactly as before (prefix/substring) — spot-check `/comp` → `/compact`, `@RE` → `README`.

Record observations (exact input, exact output) for the Linear completion comment.

**Step 4: Commit any verification fixes**

---

## Task 9: Linear — completion comment (Korean) and state

Per the fix-linear-issue flow:

1. Post the completion comment in **Korean** on PAT-1398: 원인 (duplicated inline decomposition + alias-prefix-only matcher), 수정 내용 (textutil package + per-surface wiring, commit SHAs), 로컬 검증 (the Task 8 manual scenarios), 결론 (ready for QA).
2. Move PAT-1398 to **Validating**.
3. Push the branch (cap skill — one conventional commit set referencing PAT-1398).

---

## Decisions & non-goals (recorded so reviewers can challenge them)

- **D1** (behavior change, **user-approved** during plan review): slash-palette chosung widens prefix-only → subsequence. Test rewritten in Task 3. Rationale: one matcher everywhere; issue's stated goal is a uniform 초성 filter.
- **D2**: double consonants distinct by construction — tests added, no special-case code.
- **D3**: `ChoseongMatch` never matches jamo-free queries → zero English behavior change, zero perf cost. This is the issue's "비한국어 패스스루" made structural.
- **Non-goal**: no caching of `ChoseongOf` projections (bounded item lists per keystroke; YAGNI — memoize only if profiling demands it later).
- **Revised scope**: `internal/fileref.Search` **is** modified, but only its match predicates (Task 3 Step 4). Plan review proved the post-filter alternative unworkable: the walker matches literal text only, so a jamo query returns nothing to filter. The change is the same additive OR used everywhere else.
- **Non-goal**: desktop global search (Cmd+K) — explicitly future work in the issue.
- **Follow-up (not this issue)**: `complete.go`'s `subsequenceMatch` and textutil's internal `subseqAfter` are both tiny subsequence matchers; consolidating them into one exported helper is a possible cleanup but out of scope to keep this diff minimal.
- **Issue's future work, out of scope**: grep-result chosung highlighting, desktop global search (Cmd+K), weighted chosung ranking — all explicitly deferred by the issue itself.
