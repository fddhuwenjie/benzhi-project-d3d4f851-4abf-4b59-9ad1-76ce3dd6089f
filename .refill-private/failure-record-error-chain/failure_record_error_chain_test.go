package failure_record_error_chain_test

import (
	"context"
	"errors"
	"testing"

	"wayfinding-release-gate/internal/application"
	"wayfinding-release-gate/internal/store"
)

var errRequestRecordUnavailable = errors.New("request record unavailable")

type rejectingFailureStore struct {
	*store.FileStore
}

func (s *rejectingFailureStore) SaveRequest(context.Context, string, application.RequestRecord) error {
	return errRequestRecordUnavailable
}

func TestRevisionConflictPreservesFailureRecordErrorChain(t *testing.T) {
	ctx := context.Background()
	files, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	svc := application.NewService(&rejectingFailureStore{FileStore: files})
	projectID := "failure-record"
	if _, err := svc.CreateProject(ctx, application.CreateProjectCommand{
		Meta:         application.CommandMeta{ProjectID: projectID, RequestID: "create", ExpectedRevision: 0, ActorID: "designer"},
		BuildingName: "测试楼", DesignerID: "designer", ReviewerID: "reviewer",
	}); err != nil {
		t.Fatal(err)
	}

	_, err = svc.FreezeBaseline(ctx, application.FreezeBaselineCommand{
		Meta: application.CommandMeta{ProjectID: projectID, RequestID: "stale-command", ExpectedRevision: 0, ActorID: "designer"},
	})
	if !errors.Is(err, application.ErrRevisionConflict) {
		t.Fatalf("测试前提不成立，未返回 revision 冲突: %v", err)
	}
	if !errors.Is(err, errRequestRecordUnavailable) {
		t.Fatalf("失败请求记录的持久化错误从错误链中丢失: %v", err)
	}
}
