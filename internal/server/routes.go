package server

import (
	"net/http"

	"rhovic/backend/internal/config"
	"rhovic/backend/internal/handlers"
	"rhovic/backend/internal/mailer"
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
	securityEventsRepo := repo.NewSecurityEventsRepo(d.DB)
	productsRepo := repo.NewProductsRepo(d.DB)
	vendorsRepo := repo.NewVendorsRepo(d.DB)
	settingsRepo := repo.NewSettingsRepo(d.DB)
	checkoutRepo := repo.NewCheckoutRepo()
	ledgerRepo := repo.NewLedgerRepo()
	vpRepo := repo.NewVendorProductsRepo()
	payoutsRepo := repo.NewPayoutsRepo(d.DB)
	disputesRepo := repo.NewDisputesRepo(d.DB)
	adminLogsRepo := repo.NewAdminLogsRepo()
	metricsRepo := repo.NewAdminMetricsRepo(d.DB)
	categoriesRepo := repo.NewCategoriesRepo(d.DB)
	visitAnalyticsRepo := repo.NewVisitAnalyticsRepo(d.DB)

	// external
	ps := paystack.New(d.Cfg.PaystackSecretKey)
	mail := mailer.New(mailer.Config{
		Provider:          d.Cfg.EmailProvider,
		FrontendURL:       d.Cfg.FrontendURL,
		ResendAPIKey:      d.Cfg.ResendAPIKey,
		ResendFromEmail:   d.Cfg.ResendFromEmail,
		SendGridAPIKey:    d.Cfg.SendGridAPIKey,
		SendGridFromEmail: d.Cfg.SendGridFromEmail,
	})
	captchaSvc := services.NewCaptchaService(d.Cfg.CaptchaProvider, d.Cfg.CaptchaSecretKey)

	// services
	authSvc := services.NewAuthService(usersRepo, refreshRepo, resetRepo, mail, d.Cfg.JWTKey, d.Cfg.AccessTTL, d.Cfg.RefreshTTL)
	authProtectSvc := services.NewAuthProtectionService(securityEventsRepo, d.Cfg.AuthEmailRateLimitRPM, captchaSvc)
	productsSvc := services.NewProductsService(productsRepo)
	checkoutSvc := services.NewCheckoutService(d.DB, settingsRepo)
	paymentsSvc := services.NewPaymentsService(d.DB, ps, ledgerRepo, checkoutRepo)
	vendorSvc := services.NewVendorService(d.DB, vendorsRepo, vpRepo, payoutsRepo)
	adminSvc := services.NewAdminService(d.DB, metricsRepo, usersRepo, refreshRepo, securityEventsRepo, productsRepo, vendorsRepo, settingsRepo, payoutsRepo, disputesRepo, adminLogsRepo, ledgerRepo)
	visitAnalyticsSvc := services.NewVisitAnalyticsService(visitAnalyticsRepo)

	// handlers
	authH := handlers.NewAuthHandlers(authSvc, authProtectSvc, d.Cfg.MaxBodyBytes)
	pubH := handlers.NewPublicHandlers(productsSvc, categoriesRepo)
	checkoutH := handlers.NewCheckoutHandlers(checkoutSvc, d.Cfg.MaxBodyBytes)
	webhookH := handlers.NewWebhookHandlers(d.Cfg.PaystackSecretKey, paymentsSvc)
	vendorH := handlers.NewVendorHandlers(vendorSvc, d.Cfg.MaxBodyBytes)
	vendorOrdersH := handlers.NewVendorOrdersHandlers(checkoutSvc)
	adminH := handlers.NewAdminHandlers(adminSvc, checkoutSvc, productsRepo, vendorsRepo, payoutsRepo, disputesRepo)
	analyticsH := handlers.NewAnalyticsHandlers(visitAnalyticsSvc)

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
	r.Post("/analytics/visits", analyticsH.TrackVisit)

	// CHECKOUT / CUSTOMER ORDERS (buyer must be logged in)
	r.With(middleware.JWTAuth(d.Cfg.JWTKey), middleware.RateLimit(middleware.UserLimiter(d.Cfg.AuthUserRateLimitRPM)), middleware.RequireRole("buyer")).Post("/checkout", checkoutH.Checkout)
	r.Route("/orders", func(or chi.Router) {
		or.Use(middleware.JWTAuth(d.Cfg.JWTKey))
		middleware.ApplyUserHardening(or, d.Cfg.AuthUserRateLimitRPM)
		or.Use(middleware.RequireRole("buyer"))
		or.Post("/checkout", checkoutH.Checkout) // compatibility alias
		or.Get("/{id}", checkoutH.GetOrder)
		or.Post("/{id}/payment-proof", checkoutH.UploadPaymentProof)
		or.Get("/{id}/payment-proofs/{proofID}", checkoutH.DownloadPaymentProof)
	})
	r.With(middleware.JWTAuth(d.Cfg.JWTKey), middleware.RequireRole("buyer")).Get("/my-orders", checkoutH.ListMyOrders)

	// PAYSTACK WEBHOOK (public, signature verified)
	r.Post("/payments/webhook", webhookH.PaystackWebhook)

	// VENDOR
	r.Route("/vendor", func(vr chi.Router) {
		vr.Use(middleware.JWTAuth(d.Cfg.JWTKey))
		middleware.ApplyUserHardening(vr, d.Cfg.AuthUserRateLimitRPM)
		vr.Get("/application", vendorH.Application)
		vr.Post("/apply", vendorH.Apply)
		vr.Post("/products", vendorH.CreateProduct)
		vr.Get("/products", vendorH.ListProducts)
		vr.Patch("/products/{id}", vendorH.UpdateProduct)
		vr.Delete("/products/{id}", vendorH.DeleteProduct)
		vr.Get("/orders", vendorOrdersH.List)
		vr.Get("/orders/{id}", vendorOrdersH.Get)
		vr.Patch("/orders/{id}/status", vendorOrdersH.UpdateStatus)
		vr.Post("/payouts/request", vendorH.RequestPayout)
	})

	// ADMIN
	r.Route("/admin", func(ad chi.Router) {
		ad.Use(middleware.JWTAuth(d.Cfg.JWTKey))
		middleware.ApplyUserHardening(ad, d.Cfg.AuthUserRateLimitRPM)
		ad.Use(middleware.RequireRole("super_admin", "ops_admin", "finance_admin"))

		ad.Get("/metrics", adminH.Metrics)
		ad.Get("/users", adminH.ListUsers)
		ad.Get("/security-events", adminH.ListSecurityEvents)
		ad.Post("/users/{id}/logout", adminH.LogoutUser)
		ad.Delete("/users/{id}", adminH.DeleteUser)
		ad.Get("/vendors", adminH.ListVendors)
		ad.Patch("/vendors/{id}/approve", adminH.ApproveVendor)
		ad.Patch("/vendors/{id}/reject", adminH.RejectVendor)
		ad.Post("/vendors/{id}/logout", adminH.LogoutVendor)
		ad.Delete("/vendors/{id}", adminH.DeleteVendor)

		ad.Get("/products", adminH.ListProducts)
		ad.Patch("/products/{id}/commission", adminH.UpdateProductCommission)

		ad.Get("/payouts", adminH.ListPayouts)
		ad.Patch("/payouts/{id}/approve", adminH.ApprovePayout)
		ad.Patch("/payouts/{id}/reject", adminH.RejectPayout)

		ad.Get("/disputes", adminH.ListDisputes)

		// marketplace manual-payment order flows
		ad.Get("/orders", adminH.ListOrders)
		ad.Get("/orders/{id}", adminH.GetOrder)
		ad.Get("/payments/pending", adminH.ListPendingPayments)
		ad.Post("/orders/{id}/approve-payment", adminH.ApproveOrderPayment)
		ad.Post("/orders/{id}/reject-payment", adminH.RejectOrderPayment)
		ad.Get("/payment-proofs/{proofID}", adminH.DownloadPaymentProof)
		ad.Get("/vendor-payouts", adminH.ListVendorPayouts)
		ad.Post("/vendor-payouts/{id}/mark-paid", adminH.MarkVendorPayoutPaid)
	})
}
