package config

import (
	"os"
	"strconv"
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
	return Config{
		Port: getEnv("PORT", "8080"),

		DBURL:  mustEnv("DATABASE_URL"),
		Env:    getEnv("ENV", "dev"),
		JWTKey: mustEnv("JWT_SECRET"),

		AccessTTL:  getDurationSeconds("JWT_ACCESS_TTL_SECONDS", 900),     // 15m
		RefreshTTL: getDurationSeconds("JWT_REFRESH_TTL_SECONDS", 2592000), // 30d

		RateLimitRPM:     getInt("RATE_LIMIT_RPM", 240),
		AuthRateLimitRPM: getInt("AUTH_RATE_LIMIT_RPM", 30),

		MaxBodyBytes:      int64(getInt("MAX_BODY_BYTES", 1_048_576)), // 1MB
		PaystackSecretKey: getEnv("PAYSTACK_SECRET_KEY", ""),
		PaystackPublicKey: getEnv("PAYSTACK_PUBLIC_KEY", ""),
		BaseURL:           getEnv("BASE_URL", "http://localhost:8080"),
	}
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