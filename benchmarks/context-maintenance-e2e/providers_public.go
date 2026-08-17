//go:build profile_public

// This benchmark drives the real DeepSeek API through the generic
// OpenAI-compatible provider — a public-build capability (ADR G4) — so its
// registration lives behind the profile tag. On non-public builds the tool
// still compiles; provider.New rejects the generic kind at runtime.
package main

import _ "patty/internal/provider/openai"
