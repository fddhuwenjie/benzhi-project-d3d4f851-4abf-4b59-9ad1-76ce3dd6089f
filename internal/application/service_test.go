package application_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"wayfinding-release-gate/internal/application"
	"wayfinding-release-gate/internal/domain"
	"wayfinding-release-gate/internal/store"
)

func m(id string, rev int, actor string, seq int) application.CommandMeta {
	return application.CommandMeta{ProjectID: id, RequestID: fmt.Sprintf("r-%d", seq), ExpectedRevision: rev, ActorID: actor}
}

func TestFullRemediationApprovalAndIdempotency(t *testing.T) {
	ctx := context.Background()
	repo, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	svc := application.NewService(repo)
	id := "approval-flow"
	created, err := svc.CreateProject(ctx, application.CreateProjectCommand{Meta: m(id, 0, "designer", 1), BuildingName: "文化馆", Zones: []string{"一层"}, DesignerID: "designer", ReviewerID: "reviewer"})
	if err != nil {
		t.Fatal(err)
	}
	replay, err := svc.CreateProject(ctx, application.CreateProjectCommand{Meta: m(id, 0, "designer", 1), BuildingName: "文化馆", Zones: []string{"一层"}, DesignerID: "designer", ReviewerID: "reviewer"})
	if err != nil || replay.Revision != created.Revision {
		t.Fatalf("创建幂等重放失败: %v", err)
	}
	graph := domain.SurveyGraph{Nodes: []domain.Node{{ID: "E", Kind: "entrance"}, {ID: "J", Kind: "decision"}, {ID: "D", Kind: "destination"}}, Edges: []domain.Edge{{From: "E", To: "J"}, {From: "J", To: "D"}}, EntranceIDs: []string{"E"}, DestinationIDs: []string{"D"}}
	if _, err = svc.FreezeBaseline(ctx, application.FreezeBaselineCommand{Meta: m(id, 1, "designer", 2), Survey: graph}); err != nil {
		t.Fatal(err)
	}
	badSigns := []domain.SignProposal{{SignID: "d", NodeID: "D", DisplayText: "到达", VisibilityDistanceM: 5}}
	if _, err = svc.ReplaceSigns(ctx, application.ReplaceSignsCommand{Meta: m(id, 2, "designer", 3), Signs: badSigns}); err != nil {
		t.Fatal(err)
	}
	validated, err := svc.Validate(ctx, application.ValidateCommand{Meta: m(id, 3, "designer", 4)})
	if err != nil || len(validated.Issues) == 0 {
		t.Fatalf("应发现规则问题: %v", err)
	}
	goodSigns := []domain.SignProposal{{SignID: "j", NodeID: "J", DisplayText: "服务台", VisibilityDistanceM: 8}, {SignID: "d", NodeID: "D", DisplayText: "到达", VisibilityDistanceM: 5}}
	rev := validated.Revision
	for n, issue := range validated.Issues {
		res, resolveErr := svc.ResolveIssue(ctx, application.ResolveIssueCommand{Meta: m(id, rev, "designer", 10+n), IssueID: issue.IssueID, Resolution: "补充连续指引", Evidence: "照片摘要", UpdatedSigns: goodSigns})
		if resolveErr != nil {
			t.Fatal(resolveErr)
		}
		rev = res.Revision
	}
	verified, err := svc.Reverify(ctx, application.ReverifyCommand{Meta: m(id, rev, "designer", 20)})
	if err != nil || verified.Status != domain.StatusReadyForWalkthrough {
		t.Fatalf("复验失败: %+v %v", verified, err)
	}
	checks := verified.Route
	for i := range checks {
		checks[i].Visible = true
		checks[i].DirectionCorrect = true
	}
	walk, err := svc.Walkthrough(ctx, application.WalkthroughCommand{Meta: m(id, verified.Revision, "reviewer", 21), Checkpoints: checks})
	if err != nil || walk.Status != domain.StatusReadyForApproval {
		t.Fatalf("走查失败: %v", err)
	}
	frozen, err := svc.FreezePackage(ctx, application.FreezePackageCommand{Meta: m(id, walk.Revision, "reviewer", 22)})
	if err != nil || frozen.Status != domain.StatusFrozen || frozen.Package == nil {
		t.Fatalf("冻结失败: %+v %v", frozen, err)
	}
	if ok, _, err := svc.VerifyPackage(ctx, id); err != nil || !ok {
		t.Fatalf("安装包验证失败: %v", err)
	}
	report, err := svc.VerifyPackageDeep(ctx, id)
	if err != nil || !report.Summary.Passed || !report.Structure.Passed || !report.Business.Passed || !report.EventChain.Passed {
		t.Fatalf("安装包深度核验失败: %+v %v", report, err)
	}
	_, err = svc.Validate(ctx, application.ValidateCommand{Meta: m(id, frozen.Revision, "designer", 23)})
	if err == nil {
		t.Fatal("冻结后必须拒绝写入")
	}
}

func TestPreflightFailuresDoNotChangeProject(t *testing.T) {
	ctx := context.Background()
	repo, _ := store.Open(t.TempDir())
	svc := application.NewService(repo)
	id := "atomic-preflight"
	_, err := svc.CreateProject(ctx, application.CreateProjectCommand{Meta: m(id, 0, "d", 1), BuildingName: "楼", DesignerID: "d", ReviewerID: "r"})
	if err != nil {
		t.Fatal(err)
	}
	badGraph := domain.SurveyGraph{Nodes: []domain.Node{{ID: "E", Kind: "entrance"}, {ID: "D", Kind: "destination"}}, Edges: []domain.Edge{{From: "missing", To: "D"}}, EntranceIDs: []string{"E", "E"}, DestinationIDs: []string{"D"}}
	preflight, err := svc.PreflightBaseline(ctx, id, badGraph)
	if err != nil || len(preflight.Blockers) < 2 {
		t.Fatalf("预检应返回多个定位阻断项: %+v %v", preflight, err)
	}
	if _, err = svc.FreezeBaseline(ctx, application.FreezeBaselineCommand{Meta: m(id, 1, "d", 2), Survey: badGraph}); err == nil {
		t.Fatal("无效基线不得冻结")
	}
	p, _ := svc.GetProject(ctx, id)
	if p.Revision != 1 || p.Status != domain.StatusDraft || p.Survey.BaselineDigest != "" {
		t.Fatalf("失败冻结改变了项目: %+v", p)
	}
}

func TestWalkthroughFailureRequiresReasonAndPreservesRoute(t *testing.T) {
	ctx := context.Background()
	repo, _ := store.Open(t.TempDir())
	svc := application.NewService(repo)
	id := "targeted-walk"
	_, _ = svc.CreateProject(ctx, application.CreateProjectCommand{Meta: m(id, 0, "d", 1), BuildingName: "楼", DesignerID: "d", ReviewerID: "r"})
	graph := domain.SurveyGraph{Nodes: []domain.Node{{ID: "E", Kind: "entrance"}, {ID: "D", Kind: "destination"}}, Edges: []domain.Edge{{From: "E", To: "D"}}, EntranceIDs: []string{"E"}, DestinationIDs: []string{"D"}}
	_, _ = svc.FreezeBaseline(ctx, application.FreezeBaselineCommand{Meta: m(id, 1, "d", 2), Survey: graph})
	signs := []domain.SignProposal{{SignID: "S", NodeID: "D", DestinationRefs: []string{"D"}, Direction: "arrive", DisplayText: "到达", VisibilityDistanceM: 5}}
	_, _ = svc.ReplaceSigns(ctx, application.ReplaceSignsCommand{Meta: m(id, 2, "d", 3), Signs: signs})
	validated, err := svc.Validate(ctx, application.ValidateCommand{Meta: m(id, 3, "d", 4)})
	if err != nil {
		t.Fatal(err)
	}
	checks := validated.Route
	for i := range checks {
		checks[i].Visible = true
		checks[i].DirectionCorrect = true
	}
	checks[0].Visible = false
	if _, err = svc.Walkthrough(ctx, application.WalkthroughCommand{Meta: m(id, 4, "r", 5), Checkpoints: checks}); err == nil {
		t.Fatal("失败站点缺少原因应被拒绝")
	}
	p, _ := svc.GetProject(ctx, id)
	if p.Revision != 4 {
		t.Fatalf("拒绝后修订发生变化: %d", p.Revision)
	}
	checks[0].Note = "入口处被施工围挡遮挡"
	failed, err := svc.Walkthrough(ctx, application.WalkthroughCommand{Meta: m(id, 4, "r", 6), Checkpoints: checks})
	if err != nil {
		t.Fatal(err)
	}
	if failed.Status != domain.StatusWalkthroughFailed || len(failed.Route) != 1 || failed.Route[0].ID != checks[0].ID {
		t.Fatalf("定向清单不正确: %+v", failed)
	}
	if _, err = svc.Reverify(ctx, application.ReverifyCommand{Meta: m(id, failed.Revision, "d", 7)}); err == nil {
		t.Fatal("没有关联标识整改时不得取得定向复验资格")
	}
	signs = append(signs, domain.SignProposal{SignID: "S-REMEDIATION", NodeID: checks[0].NodeID, DestinationRefs: []string{"D"}, Direction: "straight", DisplayText: "整改后的现场指引", VisibilityDistanceM: 8})
	replaced, err := svc.ReplaceSigns(ctx, application.ReplaceSignsCommand{Meta: m(id, failed.Revision, "d", 8), Signs: signs})
	if err != nil {
		t.Fatal(err)
	}
	revalidated, err := svc.Validate(ctx, application.ValidateCommand{Meta: m(id, replaced.Revision, "d", 9)})
	if err != nil || revalidated.Status != domain.StatusReadyForWalkthrough || len(revalidated.Route) != 1 {
		t.Fatalf("整改复验后未恢复定向路线: %+v %v", revalidated, err)
	}
	targeted := revalidated.Route
	targeted[0].Visible = true
	targeted[0].DirectionCorrect = true
	passed, err := svc.Walkthrough(ctx, application.WalkthroughCommand{Meta: m(id, revalidated.Revision, "r", 10), Checkpoints: targeted})
	if err != nil || passed.Status != domain.StatusReadyForApproval {
		t.Fatalf("定向复验未闭环: %+v %v", passed, err)
	}
}

func TestRevisionFailureIsPersistentlyIdempotent(t *testing.T) {
	ctx := context.Background()
	repo, _ := store.Open(t.TempDir())
	svc := application.NewService(repo)
	id := "conflict"
	_, _ = svc.CreateProject(ctx, application.CreateProjectCommand{Meta: m(id, 0, "d", 1), BuildingName: "楼", DesignerID: "d", ReviewerID: "r"})
	cmd := application.FreezeBaselineCommand{Meta: m(id, 0, "d", 2)}
	_, err := svc.FreezeBaseline(ctx, cmd)
	if !errors.Is(err, application.ErrRevisionConflict) {
		t.Fatalf("预期 revision 冲突: %v", err)
	}
	cmd.Meta.ExpectedRevision = 1
	_, err = svc.FreezeBaseline(ctx, cmd)
	if !errors.Is(err, application.ErrIdempotencyConflict) {
		t.Fatalf("同 request_id 换内容应冲突: %v", err)
	}
}
