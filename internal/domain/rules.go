package domain

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	RuleContinuity              = "CONTINUITY"
	RuleDecisionCoverage        = "DECISION_COVERAGE"
	RuleAccessibleConsistency   = "ACCESSIBLE_CONSISTENCY"
	RuleDuplicateContent        = "DUPLICATE_CONTENT"
	RuleDestinationReachability = "DESTINATION_REACHABILITY"
)

func (p *LayoutProject) ValidateRules() []ValidationIssue {
	g := NormalizeGraph(p.Survey)
	nodes := map[string]Node{}
	for _, n := range g.Nodes {
		nodes[n.ID] = n
	}
	issues := make([]ValidationIssue, 0)
	add := func(code, node, sev string) {
		id := Digest([]string{p.ProjectID, code, node})
		issues = append(issues, ValidationIssue{IssueID: id[:16], RuleCode: code, NodeID: node, Severity: sev, Status: IssueOpen})
	}
	adj := map[string][]string{}
	for _, e := range g.Edges {
		adj[e.From] = append(adj[e.From], e.To)
	}
	for k := range adj {
		sort.Strings(adj[k])
	}
	for _, from := range g.EntranceIDs {
		for _, to := range adj[from] {
			if !hasSignAt(p.Signs, to) {
				add(RuleContinuity, to, "error")
			}
		}
	}
	for _, n := range g.Nodes {
		if len(adj[n.ID]) > 1 && !hasSignAt(p.Signs, n.ID) {
			add(RuleDecisionCoverage, n.ID, "error")
		}
	}
	for _, e := range g.Edges {
		if e.Accessible && !edgeAccessibleFlag(g, e) {
			add(RuleAccessibleConsistency, e.From, "error")
		}
	}
	seen := map[string]string{}
	for _, s := range p.Signs {
		key := strings.ToLower(strings.TrimSpace(s.DisplayText))
		if key != "" {
			if prev, ok := seen[key]; ok {
				add(RuleDuplicateContent, s.NodeID, "warning")
				_ = prev
			} else {
				seen[key] = s.SignID
			}
		}
	}
	reachable := Reachable(g)
	for _, id := range g.DestinationIDs {
		if !reachable[id] {
			add(RuleDestinationReachability, id, "error")
		}
	}
	sort.Slice(issues, func(i, j int) bool { return issues[i].IssueID < issues[j].IssueID })
	for i := range issues {
		issues[i].Impact = AnalyzeIssueImpact(g, issues[i])
	}
	return issues
}

func hasSignAt(signs []SignProposal, node string) bool {
	for _, s := range signs {
		if s.NodeID == node {
			return true
		}
	}
	return false
}
func edgeAccessibleFlag(g SurveyGraph, e Edge) bool {
	key := e.From + "->" + e.To
	if v, ok := g.AccessibleEdgeFlags[key]; ok {
		return v
	}
	return e.Accessible
}

func Reachable(g SurveyGraph) map[string]bool {
	adj := map[string][]string{}
	for _, e := range g.Edges {
		adj[e.From] = append(adj[e.From], e.To)
	}
	seen := map[string]bool{}
	q := append([]string{}, g.EntranceIDs...)
	for len(q) > 0 {
		n := q[0]
		q = q[1:]
		if seen[n] {
			continue
		}
		seen[n] = true
		q = append(q, adj[n]...)
	}
	return seen
}

func (p *LayoutProject) ApplyResolution(issueID, resolution, evidence string) error {
	return p.AppendResolution(issueID, resolution, evidence, "", time.Now().UTC(), p.Revision)
}

func (p *LayoutProject) AppendResolution(issueID, resolution, evidence, actor string, at time.Time, signRevision int) error {
	for i := range p.Issues {
		if p.Issues[i].IssueID == issueID {
			if strings.TrimSpace(resolution) == "" || strings.TrimSpace(evidence) == "" {
				return fmt.Errorf("整改结论和证据不能为空")
			}
			p.Issues[i].Resolution = resolution
			p.Issues[i].EvidenceDigest = Digest(evidence)
			p.Issues[i].Status = IssueOpen
			p.Issues[i].VerifiedAt = nil
			p.Issues[i].ReverifyReasons = []string{"等待规则复验"}
			p.Issues[i].EvidenceHistory = append(p.Issues[i].EvidenceHistory, EvidenceVersion{
				Version: len(p.Issues[i].EvidenceHistory) + 1, Resolution: strings.TrimSpace(resolution), Evidence: strings.TrimSpace(evidence),
				EvidenceDigest: Digest(strings.TrimSpace(evidence)), SubmittedBy: actor, SubmittedAt: at, SignRevision: signRevision, Valid: true,
			})
			return nil
		}
	}
	return fmt.Errorf("问题不存在")
}

func (p *LayoutProject) InvalidateEvidence(changedNodes, changedDestinations map[string]bool, reason string) {
	for i := range p.Issues {
		issue := &p.Issues[i]
		affected := changedNodes[issue.NodeID]
		if !affected {
			for _, id := range issue.Impact.DestinationIDs {
				if changedDestinations[id] {
					affected = true
					break
				}
			}
		}
		if !affected || len(issue.EvidenceHistory) == 0 {
			continue
		}
		latest := &issue.EvidenceHistory[len(issue.EvidenceHistory)-1]
		if latest.Valid {
			latest.Valid = false
			latest.InvalidReason = reason
			issue.Status = IssueOpen
			issue.VerifiedAt = nil
			issue.ReverifyReasons = []string{"整改证据待重新确认"}
		}
	}
}

func (p *LayoutProject) ReverifyIssues() {
	fresh := p.ValidateRules()
	freshByKey := map[string]ValidationIssue{}
	for _, n := range fresh {
		freshByKey[n.RuleCode+"\x00"+n.NodeID] = n
	}
	for i := range p.Issues {
		old := &p.Issues[i]
		wasResolved := old.Status == IssueResolved
		previousVerifiedAt := old.VerifiedAt
		old.ReverifyReasons = nil
		eligible := old.Resolution != "" && old.EvidenceDigest != ""
		if len(old.EvidenceHistory) > 0 {
			eligible = old.EvidenceHistory[len(old.EvidenceHistory)-1].Valid && strings.TrimSpace(old.EvidenceHistory[len(old.EvidenceHistory)-1].Resolution) != ""
		}
		p.Issues[i].Status = IssueOpen
		_, stillFailing := freshByKey[old.RuleCode+"\x00"+old.NodeID]
		if stillFailing {
			old.ReverifyReasons = append(old.ReverifyReasons, "规则仍未满足")
		}
		if old.Resolution == "" {
			old.ReverifyReasons = append(old.ReverifyReasons, "处置结论不完整")
		}
		if !eligible {
			old.ReverifyReasons = append(old.ReverifyReasons, "缺少当前有效的整改证据")
		}
		if eligible && !stillFailing {
			p.Issues[i].Status = IssueResolved
		}
		if p.Issues[i].Status == IssueResolved {
			if wasResolved && previousVerifiedAt != nil {
				p.Issues[i].VerifiedAt = previousVerifiedAt
			} else {
				p.Issues[i].VerifiedAt = nowPtr()
			}
			if len(old.EvidenceHistory) > 0 {
				latest := &old.EvidenceHistory[len(old.EvidenceHistory)-1]
				if latest.ClosedAt == nil {
					latest.ClosedAt = old.VerifiedAt
				}
			}
		} else {
			old.VerifiedAt = nil
		}
	}
	for _, n := range fresh {
		found := false
		for _, o := range p.Issues {
			if o.RuleCode == n.RuleCode && o.NodeID == n.NodeID {
				found = true
			}
		}
		if !found {
			p.Issues = append(p.Issues, n)
		}
	}
	sort.Slice(p.Issues, func(i, j int) bool { return p.Issues[i].IssueID < p.Issues[j].IssueID })
}

func MergeValidationIssues(current, fresh []ValidationIssue) []ValidationIssue {
	oldByKey := map[string]ValidationIssue{}
	for _, issue := range current {
		oldByKey[issue.RuleCode+"\x00"+issue.NodeID] = issue
	}
	out, present := make([]ValidationIssue, 0, len(current)+len(fresh)), map[string]bool{}
	for _, issue := range fresh {
		key := issue.RuleCode + "\x00" + issue.NodeID
		present[key] = true
		if old, ok := oldByKey[key]; ok {
			issue.Resolution = old.Resolution
			issue.EvidenceDigest = old.EvidenceDigest
			issue.EvidenceHistory = old.EvidenceHistory
			issue.Status = IssueOpen
			issue.VerifiedAt = nil
			issue.ReverifyReasons = []string{"规则再次触发"}
		}
		out = append(out, issue)
	}
	for _, old := range current {
		key := old.RuleCode + "\x00" + old.NodeID
		if !present[key] {
			out = append(out, old)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].IssueID < out[j].IssueID })
	return out
}
func nowPtr() *time.Time { t := time.Now().UTC(); return &t }
