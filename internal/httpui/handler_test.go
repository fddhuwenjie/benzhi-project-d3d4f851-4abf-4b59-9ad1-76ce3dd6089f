package httpui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"wayfinding-release-gate/internal/application"
	"wayfinding-release-gate/internal/store"
)

func testHandler(t *testing.T) http.Handler {
	t.Helper()
	repo, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return New(application.NewService(repo))
}
func TestIndexAndContentTypeLimit(t *testing.T) {
	h := testHandler(t)
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "<body>") {
		t.Fatal("首页未提供完整 HTML")
	}
	r = httptest.NewRequest(http.MethodPost, "/api/projects", strings.NewReader(`{}`))
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 422 || !strings.Contains(w.Body.String(), "Content-Type") {
		t.Fatalf("应拒绝非 JSON: %d %s", w.Code, w.Body.String())
	}
}
func TestRevisionConflictErrorCode(t *testing.T) {
	h := testHandler(t)
	body := `{"meta":{"project_id":"p","request_id":"r1","expected_revision":0,"actor_id":"d"},"building_name":"楼","designer_id":"d","reviewer_id":"r"}`
	r := httptest.NewRequest(http.MethodPost, "/api/projects", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 201 {
		t.Fatalf("创建失败: %s", w.Body.String())
	}
	bad := `{"meta":{"project_id":"p","request_id":"r2","expected_revision":0,"actor_id":"d"},"survey":{"nodes":[],"edges":[],"entrance_ids":[],"destination_ids":[],"accessible_edge_flags":{}}}`
	r = httptest.NewRequest(http.MethodPost, "/api/projects/p/baseline", strings.NewReader(bad))
	r.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 409 || !strings.Contains(w.Body.String(), "revision_conflict") {
		t.Fatalf("冲突错误协议不正确: %d %s", w.Code, w.Body.String())
	}
}
