package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
	"wayfinding-release-gate/internal/domain"
)

func (s *Service) PreflightBaseline(ctx context.Context, projectID string, survey domain.SurveyGraph) (domain.BaselinePreflight, error) {
	p, err := s.repo.LoadProject(ctx, projectID)
	if err != nil {
		return domain.BaselinePreflight{}, err
	}
	if p.Status != domain.StatusDraft {
		return domain.BaselinePreflight{}, errors.New("只有草稿项目可预检基线")
	}
	return domain.PreflightBaseline(survey), nil
}

func (s *Service) PreflightSigns(ctx context.Context, projectID string, signs []domain.SignProposal) (domain.SignPreflight, error) {
	p, err := s.repo.LoadProject(ctx, projectID)
	if err != nil {
		return domain.SignPreflight{}, err
	}
	if p.Status == domain.StatusDraft || p.Status == domain.StatusFrozen {
		return domain.SignPreflight{}, errors.New("当前状态不能预检标识")
	}
	return domain.PreflightSigns(p.Survey, p.Signs, signs), nil
}

func (s *Service) CreateProject(ctx context.Context, c CreateProjectCommand) (MutationResult, error) {
	if err := c.Meta.validate(); err != nil {
		return MutationResult{}, err
	}
	unlock := s.locks.Lock(c.Meta.ProjectID)
	defer unlock()
	if old, ok, err := s.replay(ctx, c.Meta, c); ok {
		return deref(old), err
	} else if err != nil {
		return MutationResult{}, err
	}
	if c.Meta.ExpectedRevision != 0 {
		return MutationResult{}, ErrRevisionConflict
	}
	if _, err := s.repo.LoadProject(ctx, c.Meta.ProjectID); err == nil {
		return MutationResult{}, errors.New("项目已存在")
	} else if !errors.Is(err, ErrNotFound) {
		return MutationResult{}, err
	}
	p := &domain.LayoutProject{ProjectID: c.Meta.ProjectID, BuildingName: c.BuildingName, Zones: c.Zones, DesignerID: c.DesignerID, ReviewerID: c.ReviewerID, Status: domain.StatusDraft, Revision: 1, CreatedAt: s.now()}
	if err := p.ValidateIdentity(); err != nil {
		return MutationResult{}, err
	}
	result := MutationResult{ProjectID: p.ProjectID, Revision: 1, Status: p.Status, Message: "项目已创建"}
	b, _ := json.Marshal(result)
	rec := RequestRecord{ProjectID: p.ProjectID, RequestID: c.Meta.RequestID, Fingerprint: fingerprint(c), Response: b, CreatedAt: s.now()}
	e := Event{ProjectID: p.ProjectID, Type: "project.created", ActorID: c.Meta.ActorID, Revision: 1, At: s.now(), Details: map[string]string{"building": p.BuildingName}}
	if err := s.repo.Commit(ctx, p, rec, e); err != nil {
		return MutationResult{}, err
	}
	return result, nil
}

func (s *Service) FreezeBaseline(ctx context.Context, c FreezeBaselineCommand) (MutationResult, error) {
	return s.execute(ctx, c.Meta, c, "baseline.frozen", func(p *domain.LayoutProject) (MutationResult, error) {
		if p.Status != domain.StatusDraft {
			return MutationResult{}, errors.New("只有草稿项目可冻结基线")
		}
		preflight := domain.PreflightBaseline(c.Survey)
		if len(preflight.Blockers) > 0 {
			return MutationResult{}, fmt.Errorf("基线预检失败 [%s]: %s", preflight.Blockers[0].Code, preflight.Blockers[0].Message)
		}
		preflight.Graph.BaselineDigest = preflight.Graph.ComputeDigest()
		p.Survey = preflight.Graph
		t := s.now()
		p.BaselineFrozenAt = &t
		if err := p.Transition(domain.StatusBaselineFrozen); err != nil {
			return MutationResult{}, err
		}
		return MutationResult{Message: "勘测基线已冻结", BaselinePreflight: &preflight}, nil
	})
}

func (s *Service) ReplaceSigns(ctx context.Context, c ReplaceSignsCommand) (MutationResult, error) {
	c.Signs = domain.NormalizeSigns(c.Signs)
	return s.execute(ctx, c.Meta, c, "signs.replaced", func(p *domain.LayoutProject) (MutationResult, error) {
		if p.Status != domain.StatusBaselineFrozen && p.Status != domain.StatusNeedsRemediation && p.Status != domain.StatusWalkthroughFailed {
			return MutationResult{}, errors.New("当前状态不能编辑标识")
		}
		preflight := domain.PreflightSigns(p.Survey, p.Signs, normalizeLegacySigns(p.Survey, c.Signs))
		if len(preflight.Errors) > 0 {
			return MutationResult{}, errors.New(preflight.Errors[0].Message)
		}
		changedNodes, changedDestinations := domain.ChangedSignScope(p.Signs, preflight.Signs)
		if len(preflight.Diff.Added)+len(preflight.Diff.Modified)+len(preflight.Diff.Deleted) > 0 {
			p.SignsRevision++
			p.InvalidateEvidence(changedNodes, changedDestinations, "标识方案变化涉及问题范围")
			p.MarkWalkthroughRemediation(changedNodes, changedDestinations)
		}
		p.Signs = preflight.Signs
		if p.Status == domain.StatusWalkthroughFailed {
			_ = p.Transition(domain.StatusNeedsRemediation)
		}
		return MutationResult{Message: "标识方案已更新", SignPreflight: &preflight}, nil
	})
}

func (s *Service) Validate(ctx context.Context, c ValidateCommand) (MutationResult, error) {
	return s.execute(ctx, c.Meta, c, "rules.validated", func(p *domain.LayoutProject) (MutationResult, error) {
		if len(p.Signs) == 0 {
			return MutationResult{}, errors.New("请先录入候选标识")
		}
		fresh := p.ValidateRules()
		p.Issues = domain.MergeValidationIssues(p.Issues, fresh)
		_, p.IssueSummary = domain.QueryIssues(p.Issues, domain.IssueFilter{})
		p.LastValidatedAt = timePtr(s.now())
		p.ValidatedSignsRevision = p.SignsRevision
		hasOpen := false
		for _, issue := range p.Issues {
			if issue.Status == domain.IssueOpen {
				hasOpen = true
				break
			}
		}
		if hasOpen {
			if p.Status == domain.StatusBaselineFrozen {
				_ = p.Transition(domain.StatusNeedsRemediation)
			}
			return MutationResult{Message: "校验发现问题", Issues: p.Issues}, nil
		}
		if p.Status == domain.StatusBaselineFrozen || p.Status == domain.StatusNeedsRemediation {
			_ = p.Transition(domain.StatusReadyForWalkthrough)
		}
		p.WalkthroughRoute, _ = p.ExpectedWalkthroughRoute()
		return MutationResult{Message: "规则校验通过", Issues: p.Issues, Route: p.WalkthroughRoute}, nil
	})
}
func timePtr(t time.Time) *time.Time { return &t }
