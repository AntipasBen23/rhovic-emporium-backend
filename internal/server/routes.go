package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// RegisterRoutes mounts all route groups (public, auth, vendor, admin, etc).
// No business logic here. Only structure.
// Security middleware will be attached per-group in upcoming files.
func RegisterRoutes(r chi.Router) {
	// Root
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("RHOVIC API"))
	})

	// -------------------------
	// Auth (rate-limited HARD)
	// -------------------------
	r.Route("/auth", func(ar chi.Router) {
		// TODO (next files): attach auth-specific rate limiting + brute-force protection.
		ar.Post("/register", notImplemented("POST /auth/register"))
		ar.Post("/login", notImplemented("POST /auth/login"))
		ar.Post("/refresh", notImplemented("POST /auth/refresh"))
		ar.Post("/logout", notImplemented("POST /auth/logout"))
	})

	// -------------------------
	// Public
	// -------------------------
	r.Route("/", func(pr chi.Router) {
		pr.Get("/products", notImplemented("GET /products"))
		pr.Get("/products/{id}", notImplemented("GET /products/{id}"))
		pr.Get("/vendors/{id}", notImplemented("GET /vendors/{id}"))
	})

	// -------------------------
	// Checkout + Payments
	// -------------------------
	r.Route("/", func(or chi.Router) {
		// TODO: JWT required for checkout (buyer must be authenticated)
		or.Post("/orders/checkout", notImplemented("POST /orders/checkout"))

		// Webhook must be publicly reachable, but strictly verified (Paystack signature).
		or.Post("/payments/webhook", notImplemented("POST /payments/webhook"))
	})

	// -------------------------
	// Vendor (JWT + role=vendor)
	// -------------------------
	r.Route("/vendor", func(vr chi.Router) {
		// TODO (next files): attach JWT auth + role guard (vendor), plus rate limiting.
		vr.Post("/products", notImplemented("POST /vendor/products"))
		vr.Patch("/products/{id}", notImplemented("PATCH /vendor/products/{id}"))
		vr.Get("/orders", notImplemented("GET /vendor/orders"))
		vr.Get("/payouts", notImplemented("GET /vendor/payouts"))
		vr.Post("/payouts/request", notImplemented("POST /vendor/payouts/request"))
	})

	// -------------------------
	// Admin (JWT + role-based access)
	// -------------------------
	r.Route("/admin", func(ad chi.Router) {
		// TODO (next files):
		// - Admin-only JWT middleware (separate token audience/claims if desired)
		// - Role guards: super_admin / ops_admin / finance_admin
		// - Strong audit logging on every mutating endpoint
		ad.Get("/metrics", notImplemented("GET /admin/metrics"))

		ad.Get("/vendors", notImplemented("GET /admin/vendors"))
		ad.Get("/vendors/{id}", notImplemented("GET /admin/vendors/{id}"))
		ad.Patch("/vendors/{id}/approve", notImplemented("PATCH /admin/vendors/{id}/approve"))
		ad.Patch("/vendors/{id}/suspend", notImplemented("PATCH /admin/vendors/{id}/suspend"))
		ad.Patch("/vendors/{id}/plan", notImplemented("PATCH /admin/vendors/{id}/plan"))
		ad.Patch("/vendors/{id}/commission", notImplemented("PATCH /admin/vendors/{id}/commission"))

		ad.Get("/orders", notImplemented("GET /admin/orders"))
		ad.Get("/orders/{id}", notImplemented("GET /admin/orders/{id}"))

		ad.Get("/payments", notImplemented("GET /admin/payments"))
		ad.Get("/payments/{id}", notImplemented("GET /admin/payments/{id}"))

		ad.Get("/payouts", notImplemented("GET /admin/payouts"))
		ad.Patch("/payouts/{id}/approve", notImplemented("PATCH /admin/payouts/{id}/approve"))
		ad.Patch("/payouts/{id}/reject", notImplemented("PATCH /admin/payouts/{id}/reject"))

		ad.Get("/disputes", notImplemented("GET /admin/disputes"))
		ad.Patch("/disputes/{id}", notImplemented("PATCH /admin/disputes/{id}"))
		ad.Post("/orders/{id}/refund", notImplemented("POST /admin/orders/{id}/refund"))

		ad.Patch("/commission/default", notImplemented("PATCH /admin/commission/default"))
		ad.Get("/audit-logs", notImplemented("GET /admin/audit-logs"))
	})
}

func notImplemented(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotImplemented)
		w.Write([]byte(`{"error":"not implemented","route":"` + name + `"}`))
	}
}