package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
	"wayfinding-release-gate/internal/application"
	"wayfinding-release-gate/internal/domain"
)

func TestCommitRestartAndEventIntegrity(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	p := &domain.LayoutProject{ProjectID: "project-a", BuildingName: "测试楼", DesignerID: "d", ReviewerID: "r", Status: domain.StatusDraft, Revision: 1, CreatedAt: time.Now()}
	rec := application.RequestRecord{ProjectID: p.ProjectID, RequestID: "req-1", Fingerprint: "abc", Response: []byte(`{"revision":1}`), CreatedAt: time.Now()}
	event := application.Event{ProjectID: p.ProjectID, Type: "project.created", ActorID: "d", Revision: 1, At: time.Now()}
	if err := s.Commit(context.Background(), p, rec, event); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := reopened.LoadProject(context.Background(), p.ProjectID)
	if err != nil || loaded.Revision != 1 {
		t.Fatalf("重启加载失败: %v", err)
	}
	events, err := reopened.Events(context.Background(), p.ProjectID)
	if err != nil || len(events) != 1 || events[0].Digest == "" {
		t.Fatalf("事件链无效: %+v %v", events, err)
	}
}

func TestOpenRejectsCorruptEventChain(t *testing.T) {
	dir := t.TempDir()
	if _, err := Open(dir); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "events", "broken.frames")
	if err := os.WriteFile(path, []byte("{\"length\":2,\"event\":{}}\n"), 0640); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(dir); err == nil {
		t.Fatal("损坏事件链必须阻止启动")
	}
}
