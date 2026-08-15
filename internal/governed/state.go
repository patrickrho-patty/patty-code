// Package governed wires the PAPER governance clients into the
// harness's tool-authorization path. The Controller wraps every
// executor gate with this layer so a PAPER-governed session
// enforces the enterprise workflow + inline security boundaries
// at the same chokepoint the permission policy already owns.
//
// The layer is state-optional: a harness without a PAPER session
// (local development, tests) installs no State and the wrapper is
// a pass-through. A connected session installs the clients the
// relay pushed (tool registry, workflow gates, sandbox policy)
// and the checks fire before the base permission policy runs.
// Governance denial is authoritative — the base policy cannot
// override a governance block, because enterprise policy (change
// freeze, recall, sub-minimum build) outranks local preference.
package governed

import (
	"context"
	"encoding/json"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"patty/internal/approvals"
	"patty/internal/changeboard"
	"patty/internal/sandbox"
	"patty/internal/workflow"
)

// Checker is the harness's gate contract (agent.Gate), repeated
// here so this package has no import cycle with agent.
type Checker interface {
	Check(ctx context.Context, toolName string, args json.RawMessage, readOnly bool) (bool, string, error)
}

// State is the installable governance view: the tool/MCP approval
// registry (C3+C4), the enterprise workflow gates (D1/D3/D5/D6),
// the sandbox-baseline policy (E4), and the session's repo/model
// context those gates evaluate against. Every field is optional;
// a nil client skips its check.
type State struct {
	mu             sync.RWMutex
	registry       *approvals.Registry
	gates          *workflow.GatesClient
	sandbox        *sandbox.PolicyStore
	board          *changeboard.Board
	submissionSink func(*changeboard.Submission)
	dialPolicy     func(host string) bool
	repoID         string
	modelID        string
}

// NewState returns an empty governance state. All checks are
// skipped until the corresponding client is installed.
func NewState() *State { return &State{} }

// SetRegistry installs the tool/MCP/network/secret approval
// registry (C3+C4).
func (s *State) SetRegistry(r *approvals.Registry) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.registry = r
	s.mu.Unlock()
}

// SetGates installs the enterprise workflow gates (D1/D3/D5/D6).
func (s *State) SetGates(g *workflow.GatesClient) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.gates = g
	s.mu.Unlock()
}

// SetSandboxPolicy installs the sandbox-baseline policy (E4).
func (s *State) SetSandboxPolicy(p *sandbox.PolicyStore) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.sandbox = p
	s.mu.Unlock()
}

// SetChangeBoard installs the change-control board (D2 §33.4):
// high-risk AI writes auto-submit and block until approved.
func (s *State) SetChangeBoard(b *changeboard.Board) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.board = b
	s.mu.Unlock()
}

// SetDialPolicy installs the authoritative dial check (E3 air-gap:
// the provider's AirGap().AllowsDial). Applied before network grants.
func (s *State) SetDialPolicy(policy func(host string) bool) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.dialPolicy = policy
	s.mu.Unlock()
}

// SetSubmissionSink installs the wire notifier for NEW pending
// changeboard submissions (D2): the provider turns each into an
// ActionEnvelope the control plane surfaces for review.
func (s *State) SetSubmissionSink(sink func(*changeboard.Submission)) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.submissionSink = sink
	s.mu.Unlock()
}

// SetContext records the session's repository + model identifiers
// the workflow gates evaluate against (freeze scope, recall).
func (s *State) SetContext(repoID, modelID string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.repoID = repoID
	s.modelID = modelID
	s.mu.Unlock()
}

// writeActionTools are the tool names that constitute an AI write
// for the workflow gates + coding standards.
var writeActionTools = map[string]bool{
	"write_file": true,
	"ln":         true, // the harness's write_file implementation surface
	"edit_file":  true,
	"move_file":  true,
}

// shellActionTools are the tool names that execute local shell.
var shellActionTools = map[string]bool{
	"bash":  true,
	"sh":    true,
	"shell": true,
}

// dispatchActionFor maps a tool call to the workflow-gate action
// name. The relay and the harness agree on these stable strings.
// A read-only shell call counts as `read` so a change-freeze
// still permits inspection and test runs (D3 semantics: freeze
// blocks AI writes, not reads/review/tests).
func dispatchActionFor(toolName string, readOnly bool) string {
	switch {
	case writeActionTools[toolName] && !readOnly:
		return "file_write"
	case shellActionTools[toolName]:
		if readOnly {
			return "read"
		}
		return "shell_exec"
	default:
		return "tool_use"
	}
}

// CheckToolCall runs every installed governance check for a tool
// call. It returns blocked=true with a human-readable (Korean where
// the relay supplied Korean text) reason when any gate denies.
// Checks run in precedence order: version/ring/recall/freeze/ack
// (workflow) first — an org-wide block outranks per-tool state —
// then tool approval, then sandbox, then coding standards, then
// network grants.
func (s *State) CheckToolCall(toolName string, args json.RawMessage, readOnly bool) (bool, string) {
	if s == nil {
		return false, ""
	}
	s.mu.RLock()
	registry := s.registry
	gates := s.gates
	sandboxStore := s.sandbox
	sink := s.submissionSink
	dialPolicy := s.dialPolicy
	repoID := s.repoID
	modelID := s.modelID
	s.mu.RUnlock()

	// D1/D3/D5/D6: enterprise workflow gates (freeze, recall,
	// version, acknowledgement).
	if gates != nil {
		dec := gates.CheckDispatch(dispatchActionFor(toolName, readOnly), repoID, modelID)
		if !dec.Allow {
			return true, governanceReason(dec.Reason, dec.ReasonKo)
		}
	}

	// D2: change-control board for high-risk AI writes. Approved
	// submissions pass; anything else auto-submits, notifies the
	// control plane (submission wire sink), and blocks with the Korean
	// surface (§33.4) until a reviewer approves over the directive
	// channel.
	if s.board != nil && writeActionTools[toolName] && !readOnly {
		if path, content := fileWritePayload(args); path != "" {
			if risk := riskClassOf(path, content); risk != "" {
				sub := &changeboard.Submission{
					SubmissionID:  "sub-" + path,
					RepositoryID:  repoID,
					RiskClass:     risk,
					Description:   "auto-submitted: high-risk change class",
					DescriptionKo: "고위험 변경 클래스 자동 제출 — 승인까지 차단됩니다",
					Submitter:     "patty-governed",
				}
				rec, _ := s.board.Submit(sub)
				if rec != nil && rec.IsPending() && sink != nil {
					sink(rec)
				}
				if rec == nil || !rec.IsApproved() {
					if rec != nil && rec.Status == changeboard.StatusRejected {
						return true, "변경 통제 위원회가 이 변경을 거절했습니다 (change-control board rejected)"
					}
					return true, "변경 통제 승인 대기중 — 고위험 변경은 승인 후 진행됩니다 (change-control approval pending)"
				}
			}
		}
	}

	// C3: tool/MCP approval against the relay's registry.
	if registry != nil {
		if dec := registry.CheckTool(toolName, time.Now().UnixMilli()); !dec.Allow {
			return true, governanceReason(dec.Reason, dec.ReasonKo)
		}
	}

	// E4: sandbox-baseline enforcement for local shell execution.
	// The opt-in is NOT automatic: a repo that requires remote
	// execution denies local shell at the gate. An explicit local
	// opt-in is a user action, not something this layer grants.
	if sandboxStore != nil && shellActionTools[toolName] && !readOnly {
		dec := sandboxStore.CheckExecution(repoID, false)
		if !dec.Allow {
			return true, governanceReason(dec.Reason, dec.ReasonKo)
		}
	}

	// D4: coding standards for file writes.
	if gates != nil && writeActionTools[toolName] && !readOnly {
		path, content := fileWritePayload(args)
		if path != "" {
			if std := gates.CheckCodingStandard(path, content); std != nil {
				return true, governanceReason(std.Description, std.DescriptionKo)
			}
		}
	}

	// C4: network-broker grants for outbound dials embedded in
	// shell commands (curl/wget/http URLs).
	if shellActionTools[toolName] {
		for _, host := range hostsInCommand(commandOf(args)) {
			// E3 sovereign air-gap: unconditionally authoritative —
			// not even an approved network grant dials past it.
			if dialPolicy != nil && !dialPolicy(host) {
				return true, "에어갭(air-gap) 모드 — 허용되지 않은 외부 접속입니다: " + host
			}
			if registry != nil {
				if dec := registry.CheckNetwork(host); !dec.Allow {
					return true, governanceReason(dec.Reason, dec.ReasonKo)
				}
			}
		}
	}

	return false, ""
}

// governanceReason prefers the Korean explanation (the Korean
// enterprise differentiator, PRD §33.6/§33.13) and falls back to
// the English reason.
func governanceReason(en, ko string) string {
	if ko != "" {
		return ko
	}
	if en != "" {
		return en
	}
	return "blocked by PCCP governance policy"
}

// commandOf extracts the command string from a bash tool call.
func commandOf(args json.RawMessage) string {
	if len(args) == 0 {
		return ""
	}
	var m map[string]any
	if err := json.Unmarshal(args, &m); err != nil {
		return ""
	}
	if cmd, ok := m["command"].(string); ok {
		return cmd
	}
	return ""
}

// fileWritePayload extracts the path + content from a file-write
// tool call for the coding-standard check.
func fileWritePayload(args json.RawMessage) (path, content string) {
	if len(args) == 0 {
		return "", ""
	}
	var m map[string]any
	if err := json.Unmarshal(args, &m); err != nil {
		return "", ""
	}
	path, _ = m["path"].(string)
	if path == "" {
		path, _ = m["file_path"].(string)
	}
	content, _ = m["content"].(string)
	return path, content
}

// urlPattern matches absolute http(s) URLs embedded in shell
// commands (curl/wget targets, export endpoints).
var urlPattern = regexp.MustCompile(`https?://[A-Za-z0-9.\-_]+(:\d+)?`)

// hostsInCommand extracts the distinct hostnames the command
// dials. Duplicate hosts are returned once.
func hostsInCommand(command string) []string {
	if command == "" {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	for _, raw := range urlPattern.FindAllString(command, -1) {
		u, err := url.Parse(raw)
		if err != nil || u.Hostname() == "" {
			continue
		}
		host := u.Hostname()
		if !seen[host] {
			seen[host] = true
			out = append(out, host)
		}
	}
	return out
}

// TrimHint keeps the deny reason bounded even if a relay pushes a
// very long explanation.
const maxReasonLen = 600

func boundedReason(reason string) string {
	if len(reason) <= maxReasonLen {
		return reason
	}
	return reason[:maxReasonLen] + "…"
}

// riskPatterns maps high-risk change classes (D2 §33.4) to path /
// content signals. A dependency-manifest change is high-risk by
// definition; crypto/payment/PII paths match by name and content.
var riskPatterns = []struct {
	class changeboard.RiskClass
	re    *regexp.Regexp
}{
	{changeboard.RiskCrypto, regexp.MustCompile(`(?i)(crypto|cipher|signature|ed25519|rsa|secp256|kms|keymgmt)`)},
	{changeboard.RiskHigh, regexp.MustCompile(`(?i)(payment|billing|checkout|stripe|tosspayments|pg|evidence-receipt|secret|credential|pii|rrn|resident)`)},
}

// depManifestRe identifies dependency manifests: any change there is
// a new dependency (MEDIUM at minimum).
var depManifestRe = regexp.MustCompile(`(^|/)(go\.(mod|sum)|package\.json|pnpm-lock\.yaml|yarn\.lock|Cargo\.(toml|lock)|pyproject\.toml|requirements\.txt|Gemfile|pom\.xml)$`)

// riskClassOf classifies a file write for the change board. Empty
// means the change is not board-scope.
func riskClassOf(path, content string) changeboard.RiskClass {
	if depManifestRe.MatchString(path) {
		return changeboard.RiskMedium
	}
	_ = content
	for _, rp := range riskPatterns {
		if rp.re.MatchString(path) || (len(content) < 1<<20 && rp.re.MatchString(content)) {
			return rp.class
		}
	}
	if strings.HasPrefix(path, "mcp") || strings.Contains(path, "/mcp/") || strings.Contains(path, "mcp.json") {
		return changeboard.RiskHigh
	}
	return ""
}
