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
	"github.com/jackc/pgx/v5/pgxpool"
)

const Version = "1.0.5-FINAL-SURVIVAL"

func main() {
	log.Printf("Starting RHOVIC API server version %s...", Version)
	cfg := config.Load()

	ctx := context.Background()
	var pool *pgxpool.Pool
	var err error

	if cfg.DBURL != "" {
		pool, err = db.NewPool(ctx, cfg.DBURL)
		if err != nil {
			log.Printf("ERROR: Failed to connect to database: %v", err)
		} else {
			defer pool.Close()
			log.Println("SUCCESS: Connected to database.")
		}
	} else {
		log.Println("WARNING: Skipping database connection because DBURL is empty.")
	}

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
