package httpui

import (
	"net/http"
	"wayfinding-release-gate/internal/application"
)

func (h *Handler) CreateProjectHandler(w http.ResponseWriter, r *http.Request) {
	var c application.CreateProjectCommand
	if err := decodeJSON(w, r, &c); err != nil {
		writeError(w, err)
		return
	}
	result, err := h.service.CreateProject(r.Context(), c)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}
func (h *Handler) FreezeBaselineHandler(w http.ResponseWriter, r *http.Request) {
	var c application.FreezeBaselineCommand
	if err := decodeJSON(w, r, &c); err != nil {
		writeError(w, err)
		return
	}
	if err := bindProjectID(r.PathValue("id"), &c.Meta); err != nil {
		writeError(w, err)
		return
	}
	result, err := h.service.FreezeBaseline(r.Context(), c)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
func (h *Handler) BaselinePreflightHandler(w http.ResponseWriter, r *http.Request) {
	var c application.BaselinePreflightCommand
	if err := decodeJSON(w, r, &c); err != nil {
		writeError(w, err)
		return
	}
	result, err := h.service.PreflightBaseline(r.Context(), r.PathValue("id"), c.Survey)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
func (h *Handler) ReplaceSignsHandler(w http.ResponseWriter, r *http.Request) {
	var c application.ReplaceSignsCommand
	if err := decodeJSON(w, r, &c); err != nil {
		writeError(w, err)
		return
	}
	if err := bindProjectID(r.PathValue("id"), &c.Meta); err != nil {
		writeError(w, err)
		return
	}
	result, err := h.service.ReplaceSigns(r.Context(), c)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
func (h *Handler) SignPreflightHandler(w http.ResponseWriter, r *http.Request) {
	var c application.SignPreflightCommand
	if err := decodeJSON(w, r, &c); err != nil {
		writeError(w, err)
		return
	}
	result, err := h.service.PreflightSigns(r.Context(), r.PathValue("id"), c.Signs)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
func (h *Handler) ValidateHandler(w http.ResponseWriter, r *http.Request) {
	var c application.ValidateCommand
	if err := decodeJSON(w, r, &c); err != nil {
		writeError(w, err)
		return
	}
	if err := bindProjectID(r.PathValue("id"), &c.Meta); err != nil {
		writeError(w, err)
		return
	}
	result, err := h.service.Validate(r.Context(), c)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
