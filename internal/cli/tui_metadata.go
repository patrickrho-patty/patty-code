package cli

import (
	"strings"

	"patty/internal/agent"
)

// tuiSessionIdentity is the optional identity surface carried by the TUI. The
// TUI can render stable session identity through a deliberate detail surface
// later without teaching every chrome renderer about individual ID fields.
// Harness and user IDs remain empty until their owning frontend supplies them.
type tuiSessionIdentity struct {
	SessionID string
	HarnessID string
	UserID    string
}

// tuiDisplayMetadata is the read-only projection consumed by TUI chrome and
// external statusline integrations. It is intentionally a display contract,
// not a replacement for controller domain state.
type tuiDisplayMetadata struct {
	Identity      tuiSessionIdentity
	Model         string
	ModelRef      string
	Effort        string
	ContextUsed   int
	ContextWindow int
	BuildVersion  string
}

// tuiStatuslineContext is the stable, extensible JSON contract for custom
// statusline commands. Optional identity fields are omitted when unavailable,
// preserving the existing payload for frontends that do not provide them.
type tuiStatuslineContext struct {
	Model         string `json:"model"`
	ContextUsed   int    `json:"contextUsed"`
	ContextWindow int    `json:"contextWindow"`
	CWD           string `json:"cwd"`
	SessionID     string `json:"sessionId,omitempty"`
	HarnessID     string `json:"harnessId,omitempty"`
	UserID        string `json:"userId,omitempty"`
}

func (m chatTUI) displayMetadata() tuiDisplayMetadata {
	metadata := tuiDisplayMetadata{
		Model:        strings.TrimSpace(m.label),
		ModelRef:     strings.TrimSpace(m.modelRef),
		Effort:       strings.TrimSpace(m.effortLevel),
		BuildVersion: activeBuildVersion,
		Identity:     m.displayIdentity,
	}
	if m.ctrl == nil {
		return metadata
	}
	metadata.ContextUsed, metadata.ContextWindow = m.ctrl.ContextSnapshot()
	if metadata.Identity.SessionID == "" {
		metadata.Identity.SessionID = strings.TrimSpace(agent.BranchID(m.ctrl.SessionPath()))
	}
	return metadata
}

func (m chatTUI) statuslineContext(cwd string) tuiStatuslineContext {
	metadata := m.displayMetadata()
	return tuiStatuslineContext{
		Model:         metadata.Model,
		ContextUsed:   metadata.ContextUsed,
		ContextWindow: metadata.ContextWindow,
		CWD:           cwd,
		SessionID:     metadata.Identity.SessionID,
		HarnessID:     metadata.Identity.HarnessID,
		UserID:        metadata.Identity.UserID,
	}
}
