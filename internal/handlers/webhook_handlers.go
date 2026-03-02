package handlers

import (
	"io"
	"net/http"

	"rhovic/backend/internal/httpjson"
	"rhovic/backend/internal/paystack"
)

type WebhookHandlers struct {
	secret string
}

func NewWebhookHandlers(paystackSecret string) *WebhookHandlers {
	return &WebhookHandlers{secret: paystackSecret}
}

func (h *WebhookHandlers) PaystackWebhook(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	sig := r.Header.Get("X-Paystack-Signature")

	if !paystack.VerifySignature(h.secret, body, sig) {
		httpjson.Error(w, 400, "invalid webhook signature", "")
		return
	}

	// TODO next: parse event, confirm transaction, idempotent processing.
	// We return 200 to stop retries once signature passes (but later we’ll process properly).
	httpjson.Write(w, 200, map[string]any{"ok": true})
}