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
	"time"

	"github.com/golang-jwt/jwt/v5"

	"rhovic/backend/internal/domain"
	"rhovic/backend/internal/repo"
	"rhovic/backend/internal/util"
)

type VisitAnalyticsService struct {
	repo    *repo.VisitAnalyticsRepo
	users   *repo.UsersRepo
	client  *http.Client
	jwtKey  []byte
}

type VisitTrackInput struct {
	Path      string `json:"path"`
	Referrer  string `json:"referrer"`
	UserAgent string `json:"user_agent"`
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

func NewVisitAnalyticsService(repo *repo.VisitAnalyticsRepo, users *repo.UsersRepo, jwtSecret string) *VisitAnalyticsService {
	return &VisitAnalyticsService{
		repo:  repo,
		users: users,
		jwtKey: []byte(jwtSecret),
		client: &http.Client{
			Timeout: 3 * time.Second,
		},
	}
}

func (s *VisitAnalyticsService) Track(ctx context.Context, r *http.Request, input VisitTrackInput) error {
	path := strings.TrimSpace(input.Path)
	if path == "" {
		path = "/"
	}

	if input.UserAgent == "" {
		input.UserAgent = r.UserAgent()
	}

	ip := clientIP(r)
	geo := s.lookupGeo(ctx, ip)
	userID, userEmail := s.identifyUser(ctx, r)
	event := repo.VisitEvent{
		ID:         util.NewID(),
		VisitorKey: visitorKey(ip, input.UserAgent),
		UserID:     userID,
		UserEmail:  userEmail,
		Path:       path,
		Referrer:   strings.TrimSpace(input.Referrer),
		Country:    geo.Country,
		Region:     geo.Region,
		State:      firstNonEmpty(geo.State, geo.Region),
		City:       geo.City,
		UserAgent:  strings.TrimSpace(input.UserAgent),
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

func (s *VisitAnalyticsService) identifyUser(ctx context.Context, r *http.Request) (*string, string) {
	if s.users == nil || len(s.jwtKey) == 0 {
		return nil, ""
	}
	cookie, err := r.Cookie("rhovic_access_token")
	if err != nil || strings.TrimSpace(cookie.Value) == "" {
		return nil, ""
	}
	tok, err := jwt.Parse(strings.TrimSpace(cookie.Value), func(t *jwt.Token) (any, error) {
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
