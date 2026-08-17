//go:build profile_public

package main

// Generic providers register only in public builds (ADR G4). This file is
// the desktop side of the gate; cmd/patcode/providers_public.go wires the
// same packages into the public CLI.
import (
	_ "patty/internal/provider/anthropic"
	_ "patty/internal/provider/openai"
	_ "patty/internal/provider/responses"
)
