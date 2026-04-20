package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"rhovic/backend/internal/domain"
	"rhovic/backend/internal/repo"
	"rhovic/backend/internal/util"
)

type VisitAnalyticsService struct {
	repo   *repo.VisitAnalyticsRepo
	users  *repo.UsersRepo
	client *http.Client
	jwtKey []byte
	geo    geoCache
}

type VisitTrackInput struct {
	Path      string `json:"path"`
	Referrer  string `json:"referrer"`
	UserAgent string `json:"user_agent"`
}

// TrackRequest holds everything extracted from an HTTP request before it closes.
// Build one with CaptureRequest, then pass to Track in a goroutine.
type TrackRequest struct {
	Input      VisitTrackInput
	IP         string
	AuthCookie string
}

type VisitGeo struct {
	Country string
	Region  string
	State   string
	City    string
}

type ipWhoResponse struct {
	Success bool   `json:"success"`
	Country string `json:"country"`
	Region  string `json:"region"`
	City    string `json:"city"`
}

type geoCacheEntry struct {
	geo       VisitGeo
	expiresAt time.Time
}

type geoCache struct {
	mu      sync.RWMutex
	entries map[string]geoCacheEntry
}

func (c *geoCache) get(ip string) (VisitGeo, bool) {
	c.mu.RLock()
	entry, ok := c.entries[ip]
	c.mu.RUnlock()
	if !ok || time.Now().After(entry.expiresAt) {
		return VisitGeo{}, false
	}
	return entry.geo, true
}

func (c *geoCache) set(ip string, geo VisitGeo) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.entries) >= 5000 {
		now := time.Now()
		for k, v := range c.entries {
			if now.After(v.expiresAt) {
				delete(c.entries, k)
			}
		}
		if len(c.entries) >= 5000 {
			c.entries = make(map[string]geoCacheEntry)
		}
	}
	c.entries[ip] = geoCacheEntry{geo: geo, expiresAt: time.Now().Add(6 * time.Hour)}
}

func NewVisitAnalyticsService(repo *repo.VisitAnalyticsRepo, users *repo.UsersRepo, jwtSecret string) *VisitAnalyticsService {
	return &VisitAnalyticsService{
		repo:   repo,
		users:  users,
		jwtKey: []byte(jwtSecret),
		client: &http.Client{Timeout: 3 * time.Second},
		geo:    geoCache{entries: make(map[string]geoCacheEntry)},
	}
}

// CaptureRequest extracts IP, user-agent, and auth cookie from r so tracking
// can run asynchronously after the handler has already responded.
func CaptureRequest(r *http.Request, input VisitTrackInput) TrackRequest {
	if input.UserAgent == "" {
		input.UserAgent = r.UserAgent()
	}
	var cookie string
	if c, err := r.Cookie("rhovic_access_token"); err == nil {
		cookie = strings.TrimSpace(c.Value)
	}
	return TrackRequest{
		Input:      input,
		IP:         clientIP(r),
		AuthCookie: cookie,
	}
}

func (s *VisitAnalyticsService) Track(ctx context.Context, tr TrackRequest) error {
	path := strings.TrimSpace(tr.Input.Path)
	if path == "" || !strings.HasPrefix(path, "/") {
		path = "/"
	}

	geo, ok := s.geo.get(tr.IP)
	if !ok {
		geo = s.lookupGeo(ctx, tr.IP)
		s.geo.set(tr.IP, geo)
	}

	userID, userEmail := s.identifyUser(ctx, tr.AuthCookie)

	event := repo.VisitEvent{
		ID:         util.NewID(),
		VisitorKey: visitorKey(tr.IP, tr.Input.UserAgent),
		UserID:     userID,
		UserEmail:  userEmail,
		Path:       path,
		Referrer:   strings.TrimSpace(tr.Input.Referrer),
		Country:    geo.Country,
		Region:     geo.Region,
		State:      firstNonEmpty(geo.State, geo.Region),
		City:       geo.City,
		UserAgent:  strings.TrimSpace(tr.Input.UserAgent),
		CreatedAt:  time.Now().UTC(),
	}
	return s.repo.Create(ctx, event)
}

func (s *VisitAnalyticsService) ListSessions(ctx context.Context, search, country string, limit, offset int) (repo.VisitorSessionListResult, error) {
	return s.repo.ListSessions(ctx, search, country, limit, offset)
}

func (s *VisitAnalyticsService) GetSession(ctx context.Context, visitorKey string) (repo.VisitorSessionDetail, error) {
	return s.repo.GetSession(ctx, visitorKey)
}

func (s *VisitAnalyticsService) lookupGeo(ctx context.Context, ip string) VisitGeo {
	if ip == "" || ip == "127.0.0.1" || ip == "::1" {
		return VisitGeo{}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("https://ipwho.is/%s?fields=success,country,region,city", ip), nil)
	if err != nil {
		return VisitGeo{}
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return VisitGeo{}
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return VisitGeo{}
	}
	var out ipWhoResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil || !out.Success {
		return VisitGeo{}
	}
	return VisitGeo{
		Country: strings.TrimSpace(out.Country),
		Region:  strings.TrimSpace(out.Region),
		State:   strings.TrimSpace(out.Region),
		City:    strings.TrimSpace(out.City),
	}
}

func clientIP(r *http.Request) string {
	for _, header := range []string{"CF-Connecting-IP", "X-Forwarded-For", "X-Real-IP"} {
		value := strings.TrimSpace(r.Header.Get(header))
		if value == "" {
			continue
		}
		if header == "X-Forwarded-For" {
			parts := strings.Split(value, ",")
			value = strings.TrimSpace(parts[0])
		}
		if ip := net.ParseIP(value); ip != nil {
			return ip.String()
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		if ip := net.ParseIP(host); ip != nil {
			return ip.String()
		}
		return host
	}
	return r.RemoteAddr
}

func visitorKey(ip, userAgent string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(ip) + "|" + strings.TrimSpace(userAgent)))
	return hex.EncodeToString(sum[:])
}

func (s *VisitAnalyticsService) identifyUser(ctx context.Context, cookieVal string) (*string, string) {
	if s.users == nil || len(s.jwtKey) == 0 || cookieVal == "" {
		return nil, ""
	}
	tok, err := jwt.Parse(cookieVal, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return s.jwtKey, nil
	})
	if err != nil || !tok.Valid {
		return nil, ""
	}
	claims, ok := tok.Claims.(jwt.MapClaims)
	if !ok {
		return nil, ""
	}
	sub, _ := claims["sub"].(string)
	if strings.TrimSpace(sub) == "" {
		return nil, ""
	}
	user, err := s.users.GetByID(ctx, sub)
	if err != nil {
		if err == domain.ErrNotFound {
			return nil, ""
		}
		return nil, ""
	}
	id := user.ID
	return &id, user.Email
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
