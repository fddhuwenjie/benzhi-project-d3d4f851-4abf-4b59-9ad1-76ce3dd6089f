package domain

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

func (p *LayoutProject) BuildRoute() []Checkpoint {
	g := NormalizeGraph(p.Survey)
	order := make([]string, 0, len(g.Nodes))
	for _, n := range g.Nodes {
		order = append(order, n.ID)
	}
	sort.Strings(order)
	route := make([]Checkpoint, 0, len(order))
	for _, id := range order {
		route = append(route, Checkpoint{ID: Digest([]string{p.ProjectID, id})[:12], NodeID: id})
	}
	return route
}

func (p *LayoutProject) ExpectedWalkthroughRoute() ([]Checkpoint, bool) {
	full := p.BuildRoute()
	if len(p.Walkthroughs) == 0 || p.Walkthroughs[len(p.Walkthroughs)-1].Decision != "fail" {
		return full, false
	}
	failed := map[string]bool{}
	for _, id := range p.Walkthroughs[len(p.Walkthroughs)-1].FailedCheckpointIDs {
		failed[id] = true
	}
	selected := make([]Checkpoint, 0, len(failed))
	for _, checkpoint := range full {
		if failed[checkpoint.ID] {
			selected = append(selected, checkpoint)
		}
	}
	return selected, true
}

func (p *LayoutProject) MarkWalkthroughRemediation(changedNodes, changedDestinations map[string]bool) {
	if len(p.Walkthroughs) == 0 {
		return
	}
	last := p.Walkthroughs[len(p.Walkthroughs)-1]
	if last.Decision != "fail" {
		return
	}
	failed := map[string]bool{}
	for _, id := range last.FailedCheckpointIDs {
		failed[id] = true
	}
	done := map[string]bool{}
	for _, id := range p.WalkthroughRemediatedIDs {
		done[id] = true
	}
	for _, checkpoint := range last.Checkpoints {
		if failed[checkpoint.ID] && (changedNodes[checkpoint.NodeID] || changedDestinations[checkpoint.NodeID]) {
			done[checkpoint.ID] = true
		}
	}
	p.WalkthroughRemediatedIDs = p.WalkthroughRemediatedIDs[:0]
	for id := range done {
		p.WalkthroughRemediatedIDs = append(p.WalkthroughRemediatedIDs, id)
	}
	sort.Strings(p.WalkthroughRemediatedIDs)
}

func (p *LayoutProject) TargetedRemediationComplete() bool {
	if len(p.Walkthroughs) == 0 {
		return false
	}
	last := p.Walkthroughs[len(p.Walkthroughs)-1]
	if last.Decision != "fail" {
		return true
	}
	done := map[string]bool{}
	for _, id := range p.WalkthroughRemediatedIDs {
		done[id] = true
	}
	for _, id := range last.FailedCheckpointIDs {
		if !done[id] {
			return false
		}
	}
	return len(last.FailedCheckpointIDs) > 0
}

func (p *LayoutProject) RecordWalkthrough(reviewer string, checks []Checkpoint) (WalkthroughReview, error) {
	if reviewer == p.DesignerID || reviewer == "" || reviewer != p.ReviewerID {
		return WalkthroughReview{}, fmt.Errorf("复核员身份无效")
	}
	if p.Status != StatusReadyForWalkthrough {
		return WalkthroughReview{}, fmt.Errorf("当前状态不能走查")
	}
	failed := []string{}
	for i := range checks {
		c := &checks[i]
		c.FailedDimensions = nil
		if !c.Visible {
			c.FailedDimensions = append(c.FailedDimensions, "visibility")
		}
		if !c.DirectionCorrect {
			c.FailedDimensions = append(c.FailedDimensions, "direction")
		}
		if !c.Visible || !c.DirectionCorrect {
			if strings.TrimSpace(c.Note) == "" {
				return WalkthroughReview{}, fmt.Errorf("失败站点 %s 必须填写原因", c.ID)
			}
			c.Note = strings.TrimSpace(c.Note)
			failed = append(failed, c.ID)
		}
	}
	expected, targeted := p.ExpectedWalkthroughRoute()
	_ = expected
	parent := ""
	if targeted {
		parent = p.Walkthroughs[len(p.Walkthroughs)-1].ReviewID
	}
	r := WalkthroughReview{ReviewID: Digest([]string{p.ProjectID, reviewer, fmt.Sprint(len(p.Walkthroughs))})[:16], ReviewerID: reviewer, RouteSeed: p.Survey.BaselineDigest, Checkpoints: checks, FailedCheckpointIDs: failed, Decision: "pass", ParentReviewID: parent, Targeted: targeted, Round: len(p.Walkthroughs) + 1, BaselineDigest: p.Survey.BaselineDigest, SignRevision: p.SignsRevision}
	if len(failed) > 0 {
		r.Decision = "fail"
	}
	p.Walkthroughs = append(p.Walkthroughs, r)
	if len(failed) > 0 {
		p.WalkthroughRemediatedIDs = nil
		_ = p.Transition(StatusWalkthroughFailed)
	} else {
		_ = p.Transition(StatusReadyForApproval)
	}
	return r, nil
}
func (p *LayoutProject) SignApproval(reviewer string) error {
	if p.Status != StatusReadyForApproval {
		return fmt.Errorf("规则或走查尚未通过")
	}
	if reviewer == p.DesignerID || reviewer != p.ReviewerID {
		return fmt.Errorf("只有指定的独立复核员可签署")
	}
	if p.LastValidatedAt == nil {
		return fmt.Errorf("尚未执行规则校验")
	}
	for _, i := range p.Issues {
		if i.Status != IssueResolved {
			return fmt.Errorf("仍有未关闭问题")
		}
	}
	if len(p.Walkthroughs) > 0 && p.Walkthroughs[len(p.Walkthroughs)-1].Decision == "pass" {
		t := time.Now().UTC()
		p.Walkthroughs[len(p.Walkthroughs)-1].SignedAt = &t
		return nil
	}
	return fmt.Errorf("缺少通过的走查记录")
}
