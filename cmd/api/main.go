package main

import (
	"context"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"rhovic/backend/internal/config"
	"rhovic/backend/internal/db"
	"rhovic/backend/internal/middleware"
	"rhovic/backend/internal/server"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
)

const Version = "1.1.2"

func main() {
	log.Printf("Starting RHOVIC API server version %s...", Version)
	cfg := config.Load()

	ctx := context.Background()
	pool, err := db.NewPool(ctx, cfg.DBURL)
	if err != nil {
		log.Fatalf("CRITICAL: Failed to connect to database: %v", err)
	}
	defer pool.Close()
	log.Println("SUCCESS: Connected to database.")

	r := chi.NewRouter()

	// 1. GLOBAL MIDDLEWARE MUST BE FIRST
	r.Use(cors.Handler(cors.Options{
		AllowOriginFunc: func(r *http.Request, origin string) bool {
			if isAllowedOrigin(origin, cfg.CORSAllowedOrigins) {
				return true
			}
			// fallback for trusted Netlify deploy previews/custom domains
			u, err := url.Parse(origin)
			if err != nil || u.Host == "" {
				return false
			}
			host := strings.ToLower(u.Hostname())
			scheme := strings.ToLower(u.Scheme)
			return scheme == "https" && strings.HasSuffix(host, ".netlify.app")
		},
		AllowedMethods:   []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Request-Id"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	middleware.ApplyBase(r, middleware.StackOpts{
		GlobalRPM: cfg.RateLimitRPM,
		AuthRPM:   cfg.AuthRateLimitRPM,
		UserRPM:   cfg.AuthUserRateLimitRPM,
	})

	// 2. ROUTES SECOND
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	server.RegisterRoutes(r, server.Deps{Cfg: cfg, DB: pool})

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Printf("API running on :%s (env=%s)\n", cfg.Port, cfg.Env)
	log.Fatal(srv.ListenAndServe())
}

func isAllowedOrigin(origin string, allowed []string) bool {
	o := strings.TrimSpace(strings.ToLower(origin))
	if o == "" {
		return false
	}
	for _, a := range allowed {
		if o == strings.TrimSpace(strings.ToLower(a)) {
			return true
		}
	}
	return false
}
