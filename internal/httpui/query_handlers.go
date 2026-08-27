package httpui

import (
	"net/http"
	"wayfinding-release-gate/internal/domain"
)

func (h *Handler) ListProjectsHandler(w http.ResponseWriter, r *http.Request) {
	items, err := h.service.ListProjects(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"projects": items})
}
func (h *Handler) ProjectHandler(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filter := domain.IssueFilter{Severity: q.Get("severity"), RuleCode: q.Get("rule_code"), NodeID: q.Get("node_id"), Status: q.Get("status")}
	p, err := h.service.GetProjectFiltered(r.Context(), r.PathValue("id"), filter)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}
func (h *Handler) TimelineHandler(w http.ResponseWriter, r *http.Request) {
	events, err := h.service.Timeline(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"events": events})
}
func (h *Handler) DownloadPackageHandler(w http.ResponseWriter, r *http.Request) {
	p, err := h.service.GetPackage(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=installation-package.json")
	writeJSON(w, http.StatusOK, p)
}
func (h *Handler) VerifyPackageHandler(w http.ResponseWriter, r *http.Request) {
	var input struct{}
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, err)
		return
	}
	report, err := h.service.VerifyPackageDeep(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	valid := report.Summary.Passed && report.Structure.Passed && report.Business.Passed && report.EventChain.Passed
	writeJSON(w, http.StatusOK, map[string]any{"valid": valid, "verification": report})
}
