package mid_commit_cancellation_test

import (
	"context"
	"errors"
	"testing"
	"time"
	"wayfinding-release-gate/internal/application"
	"wayfinding-release-gate/internal/domain"
	"wayfinding-release-gate/internal/store"
)

type cancelOnSecondCheckContext struct {
	checks int
	done   chan struct{}
}

func (c *cancelOnSecondCheckContext) Deadline() (time.Time, bool) { return time.Time{}, false }
func (c *cancelOnSecondCheckContext) Done() <-chan struct{}       { return c.done }
func (c *cancelOnSecondCheckContext) Value(any) any               { return nil }
func (c *cancelOnSecondCheckContext) Err() error {
	c.checks++
	if c.checks >= 2 {
		if c.checks == 2 {
			close(c.done)
		}
		return context.Canceled
	}
	return nil
}

func TestCancellationBeforeCommitLeavesNoPartialState(t *testing.T) {
	root := t.TempDir()
	repo, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	project := &domain.LayoutProject{
		ProjectID:    "canceled-commit-project",
		BuildingName: "测试建筑",
		DesignerID:   "designer",
		ReviewerID:   "reviewer",
		Status:       domain.StatusDraft,
		Revision:     1,
	}
	record := application.RequestRecord{
		ProjectID:   project.ProjectID,
		RequestID:   "create-request",
		Fingerprint: "fingerprint",
	}
	event := application.Event{
		ProjectID: project.ProjectID,
		Type:      "project.created",
		ActorID:   "designer",
		Revision:  1,
	}
	ctx := &cancelOnSecondCheckContext{done: make(chan struct{})}
	if err := repo.Commit(ctx, project, record, event); !errors.Is(err, context.Canceled) || err.Error() != "提交已取消: context canceled" {
		t.Fatalf("期望 context.Canceled，实际为 %v", err)
	}

	reopened, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := reopened.LoadProject(context.Background(), project.ProjectID); !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("取消的提交留下了项目快照，LoadProject 错误为 %v", err)
	}
	if events, err := reopened.Events(context.Background(), project.ProjectID); !errors.Is(err, application.ErrNotFound) || len(events) != 0 {
		t.Fatalf("取消的提交留下了事件：events=%d err=%v", len(events), err)
	}
}
