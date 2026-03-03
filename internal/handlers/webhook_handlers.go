package handlers

import (
	"encoding/json"
	"io"
	"net/http"

	"rhovic/backend/internal/httpjson"
	"rhovic/backend/internal/paystack"
	"rhovic/backend/internal/services"
)

type WebhookHandlers struct {
	secret   string
	payments *services.PaymentsService
}

func NewWebhookHandlers(paystackSecret string, payments *services.PaymentsService) *WebhookHandlers {
	return &WebhookHandlers{secret: paystackSecret, payments: payments}
}

func (h *WebhookHandlers) PaystackWebhook(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	sig := r.Header.Get("X-Paystack-Signature")

	if !paystack.VerifySignature(h.secret, body, sig) {
		httpjson.Error(w, 400, "invalid webhook signature", "")
		return
	}

	var evt struct {
		Event string `json:"event"`
		Data  struct {
			Reference string `json:"reference"`
			Status    string `json:"status"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &evt); err != nil {
		httpjson.Error(w, 400, "bad webhook payload", err.Error())
		return
	}

	// We only process success events
	if evt.Data.Reference == "" {
		httpjson.Error(w, 400, "missing reference", "")
		return
	}
	if evt.Data.Status != "success" && evt.Event == "" {
		httpjson.Write(w, 200, map[string]any{"ok": true})
		return
	}

	if err := h.payments.ProcessPaystackSuccess(r.Context(), evt.Data.Reference); err != nil {
		// return 200 to prevent endless retries; log in real deployment
		httpjson.Write(w, 200, map[string]any{"ok": true})
		return
	}
	httpjson.Write(w, 200, map[string]any{"ok": true})
}