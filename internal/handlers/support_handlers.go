package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"rhovic/backend/internal/domain"
	"rhovic/backend/internal/httpjson"
	"rhovic/backend/internal/middleware"
	"rhovic/backend/internal/services"

	"github.com/go-chi/chi/v5"
)

type SupportHandlers struct {
	support      *services.SupportService
	maxBodyBytes int64
}

func NewSupportHandlers(support *services.SupportService, maxBodyBytes int64) *SupportHandlers {
	return &SupportHandlers{support: support, maxBodyBytes: maxBodyBytes}
}

func (h *SupportHandlers) ListCustomerThreads(w http.ResponseWriter, r *http.Request) {
	u := middleware.MustAuth(r)
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	out, err := h.support.ListCustomerThreads(r.Context(), u.UserID, limit, offset)
	if err != nil {
		httpjson.Error(w, 500, "failed", err.Error())
		return
	}
	httpjson.Write(w, 200, out)
}

func (h *SupportHandlers) CreateCustomerThread(w http.ResponseWriter, r *http.Request) {
	u := middleware.MustAuth(r)
	var req struct {
		OrderID *string `json:"order_id"`
		Subject string  `json:"subject"`
		Message string  `json:"message"`
	}
	if err := httpjson.DecodeStrict(r, &req, h.maxBodyBytes); err != nil {
		httpjson.Error(w, 400, "invalid_request", err.Error())
		return
	}
	out, err := h.support.CreateThread(r.Context(), u.UserID, req.OrderID, req.Subject, req.Message)
	if err != nil {
		h.writeSupportError(w, err)
		return
	}
	httpjson.Write(w, 201, out)
}

func (h *SupportHandlers) GetCustomerThread(w http.ResponseWriter, r *http.Request) {
	u := middleware.MustAuth(r)
	out, err := h.support.GetCustomerThread(r.Context(), u.UserID, chi.URLParam(r, "id"))
	if err != nil {
		h.writeSupportError(w, err)
		return
	}
	httpjson.Write(w, 200, out)
}

func (h *SupportHandlers) AddCustomerMessage(w http.ResponseWriter, r *http.Request) {
	u := middleware.MustAuth(r)
	var req struct {
		Message string `json:"message"`
	}
	if err := httpjson.DecodeStrict(r, &req, h.maxBodyBytes); err != nil {
		httpjson.Error(w, 400, "invalid_request", err.Error())
		return
	}
	out, err := h.support.AddCustomerMessage(r.Context(), u.UserID, chi.URLParam(r, "id"), req.Message)
	if err != nil {
		h.writeSupportError(w, err)
		return
	}
	httpjson.Write(w, 200, out)
}

func (h *SupportHandlers) ListAdminThreads(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	out, err := h.support.ListAdminThreads(r.Context(), r.URL.Query().Get("status"), r.URL.Query().Get("search"), limit, offset)
	if err != nil {
		httpjson.Error(w, 500, "failed", err.Error())
		return
	}
	httpjson.Write(w, 200, out)
}

func (h *SupportHandlers) GetAdminThread(w http.ResponseWriter, r *http.Request) {
	out, err := h.support.GetAdminThread(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		h.writeSupportError(w, err)
		return
	}
	httpjson.Write(w, 200, out)
}

func (h *SupportHandlers) AddAdminMessage(w http.ResponseWriter, r *http.Request) {
	u := middleware.MustAuth(r)
	var req struct {
		Message string `json:"message"`
	}
	if err := httpjson.DecodeStrict(r, &req, h.maxBodyBytes); err != nil {
		httpjson.Error(w, 400, "invalid_request", err.Error())
		return
	}
	out, err := h.support.AddAdminMessage(r.Context(), u.UserID, chi.URLParam(r, "id"), req.Message)
	if err != nil {
		h.writeSupportError(w, err)
		return
	}
	httpjson.Write(w, 200, out)
}

func (h *SupportHandlers) CloseAdminThread(w http.ResponseWriter, r *http.Request) {
	u := middleware.MustAuth(r)
	if err := h.support.CloseThread(r.Context(), u.UserID, chi.URLParam(r, "id")); err != nil {
		h.writeSupportError(w, err)
		return
	}
	httpjson.Write(w, 200, map[string]any{"ok": true})
}

func (h *SupportHandlers) writeSupportError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrInvalidInput):
		httpjson.Error(w, 400, "invalid_request", err.Error())
	case errors.Is(err, domain.ErrForbidden):
		httpjson.Error(w, 403, "forbidden", err.Error())
	case errors.Is(err, domain.ErrConflict):
		httpjson.Error(w, 409, "conflict", "this conversation is already closed")
	case errors.Is(err, domain.ErrNotFound):
		httpjson.Error(w, 404, "not found", err.Error())
	default:
		httpjson.Error(w, 500, "failed", err.Error())
	}
}
