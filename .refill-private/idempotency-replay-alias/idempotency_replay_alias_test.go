package idempotencyreplayalias_test

import (
	"context"
	"testing"

	"wayfinding-release-gate/internal/application"
	"wayfinding-release-gate/internal/domain"
	"wayfinding-release-gate/internal/store"
)

func TestIdempotencyReplayIsolatedFromCallerMutation(t *testing.T) {
	ctx := context.Background()
	repo, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	svc := application.NewService(repo)
	projectID := "replay-alias"
	create := application.CreateProjectCommand{
		Meta: application.CommandMeta{
			ProjectID: projectID, RequestID: "create-1", ActorID: "designer",
		},
		BuildingName: "市民中心", DesignerID: "designer", ReviewerID: "reviewer",
	}
	if _, err := svc.CreateProject(ctx, create); err != nil {
		t.Fatal(err)
	}

	freeze := application.FreezeBaselineCommand{
		Meta: application.CommandMeta{
			ProjectID: projectID, RequestID: "freeze-1", ExpectedRevision: 1, ActorID: "designer",
		},
		Survey: domain.SurveyGraph{
			Nodes: []domain.Node{
				{ID: "destination", Name: "服务台", Kind: "destination"},
				{ID: "entrance", Name: "东入口", Kind: "entrance"},
			},
			Edges:          []domain.Edge{{From: "entrance", To: "destination"}},
			EntranceIDs:    []string{"entrance"},
			DestinationIDs: []string{"destination"},
		},
	}
	first, err := svc.FreezeBaseline(ctx, freeze)
	if err != nil {
		t.Fatal(err)
	}
	if first.BaselinePreflight == nil || len(first.BaselinePreflight.Graph.Nodes) == 0 {
		t.Fatal("首次响应缺少基线预检图")
	}
	replayCommand := freeze
	replayCommand.Survey.Nodes = append([]domain.Node(nil), freeze.Survey.Nodes...)
	replayCommand.Survey.Edges = append([]domain.Edge(nil), freeze.Survey.Edges...)
	replayCommand.Survey.EntranceIDs = append([]string(nil), freeze.Survey.EntranceIDs...)
	replayCommand.Survey.DestinationIDs = append([]string(nil), freeze.Survey.DestinationIDs...)
	const callerValue = "调用方污染值"
	first.BaselinePreflight.Graph.Nodes[0].Name = callerValue

	replayed, err := svc.FreezeBaseline(ctx, replayCommand)
	if err != nil {
		t.Fatal(err)
	}
	if got := replayed.BaselinePreflight.Graph.Nodes[0].Name; got == callerValue {
		t.Fatalf("idempotency replay reused caller-owned baseline graph: %q", got)
	}
}
