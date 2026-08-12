// Package docs exposes the product documentation bundled with each Patty Code
// build. Runtime consumers should retrieve focused sections instead of adding
// the full corpus to the provider-visible prompt.
package docs

import "embed"

// Content contains the Markdown documentation shipped by this exact build.
// Images, schemas, and generated site assets are intentionally excluded from
// the agent retrieval corpus.
//
//go:embed README.md guides/*.md reference/*.md extensions/*.md operations/*.md themes/*.md
var Content embed.FS
