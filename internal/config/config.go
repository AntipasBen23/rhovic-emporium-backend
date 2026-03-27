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

	RateLimitRPM          int
	AuthRateLimitRPM      int
	AuthEmailRateLimitRPM int
	AuthUserRateLimitRPM  int
	MaxBodyBytes          int64
	PaystackSecretKey     string
	PaystackPublicKey     string
	BaseURL               string // used for callback URLs if needed
	CORSAllowedOrigins    []string
	FrontendURL           string
	EmailProvider         string
	SMTPHost              string
	SMTPPort              int
	SMTPUsername          string
	SMTPPassword          string
	SMTPFromEmail         string
	SMTPFromName          string
	ResendAPIKey          string
	ResendFromEmail       string
	SendGridAPIKey        string
	SendGridFromEmail     string
	CaptchaProvider       string
	CaptchaSecretKey      string
}

func Load() Config {

	c := Config{
		Port: getEnv("PORT", "8080"),

		DBURL:  getEnv("DATABASE_URL", getEnv("DB_URL", "")),
		Env:    getEnv("ENV", "dev"),
		JWTKey: getEnv("JWT_SECRET", getEnv("JWT_KEY", "")),

		AccessTTL:  getDurationSeconds("JWT_ACCESS_TTL_SECONDS", 900),      // 15m
		RefreshTTL: getDurationSeconds("JWT_REFRESH_TTL_SECONDS", 2592000), // 30d

		RateLimitRPM:          getInt("RATE_LIMIT_RPM", 240),
		AuthRateLimitRPM:      getInt("AUTH_RATE_LIMIT_RPM", 30),
		AuthEmailRateLimitRPM: getInt("AUTH_EMAIL_RATE_LIMIT_RPM", 8),
		AuthUserRateLimitRPM:  getInt("AUTH_USER_RATE_LIMIT_RPM", 120),

		MaxBodyBytes:      int64(getInt("MAX_BODY_BYTES", 1_048_576)), // 1MB
		PaystackSecretKey: getEnv("PAYSTACK_SECRET_KEY", ""),
		PaystackPublicKey: getEnv("PAYSTACK_PUBLIC_KEY", ""),
		BaseURL:           getEnv("BASE_URL", "http://localhost:8080"),
		FrontendURL:       getEnv("FRONTEND_URL", "http://localhost:3000"),
		EmailProvider:     strings.ToLower(getEnv("EMAIL_PROVIDER", "")),
		SMTPHost:          getEnv("SMTP_HOST", ""),
		SMTPPort:          getInt("SMTP_PORT", 587),
		SMTPUsername:      getEnv("SMTP_USERNAME", ""),
		SMTPPassword:      getEnv("SMTP_PASSWORD", ""),
		SMTPFromEmail:     getEnv("SMTP_FROM_EMAIL", ""),
		SMTPFromName:      getEnv("SMTP_FROM_NAME", "RHOVIC"),
		ResendAPIKey:      getEnv("RESEND_API_KEY", ""),
		ResendFromEmail:   getEnv("RESEND_FROM_EMAIL", ""),
		SendGridAPIKey:    getEnv("SENDGRID_API_KEY", ""),
		SendGridFromEmail: getEnv("SENDGRID_FROM_EMAIL", ""),
		CaptchaProvider:   strings.ToLower(getEnv("CAPTCHA_PROVIDER", "")),
		CaptchaSecretKey:  getEnv("CAPTCHA_SECRET_KEY", ""),
		CORSAllowedOrigins: getCSV("CORS_ALLOWED_ORIGINS", []string{
			"http://localhost:3000",
			"http://localhost:3001",
			"https://teal-souffle-5f6454.netlify.app",
			"https://rhovic-emporium-admin.netlify.app",
		}),
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

func getCSV(k string, def []string) []string {
	v := strings.TrimSpace(os.Getenv(k))
	if v == "" {
		return def
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return def
	}
	return out
}
