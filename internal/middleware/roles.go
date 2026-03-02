package middleware

import "net/http"

func RequireRole(allowed ...string) func(http.Handler) http.Handler {
	set := map[string]bool{}
	for _, a := range allowed {
		set[a] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			u := MustAuth(r)
			if u.UserID == "" || !set[u.Role] {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}