package handlers

import (
	"net/http"
	"strconv"

	"rhovic/backend/internal/httpjson"
	"rhovic/backend/internal/middleware"
	"rhovic/backend/internal/services"

	"github.com/go-chi/chi/v5"
)

type CheckoutHandlers struct {
	checkout *services.CheckoutService
	maxBody  int64
}

func NewCheckoutHandlers(svc *services.CheckoutService, maxBody int64) *CheckoutHandlers {
	return &CheckoutHandlers{checkout: svc, maxBody: maxBody}
}

type checkoutReq struct {
	Items []services.CheckoutItem `json:"items"`
}

func (h *CheckoutHandlers) Checkout(w http.ResponseWriter, r *http.Request) {
	u := middleware.MustAuth(r)
	var req checkoutReq
	if err := httpjson.DecodeStrict(r, &req, h.maxBody); err != nil {
		httpjson.Error(w, 400, "bad request", err.Error())
		return
	}
	out, err := h.checkout.Checkout(r.Context(), u.UserID, services.CheckoutRequest{Items: req.Items})
	if err != nil {
		httpjson.Error(w, 400, "checkout failed", err.Error())
		return
	}
	httpjson.Write(w, 200, out)
}

func (h *CheckoutHandlers) ListMyOrders(w http.ResponseWriter, r *http.Request) {
	u := middleware.MustAuth(r)
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	items, err := h.checkout.ListMyOrders(r.Context(), u.UserID, limit, offset)
	if err != nil {
		httpjson.Error(w, 500, "failed", err.Error())
		return
	}
	httpjson.Write(w, 200, map[string]any{"items": items})
}

func (h *CheckoutHandlers) GetOrder(w http.ResponseWriter, r *http.Request) {
	u := middleware.MustAuth(r)
	id := chi.URLParam(r, "id")
	out, err := h.checkout.GetOrderForCustomer(r.Context(), u.UserID, id)
	if err != nil {
		httpjson.Error(w, 404, "not found", err.Error())
		return
	}
	httpjson.Write(w, 200, out)
}

func (h *CheckoutHandlers) UploadPaymentProof(w http.ResponseWriter, r *http.Request) {
	u := middleware.MustAuth(r)
	id := chi.URLParam(r, "id")
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		httpjson.Error(w, 400, "bad request", "invalid multipart form")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		httpjson.Error(w, 400, "bad request", "file is required")
		return
	}
	defer file.Close()

	out, err := h.checkout.UploadPaymentProof(r.Context(), u.UserID, id, file, header)
	if err != nil {
		httpjson.Error(w, 400, "upload failed", err.Error())
		return
	}
	httpjson.Write(w, 200, out)
}
