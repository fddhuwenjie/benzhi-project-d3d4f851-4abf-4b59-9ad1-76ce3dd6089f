package canceled_lock_reference_leak_test

import (
	"context"
	"errors"
	"testing"

	"wayfinding-release-gate/internal/application"
)

func TestCanceledLockWaitReleasesCoordinatorReference(t *testing.T) {
	coordinator := application.NewCoordinator()
	unlock, err := coordinator.LockContext(context.Background(), "project-cancelled-wait")
	if err != nil {
		t.Fatalf("持有项目锁失败: %v", err)
	}

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = coordinator.LockContext(canceled, "project-cancelled-wait"); !errors.Is(err, context.Canceled) {
		t.Fatalf("取消的等待应返回 context.Canceled，实际为 %v", err)
	}

	unlock()
	if active := coordinator.Active(); active != 0 {
		t.Fatalf("取消等待后协调器仍保留 %d 个项目锁引用", active)
	}
}
