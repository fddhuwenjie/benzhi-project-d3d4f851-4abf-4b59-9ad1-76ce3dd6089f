package domain

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"time"
)

type CanonicalProject struct {
	ProjectID              string              `json:"project_id"`
	BuildingName           string              `json:"building_name"`
	Zones                  []string            `json:"zones"`
	Survey                 SurveyGraph         `json:"survey"`
	Signs                  []SignProposal      `json:"signs"`
	SignsRevision          int                 `json:"signs_revision"`
	ValidatedSignsRevision int                 `json:"validated_signs_revision"`
	Issues                 []ValidationIssue   `json:"issues"`
	ValidatedAt            *time.Time          `json:"validated_at"`
	Walkthroughs           []WalkthroughReview `json:"walkthroughs"`
	ApprovedBy             string              `json:"approved_by"`
}

type ApprovalPreflight struct {
	SourceRevision    int      `json:"source_revision"`
	PathCount         int      `json:"path_count"`
	SignCount         int      `json:"sign_count"`
	DispositionCount  int      `json:"disposition_count"`
	WalkthroughRounds int      `json:"walkthrough_rounds"`
	ApproverID        string   `json:"approver_id"`
	CandidateDigest   string   `json:"candidate_digest"`
	Blockers          []string `json:"blockers"`
}

type VerificationItem struct {
	Passed   bool     `json:"passed"`
	Failures []string `json:"failures,omitempty"`
}

type PackageVerification struct {
	CheckedAt      time.Time        `json:"checked_at"`
	PackageID      string           `json:"package_id"`
	ProjectID      string           `json:"project_id"`
	SourceRevision int              `json:"source_revision"`
	SHA256Digest   string           `json:"sha256_digest"`
	Summary        VerificationItem `json:"summary"`
	Structure      VerificationItem `json:"structure"`
	Business       VerificationItem `json:"business"`
	EventChain     VerificationItem `json:"event_chain"`
}

func CanonicalPayload(p *LayoutProject, approver string) ([]byte, string) {
	cp := CanonicalProject{ProjectID: p.ProjectID, BuildingName: p.BuildingName, Zones: append([]string{}, p.Zones...), Survey: NormalizeGraph(p.Survey), Signs: NormalizeSigns(p.Signs), SignsRevision: p.SignsRevision, ValidatedSignsRevision: p.ValidatedSignsRevision, Issues: append([]ValidationIssue{}, p.Issues...), ValidatedAt: p.LastValidatedAt, Walkthroughs: append([]WalkthroughReview{}, p.Walkthroughs...), ApprovedBy: approver}
	cp.Survey.BaselineDigest = p.Survey.BaselineDigest
	sort.Strings(cp.Zones)
	sort.Slice(cp.Issues, func(i, j int) bool { return cp.Issues[i].IssueID < cp.Issues[j].IssueID })
	sort.Slice(cp.Walkthroughs, func(i, j int) bool { return cp.Walkthroughs[i].Round < cp.Walkthroughs[j].Round })
	for i := range cp.Walkthroughs {
		cp.Walkthroughs[i].SignedAt = nil
	}
	payload, _ := json.Marshal(cp)
	payload = bytes.TrimSpace(payload)
	return payload, Digest(json.RawMessage(payload))
}

func (p *LayoutProject) PreflightApproval(approver string) ApprovalPreflight {
	pre := ApprovalPreflight{SourceRevision: p.Revision, PathCount: len(p.Survey.Edges), SignCount: len(p.Signs), WalkthroughRounds: len(p.Walkthroughs), ApproverID: approver}
	for _, issue := range p.Issues {
		pre.DispositionCount += len(issue.EvidenceHistory)
	}
	if p.Status != StatusReadyForApproval {
		pre.Blockers = append(pre.Blockers, "项目尚未进入待批准状态")
	}
	for _, issue := range p.Issues {
		if issue.Status != IssueResolved {
			pre.Blockers = append(pre.Blockers, fmt.Sprintf("问题 %s 尚未关闭", issue.IssueID))
		}
	}
	if len(p.Walkthroughs) == 0 || p.Walkthroughs[len(p.Walkthroughs)-1].Decision != "pass" {
		pre.Blockers = append(pre.Blockers, "最近一轮走查未通过")
	}
	if approver == "" || approver != p.ReviewerID {
		pre.Blockers = append(pre.Blockers, "签署人不是指定复核员")
	}
	if approver == p.DesignerID {
		pre.Blockers = append(pre.Blockers, "签署人不得与设计员相同")
	}
	_, pre.CandidateDigest = CanonicalPayload(p, approver)
	sort.Strings(pre.Blockers)
	return pre
}

func (p *LayoutProject) FreezePackage(approver string) (InstallationPackage, error) {
	pre := p.PreflightApproval(approver)
	if len(pre.Blockers) > 0 {
		return InstallationPackage{}, fmt.Errorf("批准预检未通过: %s", pre.Blockers[0])
	}
	if err := p.SignApproval(approver); err != nil {
		return InstallationPackage{}, err
	}
	payload, digest := CanonicalPayload(p, approver)
	pkg := InstallationPackage{PackageID: "pkg-" + digest[:20], ProjectID: p.ProjectID, SourceRevision: p.Revision, CanonicalPayload: payload, SHA256Digest: digest, ApprovedBy: approver, FrozenAt: time.Now().UTC()}
	p.Package = &pkg
	_ = p.Transition(StatusFrozen)
	return pkg, nil
}

func VerifyPackage(pkg InstallationPackage) bool {
	if len(pkg.SHA256Digest) != 64 || len(pkg.CanonicalPayload) == 0 {
		return false
	}
	identityValid := pkg.PackageID == "" || pkg.PackageID == "pkg-"+pkg.SHA256Digest[:20]
	return pkg.SHA256Digest == Digest(json.RawMessage(bytes.TrimSpace(pkg.CanonicalPayload))) && pkg.ProjectID != "" && identityValid
}

func ParseCanonicalPayload(payload []byte) (CanonicalProject, error) {
	var cp CanonicalProject
	dec := json.NewDecoder(bytes.NewReader(payload))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cp); err != nil {
		return cp, err
	}
	return cp, nil
}

func VerifyPackageContent(pkg InstallationPackage, project *LayoutProject) (VerificationItem, VerificationItem) {
	structure, business := VerificationItem{Passed: true}, VerificationItem{Passed: true}
	cp, err := ParseCanonicalPayload(pkg.CanonicalPayload)
	if err != nil {
		structure.Passed = false
		structure.Failures = append(structure.Failures, "规范载荷 JSON 无效: "+err.Error())
		business.Passed = false
		business.Failures = append(business.Failures, "无法解析规范载荷")
		return structure, business
	}
	baseline := PreflightBaseline(cp.Survey)
	if len(baseline.Blockers) > 0 {
		structure.Passed = false
		for _, b := range baseline.Blockers {
			structure.Failures = append(structure.Failures, b.Message)
		}
	}
	signs := PreflightSigns(cp.Survey, nil, cp.Signs)
	if len(signs.Errors) > 0 {
		structure.Passed = false
		for _, e := range signs.Errors {
			structure.Failures = append(structure.Failures, e.Message)
		}
	}
	for _, issue := range cp.Issues {
		if issue.Status != IssueResolved {
			structure.Passed = false
			structure.Failures = append(structure.Failures, "规范载荷包含未关闭问题 "+issue.IssueID)
		}
		if issue.Status == IssueResolved && len(issue.EvidenceHistory) == 0 {
			structure.Passed = false
			structure.Failures = append(structure.Failures, "问题 "+issue.IssueID+" 缺少处置版本")
		} else if len(issue.EvidenceHistory) > 0 {
			latest := issue.EvidenceHistory[len(issue.EvidenceHistory)-1]
			if !latest.Valid || latest.ClosedAt == nil || latest.Resolution == "" || latest.EvidenceDigest == "" {
				structure.Passed = false
				structure.Failures = append(structure.Failures, "问题 "+issue.IssueID+" 的处置版本不完整")
			}
		}
	}
	if len(cp.Walkthroughs) == 0 || cp.Walkthroughs[len(cp.Walkthroughs)-1].Decision != "pass" {
		structure.Passed = false
		structure.Failures = append(structure.Failures, "规范载荷缺少通过走查")
	}
	if cp.ApprovedBy == "" {
		structure.Passed = false
		structure.Failures = append(structure.Failures, "规范载荷缺少签署人")
	}
	if pkg.ProjectID != project.ProjectID {
		business.Passed = false
		business.Failures = append(business.Failures, "project_id 与项目终态不一致")
	}
	if cp.ProjectID != project.ProjectID {
		business.Passed = false
		business.Failures = append(business.Failures, "载荷 project_id 与项目终态不一致")
	}
	if pkg.SourceRevision != project.Revision-1 {
		business.Passed = false
		business.Failures = append(business.Failures, "source_revision 与项目终态不一致")
	}
	if pkg.ApprovedBy != project.ReviewerID || cp.ApprovedBy != pkg.ApprovedBy {
		business.Passed = false
		business.Failures = append(business.Failures, "approved_by 与项目终态不一致")
	}
	if project.Package == nil || project.Package.PackageID != pkg.PackageID {
		business.Passed = false
		business.Failures = append(business.Failures, "package_id 与项目终态不一致")
	}
	if project.Package != nil && !project.Package.FrozenAt.Equal(pkg.FrozenAt) {
		business.Passed = false
		business.Failures = append(business.Failures, "frozen_at 与项目终态不一致")
	}
	_, expectedDigest := CanonicalPayload(project, pkg.ApprovedBy)
	if expectedDigest != pkg.SHA256Digest {
		business.Passed = false
		business.Failures = append(business.Failures, "规范载荷与项目终态内容不一致")
	}
	if project.Status != StatusFrozen {
		business.Passed = false
		business.Failures = append(business.Failures, "项目终态不是 frozen")
	}
	return structure, business
}

func (p *LayoutProject) Summary() string {
	return fmt.Sprintf("%s 状态=%s 修订=%d 问题=%d", p.BuildingName, p.Status, p.Revision, len(p.Issues))
}
