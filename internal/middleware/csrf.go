package middleware

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

const csrfHeader = "X-CSRF-Token"
const csrfCookie = "rhovic_csrf_token"

func CSRFCookieName() string { return csrfCookie }

func CSRFProtect() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodPost, http.MethodPatch, http.MethodPut, http.MethodDelete:
			default:
				next.ServeHTTP(w, r)
				return
			}

			if shouldSkipCSRF(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			if strings.HasPrefix(strings.TrimSpace(r.Header.Get("Authorization")), "Bearer ") {
				next.ServeHTTP(w, r)
				return
			}

			authCookie, authErr := r.Cookie("rhovic_access_token")
			refreshCookie, refreshErr := r.Cookie("rhovic_refresh_token")
			if (authErr != nil || strings.TrimSpace(authCookie.Value) == "") &&
				(refreshErr != nil || strings.TrimSpace(refreshCookie.Value) == "") {
				next.ServeHTTP(w, r)
				return
			}

			csrf, err := r.Cookie(csrfCookie)
			header := strings.TrimSpace(r.Header.Get(csrfHeader))
			if err != nil || strings.TrimSpace(csrf.Value) == "" || header == "" {
				http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
				return
			}
			if subtle.ConstantTimeCompare([]byte(csrf.Value), []byte(header)) != 1 {
				http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func shouldSkipCSRF(path string) bool {
	switch strings.TrimSpace(path) {
	case "/auth/login",
		"/auth/register",
		"/auth/forgot-password",
		"/auth/reset-password",
		"/auth/verify-email",
		"/auth/resend-verification":
		return true
	default:
		return false
	}
}
