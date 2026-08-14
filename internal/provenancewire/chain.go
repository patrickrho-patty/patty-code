package provenancewire

import (
	"fmt"
	"strings"
)

// SpanLookupKey is the cross-repo key the relay's
// `provenance.LookupCodeSpan` uses to find a span by file path
// and line range. The connector emits these keys from its
// attribution-chain reader.
type SpanLookupKey struct {
	FilePath  string
	StartLine int
	EndLine   int
}

// String renders the key as a deterministic label suitable for
// logs and audit chains.
func (k SpanLookupKey) String() string {
	return fmt.Sprintf("%s:%d-%d", k.FilePath, k.StartLine, k.EndLine)
}

// AttributionChain is the harness-side view of the prompt -> tool ->
// file -> commit chain (PRD §19.1). The harness surfaces this in
// the author UI and uses it to power replay-from-provenance.
type AttributionChain struct {
	ExchangeID     string
	SessionID      string
	UserID         string
	HarnessID      string
	ModelPackageID string
	EndpointID     string
	PolicyEpochID  string
	LeaseID        string
	Prompts        []string
	Tools          []string
	Files          []string
	Commits        []string
	Spans          []SpanLookupKey
}

// String renders the chain as a deterministic label suitable for
// audit logs.
func (c *AttributionChain) String() string {
	return fmt.Sprintf("chain{exchange=%s session=%s user=%s harness=%s files=%d}",
		c.ExchangeID, c.SessionID, c.UserID, c.HarnessID, len(c.Files))
}

// ReplayPlan is the input the harness feeds to its replay engine
// (PRD §14.3). It captures the prompt, model, and toolset the
// exchange used so a downstream replay reproduces the same code.
type ReplayPlan struct {
	ExchangeID     string
	SessionID      string
	Prompt         string
	ModelPackageID string
	EndpointID     string
	Files          []string
	ToolClasses    []string
}

// String renders the replay plan as a deterministic label.
func (p *ReplayPlan) String() string {
	return fmt.Sprintf("replay{exchange=%s model=%s files=%d}", p.ExchangeID, p.ModelPackageID, len(p.Files))
}

// replayFromChain builds a ReplayPlan from the supplied chain.
// The connector's replay engine uses this to reconstruct the
// exact context+model that produced a given span.
func replayFromChain(chain *AttributionChain) *ReplayPlan {
	if chain == nil {
		return nil
	}
	prompts := strings.Join(chain.Prompts, "\n\n")
	return &ReplayPlan{
		ExchangeID:     chain.ExchangeID,
		SessionID:      chain.SessionID,
		Prompt:         prompts,
		ModelPackageID: chain.ModelPackageID,
		EndpointID:     chain.EndpointID,
		Files:          append([]string(nil), chain.Files...),
		ToolClasses:    append([]string(nil), chain.Tools...),
	}
}
