package handlers

import (
	"net/http"

	"rhovic/backend/internal/httpjson"
	"rhovic/backend/internal/middleware"
	"rhovic/backend/internal/services"
)

type CheckoutHandlers struct {
	checkout *services.CheckoutService
	maxBody  int64
}

func NewCheckoutHandlers(svc *services.CheckoutService, maxBody int64) *CheckoutHandlers {
	return &CheckoutHandlers{checkout: svc, maxBody: maxBody}
}

type checkoutReq struct {
	BuyerEmail string               `json:"buyer_email"`
	Items      []services.CheckoutItem `json:"items"`
}

func (h *CheckoutHandlers) Checkout(w http.ResponseWriter, r *http.Request) {
	u := middleware.MustAuth(r)

	var req checkoutReq
	if err := httpjson.DecodeStrict(r, &req, h.maxBody); err != nil {
		httpjson.Error(w, 400, "bad request", err.Error())
		return
	}

	idem := r.Header.Get("Idempotency-Key")
	out, err := h.checkout.Checkout(r.Context(), u.UserID, services.CheckoutRequest{
		BuyerEmail: req.BuyerEmail,
		Items:      req.Items,
		IdemKey:    idem,
	})
	if err != nil {
		httpjson.Error(w, 400, "checkout failed", err.Error())
		return
	}
	httpjson.Write(w, 200, out)
}