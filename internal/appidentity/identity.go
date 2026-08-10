package appidentity

const (
	// AppUserModelID is shared by every current-generation Windows process,
	// shortcut, and toast that users perceive as patty. Keep it
	// version-independent across upgrades.
	AppUserModelID = "patty.code"

	// legacyTauriAppUserModelID was written to Windows shortcuts by patty
	// Desktop 0.53. Keep the current identity distinct so separately installed
	// Tauri and Wails generations do not merge into one taskbar group.
	legacyTauriAppUserModelID = "dev.patty.desktop"
)
