//go:build profile_public

package main

// Generic providers register only in public builds (ADR G4). This file is
// the desktop side of the gate; cmd/patcode needs no equivalent because it
// never blank-imported the generic packages.
import (
	_ "patty/internal/provider/anthropic"
	_ "patty/internal/provider/openai"
	_ "patty/internal/provider/responses"
)
