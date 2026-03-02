package middleware

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type limiter struct {
	mu    sync.Mutex
	rpm   int
	hits  map[string][]time.Time
	scope string
}

func GlobalLimiter(rpm int) *limiter {
	return &limiter{rpm: rpm, hits: map[string][]time.Time{}, scope: "global"}
}

func PathLimiter(rpm int) *limiter {
	return &limiter{rpm: rpm, hits: map[string][]time.Time{}, scope: "path"}
}

func RateLimit(l *limiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := clientIP(r)
			if l.scope == "path" {
				key = key + ":" + r.URL.Path
			}

			now := time.Now()
			cutoff := now.Add(-1 * time.Minute)

			l.mu.Lock()
			times := l.hits[key]
			n := 0
			for _, t := range times {
				if t.After(cutoff) {
					times[n] = t
					n++
				}
			}
			times = times[:n]

			if len(times) >= l.rpm {
				l.mu.Unlock()
				w.WriteHeader(http.StatusTooManyRequests)
				return
			}

			times = append(times, now)
			l.hits[key] = times
			l.mu.Unlock()

			next.ServeHTTP(w, r)
		})
	}
}

func clientIP(r *http.Request) string {
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}