package handlers

import (
	"net/http"
	"os"
	"strconv"

	"rhovic/backend/internal/httpjson"
	"rhovic/backend/internal/middleware"
	"rhovic/backend/internal/repo"
	"rhovic/backend/internal/services"

	"github.com/go-chi/chi/v5"
)

type AdminHandlers struct {
	admin    *services.AdminService
	checkout *services.CheckoutService
	products *repo.ProductsRepo
	vendors  *repo.VendorsRepo
	payouts  *repo.PayoutsRepo
	disputes *repo.DisputesRepo
	logsDB   interface{} // minimal for list logs later
}

func NewAdminHandlers(admin *services.AdminService, checkout *services.CheckoutService, products *repo.ProductsRepo, vendors *repo.VendorsRepo, payouts *repo.PayoutsRepo, disputes *repo.DisputesRepo) *AdminHandlers {
	return &AdminHandlers{
		admin:    admin,
		checkout: checkout,
		products: products,
		vendors:  vendors,
		payouts:  payouts,
		disputes: disputes,
	}
}

func (h *AdminHandlers) Metrics(w http.ResponseWriter, r *http.Request) {
	out, err := h.admin.Metrics(r.Context())
	if err != nil {
		httpjson.Error(w, 500, "failed", err.Error())
		return
	}
	httpjson.Write(w, 200, out)
}

func (h *AdminHandlers) ListUsers(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	items, err := h.admin.ListUsers(r.Context(), limit, offset)
	if err != nil {
		httpjson.Error(w, 500, "failed", err.Error())
		return
	}
	httpjson.Write(w, 200, map[string]any{"items": items})
}

func (h *AdminHandlers) LogoutUser(w http.ResponseWriter, r *http.Request) {
	u := middleware.MustAuth(r)
	id := chi.URLParam(r, "id")
	if err := h.admin.LogoutUser(r.Context(), u.UserID, id); err != nil {
		httpjson.Error(w, 400, "failed", err.Error())
		return
	}
	httpjson.Write(w, 200, map[string]any{"ok": true})
}

func (h *AdminHandlers) DeleteUser(w http.ResponseWriter, r *http.Request) {
	u := middleware.MustAuth(r)
	id := chi.URLParam(r, "id")
	if err := h.admin.DeleteUser(r.Context(), u.UserID, id); err != nil {
		httpjson.Error(w, 400, "failed", err.Error())
		return
	}
	httpjson.Write(w, 200, map[string]any{"ok": true})
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

func (h *AdminHandlers) ApproveVendor(w http.ResponseWriter, r *http.Request) {
	u := middleware.MustAuth(r)
	id := chi.URLParam(r, "id")
	if err := h.admin.ApproveVendor(r.Context(), u.UserID, id); err != nil {
		httpjson.Error(w, 400, "failed", err.Error())
		return
	}
	httpjson.Write(w, 200, map[string]any{"ok": true})
}

func (h *AdminHandlers) RejectVendor(w http.ResponseWriter, r *http.Request) {
	u := middleware.MustAuth(r)
	id := chi.URLParam(r, "id")
	if err := h.admin.RejectVendor(r.Context(), u.UserID, id); err != nil {
		httpjson.Error(w, 400, "failed", err.Error())
		return
	}
	httpjson.Write(w, 200, map[string]any{"ok": true})
}

func (h *AdminHandlers) LogoutVendor(w http.ResponseWriter, r *http.Request) {
	u := middleware.MustAuth(r)
	id := chi.URLParam(r, "id")
	if err := h.admin.LogoutVendor(r.Context(), u.UserID, id); err != nil {
		httpjson.Error(w, 400, "failed", err.Error())
		return
	}
	httpjson.Write(w, 200, map[string]any{"ok": true})
}

func (h *AdminHandlers) DeleteVendor(w http.ResponseWriter, r *http.Request) {
	u := middleware.MustAuth(r)
	id := chi.URLParam(r, "id")
	if err := h.admin.DeleteVendor(r.Context(), u.UserID, id); err != nil {
		httpjson.Error(w, 400, "failed", err.Error())
		return
	}
	httpjson.Write(w, 200, map[string]any{"ok": true})
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

func (h *AdminHandlers) ListOrders(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	items, err := h.checkout.AdminListOrders(r.Context(), limit, offset)
	if err != nil {
		httpjson.Error(w, 500, "failed", err.Error())
		return
	}
	httpjson.Write(w, 200, map[string]any{"items": items})
}

func (h *AdminHandlers) GetOrder(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	out, err := h.checkout.AdminGetOrder(r.Context(), id)
	if err != nil {
		httpjson.Error(w, 404, "not found", err.Error())
		return
	}
	httpjson.Write(w, 200, out)
}

func (h *AdminHandlers) ListPendingPayments(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	items, err := h.checkout.AdminListPendingPayments(r.Context(), limit, offset)
	if err != nil {
		httpjson.Error(w, 500, "failed", err.Error())
		return
	}
	httpjson.Write(w, 200, map[string]any{"items": items})
}

func (h *AdminHandlers) ApproveOrderPayment(w http.ResponseWriter, r *http.Request) {
	u := middleware.MustAuth(r)
	id := chi.URLParam(r, "id")
	if err := h.checkout.AdminApprovePayment(r.Context(), u.UserID, u.Role, id, r.RemoteAddr); err != nil {
		httpjson.Error(w, 400, "failed", err.Error())
		return
	}
	httpjson.Write(w, 200, map[string]any{"ok": true})
}

func (h *AdminHandlers) RejectOrderPayment(w http.ResponseWriter, r *http.Request) {
	u := middleware.MustAuth(r)
	id := chi.URLParam(r, "id")
	var req struct {
		Reason string `json:"reason"`
	}
	_ = httpjson.DecodeStrict(r, &req, 1_048_576)
	if err := h.checkout.AdminRejectPayment(r.Context(), u.UserID, u.Role, id, req.Reason, r.RemoteAddr); err != nil {
		httpjson.Error(w, 400, "failed", err.Error())
		return
	}
	httpjson.Write(w, 200, map[string]any{"ok": true})
}

func (h *AdminHandlers) ListVendorPayouts(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	items, err := h.checkout.AdminListVendorPayouts(r.Context(), limit, offset)
	if err != nil {
		httpjson.Error(w, 500, "failed", err.Error())
		return
	}
	httpjson.Write(w, 200, map[string]any{"items": items})
}

func (h *AdminHandlers) MarkVendorPayoutPaid(w http.ResponseWriter, r *http.Request) {
	u := middleware.MustAuth(r)
	id := chi.URLParam(r, "id")
	var req struct {
		Reference string `json:"reference"`
	}
	if err := httpjson.DecodeStrict(r, &req, 1_048_576); err != nil {
		httpjson.Error(w, 400, "bad request", err.Error())
		return
	}
	if err := h.checkout.AdminMarkVendorPayoutPaid(r.Context(), u.UserID, u.Role, id, req.Reference, r.RemoteAddr); err != nil {
		httpjson.Error(w, 400, "failed", err.Error())
		return
	}
	httpjson.Write(w, 200, map[string]any{"ok": true})
}

func (h *AdminHandlers) DownloadPaymentProof(w http.ResponseWriter, r *http.Request) {
	proofID := chi.URLParam(r, "proofID")
	path, fileType, err := h.checkout.AdminGetPaymentProof(r.Context(), proofID)
	if err != nil {
		httpjson.Error(w, 404, "not found", err.Error())
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		httpjson.Error(w, 404, "not found", "proof file missing")
		return
	}
	w.Header().Set("Content-Type", fileType)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}
