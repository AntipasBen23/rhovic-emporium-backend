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
	// (Middlewares are applied in main.go before calling RegisterRoutes)

	// Health
	r.Get("/", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("RHOVIC API")) })

	// repos
	usersRepo := repo.NewUsersRepo(d.DB)
	refreshRepo := repo.NewRefreshTokensRepo(d.DB)
	resetRepo := repo.NewPasswordResetTokensRepo(d.DB)
	productsRepo := repo.NewProductsRepo(d.DB)
	vendorsRepo := repo.NewVendorsRepo(d.DB)
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
	categoriesRepo := repo.NewCategoriesRepo(d.DB)

	// external
	ps := paystack.New(d.Cfg.PaystackSecretKey)

	// services
	authSvc := services.NewAuthService(usersRepo, refreshRepo, resetRepo, d.Cfg.JWTKey, d.Cfg.AccessTTL, d.Cfg.RefreshTTL)
	productsSvc := services.NewProductsService(productsRepo)
	checkoutSvc := services.NewCheckoutService(d.DB, ordersRepo, paymentsRepo, ledgerRepo, checkoutRepo, settingsRepo, ps)
	paymentsSvc := services.NewPaymentsService(d.DB, ps, ledgerRepo, checkoutRepo)
	vendorSvc := services.NewVendorService(d.DB, vendorsRepo, vpRepo, payoutsRepo)
	adminSvc := services.NewAdminService(d.DB, metricsRepo, productsRepo, vendorsRepo, settingsRepo, payoutsRepo, disputesRepo, adminLogsRepo, ledgerRepo)

	// handlers
	authH := handlers.NewAuthHandlers(authSvc, d.Cfg.MaxBodyBytes)
	pubH := handlers.NewPublicHandlers(productsSvc, categoriesRepo)
	checkoutH := handlers.NewCheckoutHandlers(checkoutSvc, d.Cfg.MaxBodyBytes)
	webhookH := handlers.NewWebhookHandlers(d.Cfg.PaystackSecretKey, paymentsSvc)
	vendorH := handlers.NewVendorHandlers(vendorSvc, d.Cfg.MaxBodyBytes)
	vendorOrdersH := handlers.NewVendorOrdersHandlers(vendorsRepo, vendorOrdersRepo)
	adminH := handlers.NewAdminHandlers(adminSvc, productsRepo, vendorsRepo, payoutsRepo, disputesRepo)

	// AUTH (hard rate limit)
	r.Route("/auth", func(ar chi.Router) {
		middleware.ApplyAuthHardening(ar, d.Cfg.AuthRateLimitRPM)
		ar.Post("/register", authH.Register)
		ar.Post("/login", authH.Login)
		ar.Post("/refresh", authH.Refresh)
		ar.Post("/logout", authH.Logout)
		ar.Post("/forgot-password", authH.ForgotPassword)
		ar.Post("/reset-password", authH.ResetPassword)
	})

	// PUBLIC
	r.Get("/products", pubH.ListProducts)
	r.Get("/products/{id}", pubH.GetProduct)
	r.Get("/categories", pubH.ListCategories)

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
		vr.Get("/application", vendorH.Application)
		vr.Post("/apply", vendorH.Apply)
		vr.Post("/products", vendorH.CreateProduct)
		vr.Get("/products", vendorH.ListProducts)
		vr.Patch("/products/{id}", vendorH.UpdateProduct)
		vr.Delete("/products/{id}", vendorH.DeleteProduct)
		vr.Get("/orders", vendorOrdersH.List)
		vr.Post("/payouts/request", vendorH.RequestPayout)
	})

	// ADMIN
	r.Route("/admin", func(ad chi.Router) {
		ad.Use(middleware.JWTAuth(d.Cfg.JWTKey))
		ad.Use(middleware.RequireRole("super_admin", "ops_admin", "finance_admin"))

		ad.Get("/metrics", adminH.Metrics)
		ad.Get("/vendors", adminH.ListVendors)
		ad.Patch("/vendors/{id}/approve", adminH.ApproveVendor)
		ad.Patch("/vendors/{id}/reject", adminH.RejectVendor)

		ad.Get("/products", adminH.ListProducts)
		ad.Patch("/products/{id}/commission", adminH.UpdateProductCommission)

		ad.Get("/payouts", adminH.ListPayouts)
		ad.Patch("/payouts/{id}/approve", adminH.ApprovePayout)
		ad.Patch("/payouts/{id}/reject", adminH.RejectPayout)

		ad.Get("/disputes", adminH.ListDisputes)
	})
}
