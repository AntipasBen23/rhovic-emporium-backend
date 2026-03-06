package handlers

import (
	"net/http"
	"strconv"

	"rhovic/backend/internal/httpjson"
	"rhovic/backend/internal/middleware"
	"rhovic/backend/internal/services"

	"github.com/go-chi/chi/v5"
)

type VendorOrdersHandlers struct {
	checkout *services.CheckoutService
}

func NewVendorOrdersHandlers(checkout *services.CheckoutService) *VendorOrdersHandlers {
	return &VendorOrdersHandlers{checkout: checkout}
}

func (h *VendorOrdersHandlers) List(w http.ResponseWriter, r *http.Request) {
	u := middleware.MustAuth(r)
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	items, err := h.checkout.VendorListOrders(r.Context(), u.UserID, limit, offset)
	if err != nil {
		httpjson.Error(w, 400, "failed", err.Error())
		return
	}
	httpjson.Write(w, 200, map[string]any{"items": items})
}

func (h *VendorOrdersHandlers) Get(w http.ResponseWriter, r *http.Request) {
	u := middleware.MustAuth(r)
	id := chi.URLParam(r, "id")
	out, err := h.checkout.VendorGetOrder(r.Context(), u.UserID, id)
	if err != nil {
		httpjson.Error(w, 404, "not found", err.Error())
		return
	}
	httpjson.Write(w, 200, out)
}

func (h *VendorOrdersHandlers) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	u := middleware.MustAuth(r)
	id := chi.URLParam(r, "id")
	var req struct {
		Status string `json:"status"`
	}
	if err := httpjson.DecodeStrict(r, &req, 1_048_576); err != nil {
		httpjson.Error(w, 400, "bad request", err.Error())
		return
	}
	if err := h.checkout.VendorUpdateFulfillment(r.Context(), u.UserID, id, req.Status); err != nil {
		httpjson.Error(w, 400, "failed", err.Error())
		return
	}
	httpjson.Write(w, 200, map[string]any{"ok": true})
}
