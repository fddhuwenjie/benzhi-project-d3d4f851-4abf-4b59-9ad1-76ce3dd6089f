package httpui

import (
	"errors"
	"net/http"
	"wayfinding-release-gate/internal/application"
)

func (h *Handler) ResolveIssueHandler(w http.ResponseWriter, r *http.Request) {
	var c application.ResolveIssueCommand
	if err := decodeJSON(w, r, &c); err != nil {
		writeError(w, err)
		return
	}
	if err := bindProjectID(r.PathValue("id"), &c.Meta); err != nil {
		writeError(w, err)
		return
	}
	issue := r.PathValue("issue")
	if c.IssueID == "" {
		c.IssueID = issue
	}
	if c.IssueID != issue {
		writeError(w, errors.New("路径 issue_id 与请求体不一致"))
		return
	}
	result, err := h.service.ResolveIssue(r.Context(), c)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
func (h *Handler) ReverifyHandler(w http.ResponseWriter, r *http.Request) {
	var c application.ReverifyCommand
	if err := decodeJSON(w, r, &c); err != nil {
		writeError(w, err)
		return
	}
	if err := bindProjectID(r.PathValue("id"), &c.Meta); err != nil {
		writeError(w, err)
		return
	}
	result, err := h.service.Reverify(r.Context(), c)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
func (h *Handler) WalkthroughHandler(w http.ResponseWriter, r *http.Request) {
	var c application.WalkthroughCommand
	if err := decodeJSON(w, r, &c); err != nil {
		writeError(w, err)
		return
	}
	if err := bindProjectID(r.PathValue("id"), &c.Meta); err != nil {
		writeError(w, err)
		return
	}
	result, err := h.service.Walkthrough(r.Context(), c)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
func (h *Handler) FreezePackageHandler(w http.ResponseWriter, r *http.Request) {
	var c application.FreezePackageCommand
	if err := decodeJSON(w, r, &c); err != nil {
		writeError(w, err)
		return
	}
	if err := bindProjectID(r.PathValue("id"), &c.Meta); err != nil {
		writeError(w, err)
		return
	}
	result, err := h.service.FreezePackage(r.Context(), c)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
func (h *Handler) ApprovalPreflightHandler(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.PreflightApproval(r.Context(), r.PathValue("id"), r.URL.Query().Get("approver_id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
