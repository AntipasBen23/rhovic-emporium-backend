package services

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"rhovic/backend/internal/domain"
	"rhovic/backend/internal/repo"
	"rhovic/backend/internal/util"
)

type keyedLimiter struct {
	mu      sync.Mutex
	rpm     int
	hits    map[string][]time.Time
	maxKeys int
}

func newKeyedLimiter(rpm int) *keyedLimiter {
	return &keyedLimiter{
		rpm:     rpm,
		hits:    map[string][]time.Time{},
		maxKeys: 10000,
	}
}

func (l *keyedLimiter) Allow(key string) bool {
	if l == nil || l.rpm <= 0 || strings.TrimSpace(key) == "" {
		return true
	}

	now := time.Now()
	cutoff := now.Add(-1 * time.Minute)

	l.mu.Lock()
	defer l.mu.Unlock()

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
		l.hits[key] = times
		return false
	}
	times = append(times, now)
	l.hits[key] = times
	if len(l.hits) > l.maxKeys {
		for existingKey, values := range l.hits {
			if len(values) == 0 {
				delete(l.hits, existingKey)
			}
		}
	}
	return true
}

type AuthProtectionService struct {
	emailLimiter *keyedLimiter
	events       *repo.SecurityEventsRepo
	captcha      *CaptchaService
}

func NewAuthProtectionService(events *repo.SecurityEventsRepo, emailRPM int, captcha *CaptchaService) *AuthProtectionService {
	return &AuthProtectionService{
		emailLimiter: newKeyedLimiter(emailRPM),
		events:       events,
		captcha:      captcha,
	}
}

func (s *AuthProtectionService) CheckEmailAction(ctx context.Context, action, email, ipAddress, path string) error {
	key := strings.ToLower(strings.TrimSpace(email))
	if key == "" {
		return nil
	}
	if s.emailLimiter.Allow(action + ":" + key) {
		return nil
	}
	s.logEvent(ctx, action+"_rate_limited", key, key, "", ipAddress, path, map[string]any{
		"limiter": "email",
	})
	return domain.ErrTooMany
}

func (s *AuthProtectionService) VerifyCaptcha(ctx context.Context, action, token, email, ipAddress, path string) error {
	if s.captcha == nil || !s.captcha.Enabled() {
		return nil
	}
	if s.captcha.Verify(ctx, token, ipAddress) {
		return nil
	}
	s.logEvent(ctx, action+"_captcha_failed", strings.ToLower(strings.TrimSpace(email)), strings.ToLower(strings.TrimSpace(email)), "", ipAddress, path, nil)
	return domain.ErrCaptchaFailed
}

func (s *AuthProtectionService) LogLoginFailure(ctx context.Context, email, ipAddress, path string) {
	key := strings.ToLower(strings.TrimSpace(email))
	s.logEvent(ctx, "login_failed", key, key, "", ipAddress, path, nil)
}

func (s *AuthProtectionService) logEvent(ctx context.Context, eventType, principalKey, email, userID, ipAddress, path string, details map[string]any) {
	if s == nil || s.events == nil {
		return
	}
	if details == nil {
		details = map[string]any{}
	}
	data, err := json.Marshal(details)
	if err != nil {
		data = []byte(`{}`)
	}
	_ = s.events.Log(ctx, util.NewID(), eventType, principalKey, email, userID, ipAddress, path, string(data))
}
