package middleware

import (
	"net"
	"net/http"
	"sync"
	"time"
)

type limiter struct {
	mu      sync.Mutex
	rpm     int
	hits    map[string][]time.Time
	scope   string
	maxKeys int
}

func GlobalLimiter(rpm int) *limiter {
	return &limiter{rpm: rpm, hits: map[string][]time.Time{}, scope: "global", maxKeys: 10000}
}

func PathLimiter(rpm int) *limiter {
	return &limiter{rpm: rpm, hits: map[string][]time.Time{}, scope: "path", maxKeys: 10000}
}

func UserLimiter(rpm int) *limiter {
	return &limiter{rpm: rpm, hits: map[string][]time.Time{}, scope: "user", maxKeys: 10000}
}

func RateLimit(l *limiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := clientIP(r)
			if l.scope == "path" {
				key = key + ":" + r.URL.Path
			} else if l.scope == "user" {
				if auth := MustAuth(r); auth.UserID != "" {
					key = "user:" + auth.UserID + ":" + r.URL.Path
				} else {
					key = key + ":" + r.URL.Path
				}
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
			if len(l.hits) > l.maxKeys {
				for k, v := range l.hits {
					if len(v) == 0 {
						delete(l.hits, k)
					}
				}
			}
			l.mu.Unlock()

			next.ServeHTTP(w, r)
		})
	}
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
