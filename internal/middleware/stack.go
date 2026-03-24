package middleware

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

type StackOpts struct {
	GlobalRPM int
	AuthRPM   int
	UserRPM   int
}

func ApplyBase(r chi.Router, opts StackOpts) {
	r.Use(RequestID)
	r.Use(Recoverer)
	r.Use(SecurityHeaders)
	r.Use(RateLimit(GlobalLimiter(opts.GlobalRPM)))

	// basic DoS guard: limit body size handled in handlers decode
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			// refuse weird content types for mutating requests (best-effort)
			if req.Method == "POST" || req.Method == "PATCH" || req.Method == "PUT" {
				// allow empty (e.g. webhook raw), otherwise must be JSON
				ct := req.Header.Get("Content-Type")
				if strings.HasPrefix(ct, "multipart/form-data") {
					next.ServeHTTP(w, req)
					return
				}
				if ct != "" && ct != "application/json" && ct != "application/json; charset=utf-8" {
					w.WriteHeader(http.StatusUnsupportedMediaType)
					return
				}
			}
			next.ServeHTTP(w, req)
		})
	})
}

func ApplyAuthHardening(r chi.Router, authRPM int) {
	r.Use(RateLimit(PathLimiter(authRPM)))
}

func ApplyUserHardening(r chi.Router, userRPM int) {
	r.Use(RateLimit(UserLimiter(userRPM)))
}
