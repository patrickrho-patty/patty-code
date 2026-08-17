//go:build profile_public

package main

// Generic providers register only in public builds (ADR G4). This file is
// the patcode side of the gate; it mirrors desktop/providers_public.go.
// Without these blank imports the public CLI compiles the BYOK preset
// catalog but cannot construct any of its providers — provider.New would
// fail every generic kind with "unknown kind".
import (
	_ "patty/internal/provider/anthropic"
	_ "patty/internal/provider/openai"
	_ "patty/internal/provider/responses"
)
