// at_completion.go holds the "@" completion namespace: the directory-level
// file references, the bounded workspace walk, and MCP resources sharing the
// token. Keeping it out of complete.go bounds both files' size and scope.

package cli

import (
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"patty/internal/control"
	"patty/internal/fileref"
	"patty/internal/textutil"
)

// fileItems lists one directory level for a path token. dir is the part up to
// the last '/', frag the part after; entries of dir starting with frag are
// offered (directories descend, files complete). Hidden entries are skipped
// unless frag starts with '.'. Top-level tokens also surface MCP resources.
func (m *chatTUI) fileItems(token string) []compItem {
	dir, frag := splitPathToken(token)
	// The typed token may carry backslash-escaped spaces (the form completion
	// itself inserts); filesystem lookups need the real path while inserts keep
	// the escaped grammar.
	fsFrag := control.UnescapeRefPath(frag)
	workspaceRoot := ""
	if m.ctrl != nil {
		workspaceRoot = m.ctrl.WorkspaceRoot()
	}
	readDir := control.UnescapeRefPath(dir)
	if workspaceRoot != "" {
		if readDir == "" {
			readDir = workspaceRoot
		} else if !filepath.IsAbs(readDir) {
			readDir = filepath.Join(workspaceRoot, filepath.FromSlash(readDir))
		}
	} else if readDir == "" {
		readDir = "."
	}
	entries, err := os.ReadDir(readDir)
	if err != nil {
		entries = nil
	}
	// Directories first, then files; ReadDir is already name-sorted.
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].IsDir() && !entries[j].IsDir()
	})

	showHidden := strings.HasPrefix(fsFrag, ".")
	var items []compItem
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, fsFrag) && !textutil.ChoseongMatch(name, fsFrag) {
			continue
		}
		if !showHidden && strings.HasPrefix(name, ".") {
			continue
		}
		if e.IsDir() {
			items = append(items, compItem{label: name + "/", insert: "@" + dir + control.EscapeRefPath(name) + "/", hint: "dir", descend: true})
		} else {
			items = append(items, compItem{label: name, insert: "@" + dir + control.EscapeRefPath(name)})
		}
		if len(items) >= maxCompItems {
			break
		}
	}

	// At the top level (still naming the first segment) MCP resources share the
	// '@' namespace, so offer the matching ones too.
	if !strings.Contains(token, "/") {
		seen := map[string]bool{}
		for _, it := range items {
			seen[strings.TrimPrefix(it.insert, "@")] = true
		}
		remaining := min(maxCompItems-len(items), maxFileSearchItems)
		results := m.searchFileRefs(fsFrag)
		if len(results) > remaining {
			results = results[:remaining]
		}
		for _, path := range results {
			escaped := control.EscapeRefPath(path)
			if seen[escaped] {
				continue
			}
			items = append(items, compItem{label: path, insert: "@" + escaped, hint: "file"})
			if len(items) >= maxCompItems {
				break
			}
		}
		items = append(items, m.resourceItems("", token)...)
	}
	return items
}

// searchFileRefs memoizes the bounded basename walk so re-rendering the menu
// for an unchanged @token fragment doesn't re-walk the workspace each keystroke.
func (m *chatTUI) searchFileRefs(frag string) []string {
	if m.fileSearchCache == nil {
		m.fileSearchCache = map[string][]string{}
	}
	if r, ok := m.fileSearchCache[frag]; ok {
		return r
	}
	searchRoot := "."
	if m.ctrl != nil {
		if wr := m.ctrl.WorkspaceRoot(); wr != "" {
			searchRoot = wr
		}
	}
	// ChoseongMatch doubles as the MatchFunc seam: it returns false for
	// jamo-free fragments, so literal search behavior is unchanged (D3).
	results := fileref.SearchMatch(searchRoot, frag, maxFileSearchItems, textutil.ChoseongMatch)
	paths := make([]string, 0, len(results))
	for _, r := range results {
		paths = append(paths, r.Path)
	}
	m.fileSearchCache[frag] = paths
	return paths
}

// splitPathToken splits a path token into (dir, frag): dir keeps its trailing
// slash ("internal/" ), frag is the segment being typed.
func splitPathToken(token string) (dir, frag string) {
	if i := strings.LastIndex(token, "/"); i >= 0 {
		return token[:i+1], token[i+1:]
	}
	return "", token
}

// isMCPServer reports whether name is a connected MCP server.
func (m *chatTUI) isMCPServer(name string) bool {
	if m.host == nil {
		return false
	}
	return slices.Contains(m.host.ServerNames(), name)
}

// resourceItems lists MCP resources as @server:uri completions. When server is
// "" (top level) it matches by the whole "server:uri" prefix; otherwise it lists
// the named server's resources filtered by the uri prefix.
func (m *chatTUI) resourceItems(server, frag string) []compItem {
	if m.host == nil {
		return nil
	}
	var items []compItem
	for _, r := range m.host.Resources() {
		ref := r.Server + ":" + r.URI
		switch {
		case server == "":
			if !strings.HasPrefix(ref, frag) && !textutil.ChoseongMatch(ref, frag) {
				continue
			}
		case r.Server == server:
			if !strings.HasPrefix(r.URI, frag) && !textutil.ChoseongMatch(r.URI, frag) {
				continue
			}
		default:
			continue
		}
		label := r.Name
		if label == "" {
			label = "resource"
		}
		items = append(items, compItem{label: "@" + ref, insert: "@" + ref, hint: label})
	}
	return items
}
