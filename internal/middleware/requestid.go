package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
)

type ctxKey string

const RequestIDKey ctxKey = "request_id"

func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rid := r.Header.Get("X-Request-Id")
		if rid == "" {
			b := make([]byte, 16)
			_, _ = rand.Read(b)
			rid = hex.EncodeToString(b)
		}
		w.Header().Set("X-Request-Id", rid)
		ctx := context.WithValue(r.Context(), RequestIDKey, rid)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}