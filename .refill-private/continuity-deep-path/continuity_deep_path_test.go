package continuity_deep_path_test

import (
	"context"
	"fmt"
	"testing"

	"wayfinding-release-gate/internal/application"
	"wayfinding-release-gate/internal/domain"
	"wayfinding-release-gate/internal/store"
)

func continuityMeta(projectID string, revision, sequence int) application.CommandMeta {
	return application.CommandMeta{
		ProjectID: projectID, RequestID: fmt.Sprintf("continuity-%d", sequence),
		ExpectedRevision: revision, ActorID: "designer",
	}
}

func TestContinuityChecksEveryIntermediatePathNode(t *testing.T) {
	ctx := context.Background()
	repo, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	svc := application.NewService(repo)
	projectID := "deep-continuity"
	if _, err := svc.CreateProject(ctx, application.CreateProjectCommand{
		Meta: continuityMeta(projectID, 0, 1), BuildingName: "测试楼",
		DesignerID: "designer", ReviewerID: "reviewer",
	}); err != nil {
		t.Fatal(err)
	}
	graph := domain.SurveyGraph{
		Nodes: []domain.Node{
			{ID: "E", Kind: "entrance"}, {ID: "J1", Kind: "decision"},
			{ID: "J2", Kind: "decision"}, {ID: "D", Kind: "destination"},
		},
		Edges:       []domain.Edge{{From: "E", To: "J1"}, {From: "J1", To: "J2"}, {From: "J2", To: "D"}},
		EntranceIDs: []string{"E"}, DestinationIDs: []string{"D"},
	}
	if _, err := svc.FreezeBaseline(ctx, application.FreezeBaselineCommand{Meta: continuityMeta(projectID, 1, 2), Survey: graph}); err != nil {
		t.Fatal(err)
	}
	signs := []domain.SignProposal{
		{SignID: "S-J1", NodeID: "J1", DestinationRefs: []string{"D"}, Direction: "straight", DisplayText: "继续前行", VisibilityDistanceM: 8},
		{SignID: "S-D", NodeID: "D", DestinationRefs: []string{"D"}, Direction: "arrive", DisplayText: "已经到达", VisibilityDistanceM: 5},
	}
	replaced, err := svc.ReplaceSigns(ctx, application.ReplaceSignsCommand{Meta: continuityMeta(projectID, 2, 3), Signs: signs})
	if err != nil {
		t.Fatal(err)
	}
	validated, err := svc.Validate(ctx, application.ValidateCommand{Meta: continuityMeta(projectID, replaced.Revision, 4)})
	if err != nil {
		t.Fatal(err)
	}
	for _, issue := range validated.Issues {
		if issue.RuleCode == domain.RuleContinuity && issue.NodeID == "J2" {
			return
		}
	}
	t.Fatalf("入口后的深层路径节点缺少标识却通过连续性校验: status=%s issues=%+v", validated.Status, validated.Issues)
}
