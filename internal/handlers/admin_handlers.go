package handlers

import (
	"net/http"
	"strconv"

	"rhovic/backend/internal/httpjson"
	"rhovic/backend/internal/middleware"
	"rhovic/backend/internal/repo"
	"rhovic/backend/internal/services"

	"github.com/go-chi/chi/v5"
)

type AdminHandlers struct {
	admin    *services.AdminService
	products *repo.ProductsRepo
	vendors  *repo.VendorsRepo
	payouts  *repo.PayoutsRepo
	disputes *repo.DisputesRepo
	logsDB   interface{} // minimal for list logs later
}

func NewAdminHandlers(admin *services.AdminService, products *repo.ProductsRepo, vendors *repo.VendorsRepo, payouts *repo.PayoutsRepo, disputes *repo.DisputesRepo) *AdminHandlers {
	return &AdminHandlers{admin: admin, products: products, vendors: vendors, payouts: payouts, disputes: disputes}
}

func (h *AdminHandlers) Metrics(w http.ResponseWriter, r *http.Request) {
	out, err := h.admin.Metrics(r.Context())
	if err != nil {
		httpjson.Error(w, 500, "failed", err.Error())
		return
	}
	httpjson.Write(w, 200, out)
}

func (h *AdminHandlers) ListVendors(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	items, err := h.vendors.List(r.Context(), limit, offset)
	if err != nil {
		httpjson.Error(w, 500, "failed", err.Error())
		return
	}
	httpjson.Write(w, 200, map[string]any{"items": items})
}

func (h *AdminHandlers) ListProducts(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	items, err := h.admin.AdminListProducts(r.Context(), limit, offset)
	if err != nil {
		httpjson.Error(w, 500, "failed", err.Error())
		return
	}
	httpjson.Write(w, 200, map[string]any{"items": items})
}

func (h *AdminHandlers) UpdateProductCommission(w http.ResponseWriter, r *http.Request) {
	u := middleware.MustAuth(r)
	id := chi.URLParam(r, "id")

	var req struct {
		Rate *float64 `json:"rate"`
	}
	if err := httpjson.DecodeStrict(r, &req, 1_048_576); err != nil {
		httpjson.Error(w, 400, "invalid_request", err.Error())
		return
	}

	if err := h.admin.UpdateProductCommission(r.Context(), u.UserID, id, req.Rate); err != nil {
		httpjson.Error(w, 500, "failed", err.Error())
		return
	}
	httpjson.Write(w, 200, map[string]any{"ok": true})
}

func (h *AdminHandlers) ListPayouts(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	vendorID := r.URL.Query().Get("vendor_id")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	items, err := h.payouts.List(r.Context(), status, vendorID, limit, offset)
	if err != nil {
		httpjson.Error(w, 500, "failed", err.Error())
		return
	}
	httpjson.Write(w, 200, map[string]any{"items": items})
}

func (h *AdminHandlers) ApprovePayout(w http.ResponseWriter, r *http.Request) {
	u := middleware.MustAuth(r)
	id := chi.URLParam(r, "id")
	if err := h.admin.ApprovePayout(r.Context(), u.UserID, id); err != nil {
		httpjson.Error(w, 400, "failed", err.Error())
		return
	}
	httpjson.Write(w, 200, map[string]any{"ok": true})
}

func (h *AdminHandlers) RejectPayout(w http.ResponseWriter, r *http.Request) {
	u := middleware.MustAuth(r)
	id := chi.URLParam(r, "id")
	var req struct {
		Reason string `json:"reason"`
	}
	_ = httpjson.DecodeStrict(r, &req, 1_048_576)
	if req.Reason == "" {
		req.Reason = "rejected"
	}
	if err := h.admin.RejectPayout(r.Context(), u.UserID, id, req.Reason); err != nil {
		httpjson.Error(w, 400, "failed", err.Error())
		return
	}
	httpjson.Write(w, 200, map[string]any{"ok": true})
}

func (h *AdminHandlers) ListDisputes(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	items, err := h.disputes.List(r.Context(), status, limit, offset)
	if err != nil {
		httpjson.Error(w, 500, "failed", err.Error())
		return
	}
	httpjson.Write(w, 200, map[string]any{"items": items})
}
