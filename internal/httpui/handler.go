package httpui

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"wayfinding-release-gate/internal/application"
)

type Handler struct {
	service *application.Service
	mux     *http.ServeMux
}

func New(service *application.Service) *Handler {
	h := &Handler{service: service, mux: http.NewServeMux()}
	h.routes()
	return h
}
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "same-origin")
	h.mux.ServeHTTP(w, r)
}
func (h *Handler) routes() {
	h.mux.HandleFunc("GET /", h.IndexHandler)
	h.mux.HandleFunc("GET /assets/app.css", h.CSSHandler)
	h.mux.HandleFunc("GET /assets/app.js", h.JSHandler)
	h.mux.HandleFunc("GET /healthz", h.HealthHandler)
	h.mux.HandleFunc("GET /api/projects", h.ListProjectsHandler)
	h.mux.HandleFunc("POST /api/projects", h.CreateProjectHandler)
	h.mux.HandleFunc("GET /api/projects/{id}", h.ProjectHandler)
	h.mux.HandleFunc("GET /api/projects/{id}/timeline", h.TimelineHandler)
	h.mux.HandleFunc("POST /api/projects/{id}/baseline", h.FreezeBaselineHandler)
	h.mux.HandleFunc("POST /api/projects/{id}/baseline/preflight", h.BaselinePreflightHandler)
	h.mux.HandleFunc("PUT /api/projects/{id}/signs", h.ReplaceSignsHandler)
	h.mux.HandleFunc("POST /api/projects/{id}/signs/preflight", h.SignPreflightHandler)
	h.mux.HandleFunc("POST /api/projects/{id}/validate", h.ValidateHandler)
	h.mux.HandleFunc("POST /api/projects/{id}/issues/{issue}/resolve", h.ResolveIssueHandler)
	h.mux.HandleFunc("POST /api/projects/{id}/reverify", h.ReverifyHandler)
	h.mux.HandleFunc("POST /api/projects/{id}/walkthrough", h.WalkthroughHandler)
	h.mux.HandleFunc("GET /api/projects/{id}/approval-preflight", h.ApprovalPreflightHandler)
	h.mux.HandleFunc("POST /api/projects/{id}/freeze", h.FreezePackageHandler)
	h.mux.HandleFunc("GET /api/projects/{id}/package", h.DownloadPackageHandler)
	h.mux.HandleFunc("POST /api/projects/{id}/package/verify", h.VerifyPackageHandler)
}

type errorBody struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func writeError(w http.ResponseWriter, err error) {
	status := http.StatusUnprocessableEntity
	code := "business_rule"
	switch {
	case errors.Is(err, application.ErrNotFound):
		status = http.StatusNotFound
		code = "not_found"
	case errors.Is(err, application.ErrRevisionConflict):
		status = http.StatusConflict
		code = "revision_conflict"
	case errors.Is(err, application.ErrIdempotencyConflict):
		status = http.StatusConflict
		code = "idempotency_conflict"
	case errors.Is(err, io.EOF):
		status = http.StatusBadRequest
		code = "invalid_json"
	}
	var body errorBody
	body.Error.Code = code
	body.Error.Message = err.Error()
	writeJSON(w, status, body)
}
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	if !strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "application/json") {
		return errors.New("Content-Type 必须为 application/json")
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return fmt.Errorf("请求 JSON 无效: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return errors.New("请求体只能包含一个 JSON 对象")
	}
	return nil
}
func bindProjectID(pathID string, meta *application.CommandMeta) error {
	if meta.ProjectID == "" {
		meta.ProjectID = pathID
	}
	if meta.ProjectID != pathID {
		return errors.New("路径 project_id 与请求体不一致")
	}
	return nil
}
