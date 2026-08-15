package dariproto

import (
	"time"

	"patty/internal/approvals"
	"patty/internal/sandbox"
	"patty/internal/workflow"
)

// governed_convert.go converts a decoded GovernanceStateWire into the
// connector's governed.State clients. This is the bridge that makes the
// relay-pushed gates fire on real tool calls (C3/C4/D1/D3-D6/E4).

// HarnessIdentity supplies the connector's local version/ring for the
// gates client (D5 version/ring checks compare against these).
type HarnessIdentity struct {
	Version string
	Ring    string
}

// BuildGatesClient converts the snapshot into a workflow.GatesClient
// with freeze/recalls/acks/version/standards installed.
func (s *GovernanceStateWire) BuildGatesClient(id HarnessIdentity) *workflow.GatesClient {
	gates := workflow.NewGatesClient(s.OrgID, id.Version, id.Ring)
	if s.Freeze != nil {
		gates.SetFreeze(&workflow.ChangeFreeze{
			OrganizationID: s.OrgID,
			Reason:         s.Freeze.Reason,
			ReasonKo:       s.Freeze.ReasonKo,
			AffectedRepos:  s.Freeze.AffectedRepos,
			AllowedActions: s.Freeze.AllowedActions,
			InitiatedBy:    "relay",
			StartedAt:      time.Now(),
			NotAfter:       time.UnixMilli(s.Freeze.NotAfterMs),
		})
	}
	if len(s.Recalls) > 0 {
		recalls := make([]workflow.ModelRecall, 0, len(s.Recalls))
		for _, r := range s.Recalls {
			recalls = append(recalls, workflow.ModelRecall{
				OrganizationID: s.OrgID,
				RecalledModel:  r.Model,
				Replacement:    r.Replacement,
				Reason:         r.Reason,
			})
		}
		gates.SetRecalls(recalls)
	}
	if len(s.Acks) > 0 {
		acks := make([]workflow.AcknowledgementRequirement, 0, len(s.Acks))
		for _, a := range s.Acks {
			acks = append(acks, workflow.AcknowledgementRequirement{
				OrganizationID: s.OrgID,
				PolicyEpochID:  a.PolicyEpochID,
				SummaryKo:      a.SummaryKo,
				Blocking:       a.Blocking,
			})
		}
		gates.SetAcknowledgements(acks)
	}
	if s.VersionReq != nil {
		gates.SetVersionRequirement(&workflow.VersionRequirement{
			OrganizationID: s.OrgID,
			MinVersion:     s.VersionReq.MinVersion,
			ReleaseRing:    s.VersionReq.Ring,
			CurrentVersion: id.Version,
			CurrentRing:    id.Ring,
		})
	}
	if len(s.Standards) > 0 {
		standards := make([]workflow.CodingStandard, 0, len(s.Standards))
		for _, st := range s.Standards {
			standards = append(standards, workflow.CodingStandard{
				OrganizationID: s.OrgID,
				RuleID:         st.RuleID,
				Severity:       workflow.SeverityBlock,
				Description:    st.Description,
				DescriptionKo:  st.DescriptionKo,
				BlockPattern:   st.BlockPattern,
			})
		}
		gates.SetStandards(standards)
	}
	return gates
}

// BuildApprovalsRegistry converts the snapshot's tool statuses into an
// approvals.Registry. Unknown status strings fail closed at check time.
func (s *GovernanceStateWire) BuildApprovalsRegistry() *approvals.Registry {
	reg := approvals.NewRegistry()
	if len(s.Tools) == 0 {
		return reg
	}
	tools := make([]*approvals.ToolRegistration, 0, len(s.Tools))
	now := time.Now().UnixMilli()
	for _, t := range s.Tools {
		tools = append(tools, &approvals.ToolRegistration{
			ToolID:           t.ToolID,
			Status:           approvals.ApprovalStatus(t.Status),
			ApprovedAtUnixMs: now,
		})
	}
	reg.SetTools(tools, now)
	return reg
}

// BuildSandboxStore converts the snapshot's sandbox policies into a
// sandbox.PolicyStore.
func (s *GovernanceStateWire) BuildSandboxStore() *sandbox.PolicyStore {
	store := sandbox.NewPolicyStore()
	for _, sb := range s.Sandboxes {
		store.Set(&sandbox.Policy{
			OrganizationID: s.OrgID,
			RepositoryID:   sb.RepositoryID,
			Mode:           sandbox.SandboxMode(sb.Mode),
			RiskClass:      sandbox.RiskClass(sb.RiskClass),
			IssuedAtUnixMs: time.Now().UnixMilli(),
		})
	}
	return store
}
