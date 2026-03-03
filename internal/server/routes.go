package server

import (
	"net/http"

	"rhovic/backend/internal/config"
	"rhovic/backend/internal/handlers"
	"rhovic/backend/internal/middleware"
	"rhovic/backend/internal/paystack"
	"rhovic/backend/internal/repo"
	"rhovic/backend/internal/services"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Deps struct {
	Cfg config.Config
	DB  *pgxpool.Pool
}

func RegisterRoutes(r chi.Router, d Deps) {
	middleware.ApplyBase(r, middleware.StackOpts{
		GlobalRPM: d.Cfg.RateLimitRPM,
		AuthRPM:   d.Cfg.AuthRateLimitRPM,
	})

	// Health
	r.Get("/", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("RHOVIC API")) })

	// repos
	usersRepo := repo.NewUsersRepo(d.DB)
	refreshRepo := repo.NewRefreshTokensRepo(d.DB)
	productsRepo := repo.NewProductsRepo(d.DB)
	vendorsRepo := repo.NewVendorsRepo(d.DB)
	plansRepo := repo.NewPlansRepo(d.DB)
	settingsRepo := repo.NewSettingsRepo(d.DB)
	ordersRepo := repo.NewOrdersRepo()
	paymentsRepo := repo.NewPaymentsRepo()
	checkoutRepo := repo.NewCheckoutRepo()
	ledgerRepo := repo.NewLedgerRepo()
	vpRepo := repo.NewVendorProductsRepo()
	vendorOrdersRepo := repo.NewVendorOrdersRepo(d.DB)
	payoutsRepo := repo.NewPayoutsRepo(d.DB)
	disputesRepo := repo.NewDisputesRepo(d.DB)
	adminLogsRepo := repo.NewAdminLogsRepo()
	metricsRepo := repo.NewAdminMetricsRepo(d.DB)

	// external
	ps := paystack.New(d.Cfg.PaystackSecretKey)

	// services
	authSvc := services.NewAuthService(usersRepo, refreshRepo, d.Cfg.JWTKey, d.Cfg.AccessTTL, d.Cfg.RefreshTTL)
	productsSvc := services.NewProductsService(productsRepo)
	checkoutSvc := services.NewCheckoutService(d.DB, ordersRepo, paymentsRepo, ledgerRepo, checkoutRepo, settingsRepo, ps)
	paymentsSvc := services.NewPaymentsService(d.DB, ps, ledgerRepo, checkoutRepo)
	vendorSvc := services.NewVendorService(d.DB, vendorsRepo, plansRepo, vpRepo, payoutsRepo)
	adminSvc := services.NewAdminService(d.DB, metricsRepo, vendorsRepo, settingsRepo, payoutsRepo, disputesRepo, adminLogsRepo, ledgerRepo)

	// handlers
	authH := handlers.NewAuthHandlers(authSvc, d.Cfg.MaxBodyBytes)
	pubH := handlers.NewPublicHandlers(productsSvc)
	checkoutH := handlers.NewCheckoutHandlers(checkoutSvc, d.Cfg.MaxBodyBytes)
	webhookH := handlers.NewWebhookHandlers(d.Cfg.PaystackSecretKey, paymentsSvc)
	vendorH := handlers.NewVendorHandlers(vendorSvc, d.Cfg.MaxBodyBytes)
	vendorOrdersH := handlers.NewVendorOrdersHandlers(vendorsRepo, vendorOrdersRepo)
	adminH := handlers.NewAdminHandlers(adminSvc, vendorsRepo, payoutsRepo, disputesRepo)

	// AUTH (hard rate limit)
	r.Route("/auth", func(ar chi.Router) {
		middleware.ApplyAuthHardening(ar, d.Cfg.AuthRateLimitRPM)
		ar.Post("/register", authH.Register)
		ar.Post("/login", authH.Login)
		ar.Post("/refresh", authH.Refresh)
		ar.Post("/logout", authH.Logout)
	})

	// PUBLIC
	r.Get("/products", pubH.ListProducts)
	r.Get("/products/{id}", pubH.GetProduct)

	// CHECKOUT (buyer must be logged in)
	r.Route("/orders", func(or chi.Router) {
		or.Use(middleware.JWTAuth(d.Cfg.JWTKey))
		or.Use(middleware.RequireRole("buyer"))
		or.Post("/checkout", checkoutH.Checkout)
	})

	// PAYSTACK WEBHOOK (public, signature verified)
	r.Post("/payments/webhook", webhookH.PaystackWebhook)

	// VENDOR
	r.Route("/vendor", func(vr chi.Router) {
		vr.Use(middleware.JWTAuth(d.Cfg.JWTKey))
		vr.Use(middleware.RequireRole("vendor"))
		vr.Post("/products", vendorH.CreateProduct)
		vr.Patch("/products/{id}", vendorH.UpdateProduct)
		vr.Get("/orders", vendorOrdersH.List)
		vr.Post("/payouts/request", vendorH.RequestPayout)
	})

	// ADMIN
	r.Route("/admin", func(ad chi.Router) {
		ad.Use(middleware.JWTAuth(d.Cfg.JWTKey))
		ad.Use(middleware.RequireRole("super_admin", "ops_admin", "finance_admin"))

		ad.Get("/metrics", adminH.Metrics)
		ad.Get("/vendors", adminH.ListVendors)

		ad.Get("/payouts", adminH.ListPayouts)
		ad.Patch("/payouts/{id}/approve", adminH.ApprovePayout)
		ad.Patch("/payouts/{id}/reject", adminH.RejectPayout)

		ad.Get("/disputes", adminH.ListDisputes)
	})
}
