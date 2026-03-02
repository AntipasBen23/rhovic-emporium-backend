package server

import (
	"rhovic/backend/internal/config"
	"rhovic/backend/internal/handlers"
	"rhovic/backend/internal/middleware"
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

	// repos
	usersRepo := repo.NewUsersRepo(d.DB)
	productsRepo := repo.NewProductsRepo(d.DB)

	// services
	authSvc := services.NewAuthService(usersRepo, d.Cfg.JWTKey, d.Cfg.AccessTTL, d.Cfg.RefreshTTL)
	productsSvc := services.NewProductsService(productsRepo)

	// handlers
	authH := handlers.NewAuthHandlers(authSvc, d.Cfg.MaxBodyBytes)
	pubH := handlers.NewPublicHandlers(productsSvc)
	webhookH := handlers.NewWebhookHandlers(d.Cfg.PaystackSecretKey)

	// AUTH (hard rate limit)
	r.Route("/auth", func(ar chi.Router) {
		middleware.ApplyAuthHardening(ar, d.Cfg.AuthRateLimitRPM)
		ar.Post("/register", authH.Register)
		ar.Post("/login", authH.Login)
	})

	// PUBLIC
	r.Get("/products", pubH.ListProducts)
	r.Get("/products/{id}", pubH.GetProduct)

	// PAYMENTS WEBHOOK (signature verified inside handler)
	r.Post("/payments/webhook", webhookH.PaystackWebhook)

	// VENDOR + ADMIN routes will be added next batch once checkout/payment processing is completed.
	// They need JWT + role middleware + admin audit logging.
}