package detachedapprovalload_test

import (
	"context"
	"testing"

	"wayfinding-release-gate/internal/application"
	"wayfinding-release-gate/internal/domain"
)

type blockingRepository struct {
	started chan struct{}
	release chan struct{}
	done    chan struct{}
}

func (r *blockingRepository) LoadProject(ctx context.Context, _ string) (*domain.LayoutProject, error) {
	close(r.started)
	defer close(r.done)
	select {
	case <-r.release:
		return &domain.LayoutProject{}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (*blockingRepository) ListProjects(context.Context) ([]domain.LayoutProject, error) {
	panic("unexpected ListProjects call")
}

func (*blockingRepository) FindRequest(context.Context, string, string) (*application.RequestRecord, error) {
	panic("unexpected FindRequest call")
}

func (*blockingRepository) SaveRequest(context.Context, string, application.RequestRecord) error {
	panic("unexpected SaveRequest call")
}

func (*blockingRepository) Commit(context.Context, *domain.LayoutProject, application.RequestRecord, application.Event) error {
	panic("unexpected Commit call")
}

func (*blockingRepository) Events(context.Context, string) ([]application.Event, error) {
	panic("unexpected Events call")
}

func (*blockingRepository) SavePackage(context.Context, domain.InstallationPackage) error {
	panic("unexpected SavePackage call")
}

func (*blockingRepository) LoadPackage(context.Context, string) (domain.InstallationPackage, error) {
	panic("unexpected LoadPackage call")
}

func TestPreflightApprovalCancellationDoesNotLeaveDetachedLoad(t *testing.T) {
	repo := &blockingRepository{
		started: make(chan struct{}),
		release: make(chan struct{}),
		done:    make(chan struct{}),
	}
	svc := application.NewService(repo)
	ctx, cancel := context.WithCancel(context.Background())
	returned := make(chan error, 1)
	go func() {
		_, err := svc.PreflightApproval(ctx, "approval-project", "reviewer")
		returned <- err
	}()

	<-repo.started
	cancel()
	if err := <-returned; err != context.Canceled {
		close(repo.release)
		<-repo.done
		t.Fatalf("expected context.Canceled, got %v", err)
	}

	select {
	case <-repo.done:
		// A canceled repository call must finish before the service returns.
	default:
		close(repo.release)
		<-repo.done
		t.Fatal("repository load remained active after cancellation")
	}
}
