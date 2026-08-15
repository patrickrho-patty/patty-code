package dari

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"patty/internal/admin"
	"patty/internal/changeboard"
)

// changeboard_admin.go closes the D2/E5 loop on the connector:
// - high-risk write submissions surface to the control plane as
//   ActionEnvelopes (the reviewer queue is REAL data);
// - a verified admin directive changeboard.approve/.reject mutates
//   the live, durable board — the next write passes or stays blocked;
// - pause_session/resume_session/suspend_model directives execute
//   against the provider's session state.

// governanceSubmissionPayload is the ActionEnvelope payload for a
// changeboard submission (the CP renders the review queue from it).
type governanceSubmissionPayload struct {
	SubmissionID  string `json:"submission_id"`
	RepositoryID  string `json:"repository_id,omitempty"`
	RiskClass     string `json:"risk_class"`
	Description   string `json:"description"`
	DescriptionKo string `json:"description_ko"`
	Submitter     string `json:"submitter"`
	Status        string `json:"status"`
}

// EmitChangeboardSubmission turns a pending submission into a
// governed action envelope on the session's provenance emitter (it
// rides the next flush to the relay).
func (p *Provider) EmitChangeboardSubmission(sub *changeboard.Submission) {
	if sub == nil {
		return
	}
	payload, _ := json.Marshal(governanceSubmissionPayload{
		SubmissionID:  sub.SubmissionID,
		RepositoryID:  sub.RepositoryID,
		RiskClass:     string(sub.RiskClass),
		Description:   sub.Description,
		DescriptionKo: sub.DescriptionKo,
		Submitter:     sub.Submitter,
		Status:        string(sub.Status),
	})
	p.mu.Lock()
	sess, model, harness := p.sessionID, p.model, p.harnessID
	p.mu.Unlock()
	env := &provenanceActionDraft{
		ActionID:       "act-sub-" + sub.SubmissionID,
		OrganizationID: p.orgIDForSession(),
		SessionID:      sess,
		UserID:         p.userID,
		HarnessID:      harness,
		ModelPackageID: model,
		ActionType:     "changeboard.submit",
		ActionPayload:  string(payload),
	}
	p.emitActionDraft(env)
}

// provenanceActionDraft decouples this file from provenancewire's
// builder surface (the emit method lives on the provider).
type provenanceActionDraft struct {
	ActionID       string
	OrganizationID string
	SessionID      string
	UserID         string
	HarnessID      string
	ModelPackageID string
	ActionType     string
	ActionPayload  string
}

// directiveExecutor executes VERIFIED admin directives (E5). Failures
// are recorded and returned — a directive that verifies but cannot
// execute is an incident, not a silent no-op.
type directiveExecutor struct {
	p *Provider
}

func (e directiveExecutor) Execute(cmd *admin.Command) error {
	payload := map[string]any{}
	_ = json.Unmarshal(cmd.Payload, &payload)
	subID, _ := payload["submission_id"].(string)

	switch strings.ToLower(string(cmd.CommandType)) {
	case "changeboard.approve":
		board := e.p.ChangeBoard()
		if err := board.Approve(subID, cmd.IssuedBy, "directive", e.p.nowMs()); err != nil {
			return fmt.Errorf("changeboard approve %s: %w", subID, err)
		}
		slog.Info("dari: changeboard submission APPROVED by directive", "submission", subID, "by", cmd.IssuedBy)
	case "changeboard.reject":
		board := e.p.ChangeBoard()
		if err := board.Reject(subID, cmd.IssuedBy, cmd.Reason, e.p.nowMs()); err != nil {
			return fmt.Errorf("changeboard reject %s: %w", subID, err)
		}
		slog.Info("dari: changeboard submission REJECTED by directive", "submission", subID, "by", cmd.IssuedBy)
	case "pause_session":
		e.p.SetSuspended(true)
		slog.Warn("dari: session PAUSED by admin directive", "reason", cmd.Reason)
	case "resume_session":
		e.p.SetSuspended(false)
		slog.Info("dari: session resumed by admin directive")
	case "suspend_model":
		e.p.SetSuspended(true)
		slog.Warn("dari: model suspended by admin directive", "target", cmd.Target, "reason", cmd.Reason)
	default:
		// Unknown directive types still VERIFY + record; execution is
		// explicit — refusing unknown commands is fail-closed, and the
		// dispatcher records the attempt either way.
		return fmt.Errorf("dari: admin directive %s has no executor (recorded, not executed)", cmd.CommandType)
	}
	return nil
}
