package stale_project_query_cache_test

import (
	"context"
	"testing"

	"wayfinding-release-gate/internal/application"
	"wayfinding-release-gate/internal/domain"
	"wayfinding-release-gate/internal/store"
)

func TestSharedRepositoryMutationInvalidatesProjectQueryCache(t *testing.T) {
	ctx := context.Background()
	repo, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	reader := application.NewService(repo)
	writer := application.NewService(repo)
	projectID := "shared-query-cache"

	_, err = reader.CreateProject(ctx, application.CreateProjectCommand{
		Meta: application.CommandMeta{
			ProjectID: projectID, RequestID: "create-1", ExpectedRevision: 0, ActorID: "designer",
		},
		BuildingName: "市民中心", DesignerID: "designer", ReviewerID: "reviewer",
	})
	if err != nil {
		t.Fatal(err)
	}
	first, err := reader.GetProject(ctx, projectID)
	if err != nil {
		t.Fatal(err)
	}
	if first.Revision != 1 || first.Status != domain.StatusDraft {
		t.Fatalf("初始项目状态异常: revision=%d status=%s", first.Revision, first.Status)
	}

	graph := domain.SurveyGraph{
		Nodes: []domain.Node{
			{ID: "ENT", Kind: "entrance"},
			{ID: "DEST", Kind: "destination"},
		},
		Edges:          []domain.Edge{{From: "ENT", To: "DEST"}},
		EntranceIDs:    []string{"ENT"},
		DestinationIDs: []string{"DEST"},
	}
	_, err = writer.FreezeBaseline(ctx, application.FreezeBaselineCommand{
		Meta: application.CommandMeta{
			ProjectID: projectID, RequestID: "freeze-2", ExpectedRevision: 1, ActorID: "designer",
		},
		Survey: graph,
	})
	if err != nil {
		t.Fatal(err)
	}

	updated, err := reader.GetProject(ctx, projectID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Revision != 2 || updated.Status != domain.StatusBaselineFrozen {
		t.Fatalf("共享仓储提交后查询仍返回陈旧快照: revision=%d status=%s", updated.Revision, updated.Status)
	}
}
