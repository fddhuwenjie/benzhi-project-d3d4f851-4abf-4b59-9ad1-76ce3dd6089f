package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
	"wayfinding-release-gate/internal/domain"
)

type Service struct {
	repo      Repository
	locks     *Coordinator
	now       func() time.Time
	replayMu  sync.RWMutex
	replayMap map[string]replayCacheEntry
	replayIDs []string
}

func NewService(repo Repository) *Service {
	return &Service{
		repo:      repo,
		locks:     NewCoordinator(),
		now:       func() time.Time { return time.Now().UTC() },
		replayMap: map[string]replayCacheEntry{},
	}
}

type replayCacheEntry struct {
	fingerprint string
	result      *MutationResult
}

const replayCacheLimit = 256

func replayCacheKey(meta CommandMeta) string {
	return meta.ProjectID + "\x00" + meta.RequestID
}

func (s *Service) cachedReplay(meta CommandMeta, command any) (*MutationResult, bool, error) {
	s.replayMu.RLock()
	entry, ok := s.replayMap[replayCacheKey(meta)]
	s.replayMu.RUnlock()
	if !ok {
		return nil, false, nil
	}
	if entry.fingerprint != fingerprint(command) {
		return nil, true, ErrIdempotencyConflict
	}
	return entry.result, true, nil
}

func (s *Service) rememberReplay(meta CommandMeta, command any, result *MutationResult) {
	key := replayCacheKey(meta)
	s.replayMu.Lock()
	if _, exists := s.replayMap[key]; !exists {
		if len(s.replayIDs) == replayCacheLimit {
			delete(s.replayMap, s.replayIDs[0])
			s.replayIDs = s.replayIDs[1:]
		}
		s.replayIDs = append(s.replayIDs, key)
	}
	s.replayMap[key] = replayCacheEntry{fingerprint: fingerprint(command), result: cloneMutationResult(result)}
	s.replayMu.Unlock()
}

func fingerprint(command any) string {
	b, _ := json.Marshal(command)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
func cloneProject(p *domain.LayoutProject) *domain.LayoutProject {
	b, _ := json.Marshal(p)
	var cp domain.LayoutProject
	_ = json.Unmarshal(b, &cp)
	return &cp
}
func cloneMutationResult(r *MutationResult) *MutationResult {
	if r == nil {
		return nil
	}
	b, _ := json.Marshal(r)
	var cp MutationResult
	_ = json.Unmarshal(b, &cp)
	return &cp
}

func (s *Service) replay(ctx context.Context, meta CommandMeta, command any) (*MutationResult, bool, error) {
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	if result, ok, err := s.cachedReplay(meta, command); ok {
		return cloneMutationResult(result), true, err
	}
	rec, err := s.repo.FindRequest(ctx, meta.ProjectID, meta.RequestID)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return nil, false, err
	}
	if rec == nil {
		return nil, false, nil
	}
	if rec.Fingerprint != fingerprint(command) {
		return nil, true, ErrIdempotencyConflict
	}
	if rec.Error != "" {
		return nil, true, recordedError(rec.Error)
	}
	var result MutationResult
	if err := json.Unmarshal(rec.Response, &result); err != nil {
		return nil, true, err
	}
	s.rememberReplay(meta, command, &result)
	return cloneMutationResult(&result), true, nil
}

func (s *Service) execute(ctx context.Context, meta CommandMeta, command any, eventType string, mutate func(*domain.LayoutProject) (MutationResult, error)) (MutationResult, error) {
	if err := meta.validate(); err != nil {
		return MutationResult{}, err
	}
	unlock := s.locks.Lock(meta.ProjectID)
	defer unlock()
	if old, ok, err := s.replay(ctx, meta, command); ok {
		return deref(old), err
	} else if err != nil {
		return MutationResult{}, err
	}
	p, err := s.repo.LoadProject(ctx, meta.ProjectID)
	if err != nil {
		return MutationResult{}, err
	}
	if p.Revision != meta.ExpectedRevision {
		_ = s.persistFailure(ctx, meta, command, ErrRevisionConflict)
		return MutationResult{}, ErrRevisionConflict
	}
	if err := p.EnsureEditable(); err != nil {
		_ = s.persistFailure(ctx, meta, command, err)
		return MutationResult{}, err
	}
	cp := cloneProject(p)
	result, mutErr := mutate(cp)
	rec := RequestRecord{ProjectID: meta.ProjectID, RequestID: meta.RequestID, Fingerprint: fingerprint(command), CreatedAt: s.now()}
	if mutErr != nil {
		rec.Error = mutErr.Error()
		_ = s.repo.SaveRequest(ctx, meta.ProjectID, rec)
		return MutationResult{}, mutErr
	}
	cp.Revision++
	result.ProjectID = cp.ProjectID
	result.Revision = cp.Revision
	result.Status = cp.Status
	payload, _ := json.Marshal(result)
	rec.Response = payload
	details := map[string]string{"message": result.Message}
	if result.Package != nil {
		details["package_id"] = result.Package.PackageID
		details["package_digest"] = result.Package.SHA256Digest
		details["source_revision"] = fmt.Sprint(result.Package.SourceRevision)
	}
	e := Event{ProjectID: cp.ProjectID, Type: eventType, ActorID: meta.ActorID, Revision: cp.Revision, At: s.now(), Details: details}
	if err := s.repo.Commit(ctx, cp, rec, e); err != nil {
		return MutationResult{}, err
	}
	s.rememberReplay(meta, command, &result)
	return result, nil
}
func recordedError(message string) error {
	for _, known := range []error{ErrRevisionConflict, ErrIdempotencyConflict, ErrNotFound} {
		if message == known.Error() {
			return known
		}
	}
	return errors.New(message)
}
func (s *Service) persistFailure(ctx context.Context, meta CommandMeta, command any, failure error) error {
	rec := RequestRecord{ProjectID: meta.ProjectID, RequestID: meta.RequestID, Fingerprint: fingerprint(command), Error: failure.Error(), CreatedAt: s.now()}
	return s.repo.SaveRequest(ctx, meta.ProjectID, rec)
}
func deref(r *MutationResult) MutationResult {
	if r == nil {
		return MutationResult{}
	}
	return *r
}

func (s *Service) GetProject(ctx context.Context, id string) (*domain.LayoutProject, error) {
	return s.GetProjectFiltered(ctx, id, domain.IssueFilter{})
}
func (s *Service) GetProjectFiltered(ctx context.Context, id string, filter domain.IssueFilter) (*domain.LayoutProject, error) {
	p, err := s.repo.LoadProject(ctx, id)
	if err != nil {
		return nil, err
	}
	p.Issues, p.IssueSummary = domain.QueryIssues(p.Issues, filter)
	if p.Status == domain.StatusReadyForWalkthrough || p.Status == domain.StatusWalkthroughFailed {
		p.WalkthroughRoute, _ = p.ExpectedWalkthroughRoute()
	}
	return p, nil
}
func (s *Service) ListProjects(ctx context.Context) ([]domain.LayoutProject, error) {
	return s.repo.ListProjects(ctx)
}
func (s *Service) Timeline(ctx context.Context, id string) ([]Event, error) {
	return s.repo.Events(ctx, id)
}
func (s *Service) GetPackage(ctx context.Context, id string) (domain.InstallationPackage, error) {
	pkg, err := s.repo.LoadPackage(ctx, id)
	if err != nil {
		return pkg, err
	}
	if !domain.VerifyPackage(pkg) {
		return pkg, errors.New("安装包摘要或标识校验失败")
	}
	return pkg, nil
}
func (s *Service) VerifyPackage(ctx context.Context, id string) (bool, domain.InstallationPackage, error) {
	p, err := s.repo.LoadPackage(ctx, id)
	if err != nil {
		return false, p, err
	}
	return domain.VerifyPackage(p), p, nil
}

func (s *Service) VerifyPackageDeep(ctx context.Context, id string) (domain.PackageVerification, error) {
	project, projectErr := s.repo.LoadProject(ctx, id)
	lookupID := id
	if projectErr == nil && project.Package != nil {
		lookupID = project.Package.PackageID
	}
	pkg, err := s.repo.LoadPackage(ctx, lookupID)
	if errors.Is(err, ErrNotFound) && projectErr == nil && project.Package != nil {
		pkg, err = s.repo.LoadPackage(ctx, project.Package.SHA256Digest)
	}
	if err != nil {
		return domain.PackageVerification{}, err
	}
	if projectErr != nil {
		project, err = s.repo.LoadProject(ctx, pkg.ProjectID)
		if err != nil {
			return domain.PackageVerification{}, err
		}
	}
	report := domain.PackageVerification{CheckedAt: s.now(), PackageID: pkg.PackageID, ProjectID: pkg.ProjectID, SourceRevision: pkg.SourceRevision, SHA256Digest: pkg.SHA256Digest}
	report.Summary.Passed = domain.VerifyPackage(pkg)
	if !report.Summary.Passed {
		report.Summary.Failures = []string{"规范载荷摘要或 package_id 不一致"}
	}
	report.Structure, report.Business = domain.VerifyPackageContent(pkg, project)
	report.EventChain.Passed = true
	events, eventErr := s.repo.Events(ctx, project.ProjectID)
	if eventErr != nil {
		report.EventChain.Passed = false
		report.EventChain.Failures = append(report.EventChain.Failures, "事件链读取或连续性校验失败: "+eventErr.Error())
		return report, nil
	}
	if len(events) == 0 {
		report.EventChain.Passed = false
		report.EventChain.Failures = append(report.EventChain.Failures, "事件链为空")
		return report, nil
	}
	last := events[len(events)-1]
	if last.Type != "package.frozen" {
		report.EventChain.Passed = false
		report.EventChain.Failures = append(report.EventChain.Failures, "终态事件不是 package.frozen")
	}
	if last.Revision != project.Revision {
		report.EventChain.Passed = false
		report.EventChain.Failures = append(report.EventChain.Failures, "终态事件修订与项目不一致")
	}
	if last.Details["package_digest"] != pkg.SHA256Digest {
		report.EventChain.Passed = false
		report.EventChain.Failures = append(report.EventChain.Failures, "终态事件包摘要与安装包不一致")
	}
	if last.Details["package_id"] != pkg.PackageID {
		report.EventChain.Passed = false
		report.EventChain.Failures = append(report.EventChain.Failures, "终态事件 package_id 与安装包不一致")
	}
	if last.Details["source_revision"] != fmt.Sprint(pkg.SourceRevision) {
		report.EventChain.Passed = false
		report.EventChain.Failures = append(report.EventChain.Failures, "终态事件 source_revision 与安装包不一致")
	}
	return report, nil
}

// normalizeLegacySigns 只兼容扩展前同时缺少方向和目的地的保存请求；预检 API 始终执行严格协议。
func normalizeLegacySigns(graph domain.SurveyGraph, signs []domain.SignProposal) []domain.SignProposal {
	out := append([]domain.SignProposal{}, signs...)
	for i := range out {
		if out[i].Direction == "" && len(out[i].DestinationRefs) == 0 {
			out[i].Direction = "straight"
			out[i].DestinationRefs = append([]string{}, graph.DestinationIDs...)
		}
	}
	return out
}
