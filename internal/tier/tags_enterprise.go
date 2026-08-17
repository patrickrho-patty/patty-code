//go:build !profile_public && !profile_sovereign

package tier

// Enterprise is the default profile: a bare `go build` with no tags.
const Default = Enterprise
