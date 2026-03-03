package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"rhovic/backend/internal/config"
	"rhovic/backend/internal/db"
	"rhovic/backend/internal/server"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
)

func main() {
	log.Println("Starting RHOVIC API server...")
	cfg := config.Load()

	ctx := context.Background()
	pool, err := db.NewPool(ctx, cfg.DBURL)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	r := chi.NewRouter()

	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Request-Id"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

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
