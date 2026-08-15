package dariproto

import (
	"strconv"
	"strings"
)

// VersionBelow reports whether v sorts below minV as a dotted numeric
// version (a leading v is tolerated; non-numeric segments compare
// lexicographically). It mirrors the relay's floor comparison; the
// cross-repo conformance suite pins the semantics.
func VersionBelow(v, minV string) bool {
	norm := func(s string) []string {
		s = strings.TrimPrefix(strings.TrimPrefix(s, "v"), "V")
		return strings.Split(s, ".")
	}
	a, b := norm(v), norm(minV)
	for i := 0; i < len(a) && i < len(b); i++ {
		ai, aerr := strconv.Atoi(a[i])
		bi, berr := strconv.Atoi(b[i])
		if aerr != nil || berr != nil {
			if aerr != berr {
				// Mixed numeric/non-numeric: the non-numeric side
				// (dev builds) sorts below a numeric floor.
				return aerr != nil
			}
			if a[i] != b[i] {
				return a[i] < b[i]
			}
			continue
		}
		if ai != bi {
			return ai < bi
		}
	}
	return false
}
