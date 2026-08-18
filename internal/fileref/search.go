package fileref

import (
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
)

var skipEntryNames = map[string]bool{
	".codex":       true,
	".DS_Store":    true,
	".git":         true,
	".npm":         true,
	".pnpm-store":  true,
	"node_modules": true,
	"Thumbs.db":    true,
}

// skipDirNames are build outputs across ecosystems: their contents are
// generated, so an "@" hit inside one points at a file nobody edits (#3900).
var skipDirNames = map[string]bool{
	"build":         true,
	"dist":          true,
	"target":        true,
	"__pycache__":   true,
	"venv":          true,
	".venv":         true,
	".gradle":       true,
	".next":         true,
	".nuxt":         true,
	".svelte-kit":   true,
	".pytest_cache": true,
	".mypy_cache":   true,
	".tox":          true,
	".terraform":    true,
	".dart_tool":    true,
}

// SkipEntry reports whether a workspace entry is hidden from file pickers. rel
// is the entry's slash-separated path from the workspace root.
func SkipEntry(rel, name string, isDir bool) bool {
	if skipEntryNames[name] {
		return true
	}
	return isDir && (skipDirNames[name] || skipDirPaths[rel])
}

var skipDirPaths = map[string]bool{
	"bin":                      true,
	"desktop/frontend/wailsjs": true,
	"npm/.stage":               true,
	"site/.astro":              true,
	"stage":                    true,
	"tmp":                      true,
}

const (
	minQueryLen    = 2
	maxWalkEntries = 10000
)

// SearchResult is a single entry returned by Search. It carries the relative
// path (slash-normalized) and whether the entry is a directory, so callers
// can present the correct icon and append "/" vs " " on selection.
type SearchResult struct {
	Path  string
	IsDir bool
}

// MatchFunc is an extra match predicate layered on top of Search's literal
// substring matching. It receives a candidate (basename, path segment, or
// directory name) and the cleaned query; returning true records a hit. It stays
// an opaque function so leaf packages can extend matching without importing the
// caller's domain logic (e.g. Korean 초성).
type MatchFunc func(candidate, query string) bool

// Search finds entries under root whose path matches query. A match is
// recorded when the query is a substring of the file's basename (preferred
// tier), of any slash-separated path segment (fallback tier), or of a
// directory name (lowest tier). It is bounded by limit and skips common
// generated/vendor directories so interactive completion stays responsive on
// large workspaces.
func Search(root, query string, limit int) []SearchResult {
	return SearchMatch(root, query, limit, nil)
}

// SearchMatch behaves like Search but also records a hit whenever extra
// matches, applied at each literal tier: the basename (preferred), each
// slash-separated path segment (fallback), and directory names (lowest).
// extra may be nil.
func SearchMatch(root, query string, limit int, extra MatchFunc) []SearchResult {
	query = strings.ToLower(strings.TrimSpace(query))
	if len(query) < minQueryLen || strings.ContainsAny(query, `/\`) || limit <= 0 {
		return nil
	}

	s := &searchState{
		root:       root,
		query:      query,
		showHidden: strings.HasPrefix(query, "."),
		extra:      extra,
	}
	visited := 0
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		return s.visit(path, d, err, &visited)
	})
	sort.Slice(s.basenameHits, func(i, j int) bool { return s.basenameHits[i].Path < s.basenameHits[j].Path })
	sort.Slice(s.segmentHits, func(i, j int) bool { return s.segmentHits[i].Path < s.segmentHits[j].Path })
	sort.Slice(s.dirHits, func(i, j int) bool { return s.dirHits[i].Path < s.dirHits[j].Path })
	// Directories first so the user can navigate into them; then basename
	// hits (most relevant file matches); then path-segment hits. We reserve
	// up to dirQuota slots for directories so they are never fully crowded
	// out by a large number of file matches.
	const dirQuota = 5
	out := make([]SearchResult, 0, limit)
	nDirs := min(len(s.dirHits), dirQuota)
	out = append(out, s.dirHits[:nDirs]...)
	remaining := limit - len(out)
	if remaining > 0 {
		if len(s.basenameHits) > remaining {
			s.basenameHits = s.basenameHits[:remaining]
		}
		out = append(out, s.basenameHits...)
		remaining = limit - len(out)
	}
	if remaining > 0 {
		if len(s.segmentHits) > remaining {
			s.segmentHits = s.segmentHits[:remaining]
		}
		out = append(out, s.segmentHits...)
	}
	return out
}

// searchState carries the accumulated walker state for SearchMatch. It exists
// so the WalkDir callback stays a small method instead of a deeply nested
// closure, keeping cyclomatic complexity per function low.
type searchState struct {
	root         string
	query        string
	showHidden   bool
	extra        MatchFunc
	basenameHits []SearchResult
	segmentHits  []SearchResult
	dirHits      []SearchResult
}

// visit records one directory entry (and its children) into the hit lists. It
// returns the WalkDir control flow (SkipDir/SkipAll) exactly like the inline
// callback it replaced.
func (s *searchState) visit(path string, d fs.DirEntry, err error, visited *int) error {
	if err != nil {
		if d != nil && d.IsDir() {
			return filepath.SkipDir
		}
		return nil
	}
	if path == s.root {
		return nil
	}
	*visited++
	if *visited > maxWalkEntries {
		return filepath.SkipAll
	}

	name := d.Name()
	if d.IsDir() {
		rel, rerr := filepath.Rel(s.root, path)
		if rerr != nil {
			return filepath.SkipDir
		}
		rel = filepath.ToSlash(rel)
		if SkipEntry(rel, name, true) || (!s.showHidden && strings.HasPrefix(name, ".")) {
			return filepath.SkipDir
		}
		// Allow matching directory names so the user can select a
		// folder directly from the @-menu instead of only its contents.
		if matchEntry(s.extra, s.query, name) {
			s.dirHits = append(s.dirHits, SearchResult{Path: rel, IsDir: true})
		}
		return nil
	}
	if skipEntryNames[name] {
		return nil
	}
	if !s.showHidden && strings.HasPrefix(name, ".") {
		return nil
	}
	if info, ierr := d.Info(); ierr != nil || !info.Mode().IsRegular() {
		return nil
	}
	rel, rerr := filepath.Rel(s.root, path)
	if rerr != nil {
		return nil
	}
	rel = filepath.ToSlash(rel)
	switch {
	case matchEntry(s.extra, s.query, name):
		s.basenameHits = append(s.basenameHits, SearchResult{Path: rel})
	case matchSegment(s.extra, s.query, rel):
		s.segmentHits = append(s.segmentHits, SearchResult{Path: rel})
	}
	return nil
}

// matchEntry reports a hit for a single candidate (basename or directory name):
// either the literal substring rule or the caller-supplied extra matcher.
func matchEntry(extra MatchFunc, query, candidate string) bool {
	if strings.Contains(strings.ToLower(candidate), query) {
		return true
	}
	return extra != nil && extra(candidate, query)
}

// matchSegment applies the same rule per path segment so a match never crosses
// a '/' boundary (mirrors pathSegmentContains' literal semantics).
func matchSegment(extra MatchFunc, query, relSlash string) bool {
	if pathSegmentContains(relSlash, query) {
		return true
	}
	if extra == nil {
		return false
	}
	for seg := range strings.SplitSeq(relSlash, "/") {
		if extra(seg, query) {
			return true
		}
	}
	return false
}

// pathSegmentContains reports whether query appears in any slash-separated
// segment of the slash-normalized relative path. The basename is matched
// independently by the caller, so this helper is meaningful only for
// directories above the file (e.g. "src/planind/index.tsx" with query
// "planind" matches the "planind" segment).
func pathSegmentContains(relSlash, queryLower string) bool {
	for seg := range strings.SplitSeq(relSlash, "/") {
		if strings.Contains(strings.ToLower(seg), queryLower) {
			return true
		}
	}
	return false
}
