package application

import (
	"context"
	"errors"
	"fmt"
	"wayfinding-release-gate/internal/domain"
)

func (s *Service) ResolveIssue(ctx context.Context, c ResolveIssueCommand) (MutationResult, error) {
	c.UpdatedSigns = domain.NormalizeSigns(c.UpdatedSigns)
	return s.execute(ctx, c.Meta, c, "issue.resolved", func(p *domain.LayoutProject) (MutationResult, error) {
		if p.Status != domain.StatusNeedsRemediation && p.Status != domain.StatusWalkthroughFailed {
			return MutationResult{}, errors.New("当前状态无需整改")
		}
		if len(c.UpdatedSigns) > 0 {
			preflight := domain.PreflightSigns(p.Survey, p.Signs, normalizeLegacySigns(p.Survey, c.UpdatedSigns))
			if len(preflight.Errors) > 0 {
				return MutationResult{}, errors.New(preflight.Errors[0].Message)
			}
			nodes, destinations := domain.ChangedSignScope(p.Signs, preflight.Signs)
			if len(preflight.Diff.Added)+len(preflight.Diff.Modified)+len(preflight.Diff.Deleted) > 0 {
				p.SignsRevision++
				p.InvalidateEvidence(nodes, destinations, "整改提交修改了相关标识方案")
				p.MarkWalkthroughRemediation(nodes, destinations)
			}
			p.Signs = preflight.Signs
		}
		if err := p.AppendResolution(c.IssueID, c.Resolution, c.Evidence, c.Meta.ActorID, s.now(), p.SignsRevision); err != nil {
			return MutationResult{}, err
		}
		return MutationResult{Message: "整改证据已记录", Issues: p.Issues}, nil
	})
}

func (s *Service) Reverify(ctx context.Context, c ReverifyCommand) (MutationResult, error) {
	return s.execute(ctx, c.Meta, c, "issues.reverified", func(p *domain.LayoutProject) (MutationResult, error) {
		if p.Status != domain.StatusNeedsRemediation && p.Status != domain.StatusWalkthroughFailed {
			return MutationResult{}, errors.New("当前状态不能复验")
		}
		if p.Status == domain.StatusWalkthroughFailed && !p.TargetedRemediationComplete() {
			return MutationResult{}, errors.New("走查失败后必须先完成关联标识整改")
		}
		p.ReverifyIssues()
		p.ValidatedSignsRevision = p.SignsRevision
		_, p.IssueSummary = domain.QueryIssues(p.Issues, domain.IssueFilter{})
		for _, i := range p.Issues {
			if i.Status == domain.IssueOpen {
				return MutationResult{Message: "复验仍有未解决问题", Issues: p.Issues}, nil
			}
		}
		if err := p.Transition(domain.StatusReadyForWalkthrough); err != nil {
			return MutationResult{}, err
		}
		p.WalkthroughRoute, _ = p.ExpectedWalkthroughRoute()
		return MutationResult{Message: "整改复验通过", Issues: p.Issues, Route: p.WalkthroughRoute}, nil
	})
}

func (s *Service) Walkthrough(ctx context.Context, c WalkthroughCommand) (MutationResult, error) {
	return s.execute(ctx, c.Meta, c, "walkthrough.recorded", func(p *domain.LayoutProject) (MutationResult, error) {
		expected, targeted := p.ExpectedWalkthroughRoute()
		if targeted {
			last := p.Walkthroughs[len(p.Walkthroughs)-1]
			if p.SignsRevision <= last.SignRevision || !p.TargetedRemediationComplete() {
				return MutationResult{}, errors.New("定向复验前必须完成关联标识整改")
			}
			if p.ValidatedSignsRevision != p.SignsRevision {
				return MutationResult{}, errors.New("当前标识方案尚未完成相关规则复验")
			}
			for _, issue := range p.Issues {
				if issue.Status == domain.IssueOpen {
					return MutationResult{}, errors.New("仍有未关闭规则问题，不能定向复验")
				}
			}
		}
		if len(c.Checkpoints) != len(expected) {
			return MutationResult{}, errors.New("走查站点数量与系统路线不一致")
		}
		ordered := make([]domain.Checkpoint, 0, len(expected))
		seen := map[string]bool{}
		for i, x := range expected {
			v := c.Checkpoints[i]
			if seen[v.ID] {
				return MutationResult{}, errors.New("走查站点不得重复")
			}
			seen[v.ID] = true
			if v.NodeID != x.NodeID {
				return MutationResult{}, errors.New("走查站点顺序或节点与系统路线不一致")
			}
			if v.ID != x.ID {
				return MutationResult{}, errors.New("走查站点 ID 不匹配")
			}
			ordered = append(ordered, v)
		}
		r, err := p.RecordWalkthrough(c.Meta.ActorID, ordered)
		if err != nil {
			return MutationResult{}, err
		}
		if r.Decision == "fail" {
			p.WalkthroughRoute, _ = p.ExpectedWalkthroughRoute()
			return MutationResult{Message: "走查失败，已生成定向复验清单", Route: p.WalkthroughRoute}, nil
		}
		p.WalkthroughRoute = nil
		return MutationResult{Message: "走查通过，等待独立签署", Route: expected}, nil
	})
}

func (s *Service) PreflightApproval(ctx context.Context, projectID, approver string) (domain.ApprovalPreflight, error) {
	type loadResult struct {
		project *domain.LayoutProject
		err     error
	}
	loaded := make(chan loadResult, 1)
	go func() {
		p, err := s.repo.LoadProject(context.Background(), projectID)
		loaded <- loadResult{project: p, err: err}
	}()
	select {
	case <-ctx.Done():
		return domain.ApprovalPreflight{}, ctx.Err()
	case result := <-loaded:
		if result.err != nil {
			return domain.ApprovalPreflight{}, result.err
		}
		return result.project.PreflightApproval(approver), nil
	}
}

func (s *Service) FreezePackage(ctx context.Context, c FreezePackageCommand) (MutationResult, error) {
	return s.execute(ctx, c.Meta, c, "package.frozen", func(p *domain.LayoutProject) (MutationResult, error) {
		preflight := p.PreflightApproval(c.Meta.ActorID)
		if c.SourceRevision != 0 && c.SourceRevision != preflight.SourceRevision {
			return MutationResult{}, ErrRevisionConflict
		}
		if c.CandidateDigest != "" && c.CandidateDigest != preflight.CandidateDigest {
			return MutationResult{}, fmt.Errorf("候选内容摘要已变化，请重新加载批准预检")
		}
		pkg, err := p.FreezePackage(c.Meta.ActorID)
		if err != nil {
			return MutationResult{}, err
		}
		// 先持久化内容寻址的不可变包，确保项目终态快照永远不会指向缺失文件。
		if err := s.repo.SavePackage(ctx, pkg); err != nil {
			return MutationResult{}, err
		}
		if pkg.SHA256Digest != preflight.CandidateDigest {
			return MutationResult{}, errors.New("最终包摘要与批准预检不一致")
		}
		return MutationResult{Message: "安装放样包已冻结", Package: &pkg, ApprovalPreflight: &preflight}, nil
	})
}
