//go:build !darwin

package cli

func currentKeyboardInputSourceID() string { return "" }

func currentPhysicalBackspaceDown() bool { return false }

func physicalBackspaceStateAvailable() bool { return false }

func prewarmKeyboardCompatibilityAPIs() {}
