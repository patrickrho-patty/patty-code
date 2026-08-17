// Package policy holds the cross-package rules that decide which
// provider kinds are excluded from the linked build profile. ADR G4
// says generic HTTP-protocol providers (openai, anthropic, responses,
// dashscope-responses) compile only into public builds; the former
// env hatch is retired — a compile-time gate cannot be runtime-
// undone. The official Harness must not use OpenAI/Anthropic/REST for
// Patty service inference.
//
// Extracted from provider/provider.go so the gating decision can be
// consulted from any package without dragging the provider
// registration registry along with it.
package policy

import "patty/internal/tier"

// genericBlockedKinds are the generic HTTP-protocol LLM providers
// excluded from non-public build profiles by the DARI-only policy
// (PRD v2 §0.2, §826). Package-private — the only consult path is
// IsBlockedKind; operators must not enumerate the list directly.
var genericBlockedKinds = map[string]bool{
	"openai":              true,
	"anthropic":           true,
	"responses":           true,
	"dashscope-responses": true,
}

// LegacyPaperKind maps the historical provider kind to its DARI
// successor so pre-migration configs keep resolving.
const LegacyPaperKind = "paper"

// IsBlockedKind reports whether kind is excluded from the linked
// build profile (ADR G4). Generic HTTP-protocol providers compile
// only into public builds.
func IsBlockedKind(kind string) bool {
	if tier.Default.Allows(tier.CapGenericProviders) {
		return false
	}
	return genericBlockedKinds[kind]
}
