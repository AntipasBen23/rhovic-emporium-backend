package handlers

import (
	"net/http"
	"strconv"

	"rhovic/backend/internal/httpjson"
	"rhovic/backend/internal/middleware"
	"rhovic/backend/internal/repo"
)

type VendorOrdersHandlers struct {
	vendors *repo.VendorsRepo
	orders  *repo.VendorOrdersRepo
}

func NewVendorOrdersHandlers(vendors *repo.VendorsRepo, orders *repo.VendorOrdersRepo) *VendorOrdersHandlers {
	return &VendorOrdersHandlers{vendors: vendors, orders: orders}
}

func (h *VendorOrdersHandlers) List(w http.ResponseWriter, r *http.Request) {
	u := middleware.MustAuth(r)
	v, err := h.vendors.GetByUserID(r.Context(), u.UserID)
	if err != nil {
		httpjson.Error(w, 403, "forbidden", "")
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 || limit > 200 { limit = 50 }
	if offset < 0 { offset = 0 }

	items, err := h.orders.ListByVendor(r.Context(), v.ID, limit, offset)
	if err != nil {
		httpjson.Error(w, 500, "failed", err.Error())
		return
	}
	httpjson.Write(w, 200, map[string]any{"items": items})
}