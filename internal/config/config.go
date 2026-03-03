package config

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Port string

	DBURL  string
	Env    string
	JWTKey string

	AccessTTL  time.Duration
	RefreshTTL time.Duration

	RateLimitRPM      int
	AuthRateLimitRPM  int
	MaxBodyBytes      int64
	PaystackSecretKey string
	PaystackPublicKey string
	BaseURL           string // used for callback URLs if needed
}

func Load() Config {
	envs := os.Environ()
	log.Printf("--- Environment Variable Debug (Count: %d) ---", len(envs))
	for _, e := range envs {
		pair := strings.SplitN(e, "=", 2)
		log.Printf("Found Key: [%s] (Length: %d)", pair[0], len(pair[1]))
	}
	log.Println("----------------------------------------------")

	c := Config{
		Port: getEnv("PORT", "8080"),

		DBURL:  getEnv("DATABASE_URL", getEnv("DB_URL", "")),
		Env:    getEnv("ENV", "dev"),
		JWTKey: getEnv("JWT_SECRET", getEnv("JWT_KEY", "")),

		AccessTTL:  getDurationSeconds("JWT_ACCESS_TTL_SECONDS", 900),      // 15m
		RefreshTTL: getDurationSeconds("JWT_REFRESH_TTL_SECONDS", 2592000), // 30d

		RateLimitRPM:     getInt("RATE_LIMIT_RPM", 240),
		AuthRateLimitRPM: getInt("AUTH_RATE_LIMIT_RPM", 30),

		MaxBodyBytes:      int64(getInt("MAX_BODY_BYTES", 1_048_576)), // 1MB
		PaystackSecretKey: getEnv("PAYSTACK_SECRET_KEY", ""),
		PaystackPublicKey: getEnv("PAYSTACK_PUBLIC_KEY", ""),
		BaseURL:           getEnv("BASE_URL", "http://localhost:8080"),
	}

	if c.DBURL == "" {
		log.Println("CRITICAL: Missing DATABASE_URL and DB_URL")
		panic("missing database url")
	}
	if c.JWTKey == "" {
		log.Println("CRITICAL: Missing JWT_SECRET and JWT_KEY")
		panic("missing jwt secret")
	}

	return c
}

func getEnv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func mustEnv(k string) string {
	v := os.Getenv(k)
	if v == "" {
		log.Printf("CRITICAL: Missing required environment variable: %s", k)
		panic("missing env var: " + k)
	}
	return v
}

func getInt(k string, def int) int {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func getDurationSeconds(k string, defSeconds int) time.Duration {
	return time.Duration(getInt(k, defSeconds)) * time.Second
}
