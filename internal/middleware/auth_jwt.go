package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

type AuthUser struct {
	UserID string
	Role   string
	TokenJTI string
}

const AuthUserKey ctxKey = "auth_user"

func JWTAuth(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := r.Header.Get("Authorization")
			if h == "" || !strings.HasPrefix(h, "Bearer ") {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			raw := strings.TrimPrefix(h, "Bearer ")

			tok, err := jwt.Parse(raw, func(t *jwt.Token) (any, error) {
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, jwt.ErrSignatureInvalid
				}
				return []byte(secret), nil
			})
			if err != nil || !tok.Valid {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			claims, ok := tok.Claims.(jwt.MapClaims)
			if !ok {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			uid, _ := claims["sub"].(string)
			role, _ := claims["role"].(string)
			jti, _ := claims["jti"].(string)
			if uid == "" || role == "" || jti == "" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), AuthUserKey, AuthUser{
				UserID: uid, Role: role, TokenJTI: jti,
			})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func MustAuth(r *http.Request) AuthUser {
	u, _ := r.Context().Value(AuthUserKey).(AuthUser)
	return u
}