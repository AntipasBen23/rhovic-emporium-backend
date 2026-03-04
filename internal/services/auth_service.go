package services

import (
	"context"
	"regexp"
	"time"

	"rhovic/backend/internal/domain"
	"rhovic/backend/internal/repo"
	"rhovic/backend/internal/util"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	users      *repo.UsersRepo
	refresh    *repo.RefreshTokensRepo
	jwtKey     []byte
	access     time.Duration
	refreshTTL time.Duration
}

func NewAuthService(users *repo.UsersRepo, refresh *repo.RefreshTokensRepo, jwtSecret string, accessTTL, refreshTTL time.Duration) *AuthService {
	return &AuthService{users: users, refresh: refresh, jwtKey: []byte(jwtSecret), access: accessTTL, refreshTTL: refreshTTL}
}

func (s *AuthService) Register(ctx context.Context, email, password string, role domain.Role, vendor domain.VendorRegisterProfile) (string, error) {
	if !validEmail(email) || len(password) < 8 {
		return "", domain.ErrInvalidInput
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), 12)
	uid := util.NewID()
	err := s.users.Create(ctx, domain.User{ID: uid, Email: email, PasswordHash: string(hash), Role: role})
	if err != nil {
		return uid, err
	}

	// If registering as vendor, create the vendor profile row immediately.
	if role == domain.RoleVendor {
		if err := s.users.CreateVendorProfile(ctx, uid, vendor); err != nil {
			return uid, err
		}
	}
	return uid, nil
}

func (s *AuthService) Login(ctx context.Context, email, password string) (string, string, error) {
	u, err := s.users.GetByEmail(ctx, email)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) != nil {
		return "", "", domain.ErrUnauthorized
	}
	return s.issueAndStoreRefresh(ctx, u.ID, string(u.Role))
}

func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (string, string, error) {
	claims, err := s.parse(refreshToken)
	if err != nil {
		return "", "", domain.ErrUnauthorized
	}
	if typ, _ := claims["typ"].(string); typ != "refresh" {
		return "", "", domain.ErrUnauthorized
	}
	sub, _ := claims["sub"].(string)
	role, _ := claims["role"].(string)
	jti, _ := claims["jti"].(string)
	if sub == "" || role == "" || jti == "" {
		return "", "", domain.ErrUnauthorized
	}

	hash := util.SHA256Hex(refreshToken)
	ok, err := s.refresh.IsValid(ctx, hash, jti)
	if err != nil || !ok {
		return "", "", domain.ErrUnauthorized
	}

	// rotate: revoke old, mint new
	_ = s.refresh.Revoke(ctx, hash)
	return s.issueAndStoreRefresh(ctx, sub, role)
}

func (s *AuthService) Logout(ctx context.Context, refreshToken string) error {
	if refreshToken == "" {
		return domain.ErrInvalidInput
	}
	hash := util.SHA256Hex(refreshToken)
	return s.refresh.Revoke(ctx, hash)
}

func (s *AuthService) issueAndStoreRefresh(ctx context.Context, userID, role string) (string, string, error) {
	now := time.Now()
	accessJTI := util.NewID()
	refreshJTI := util.NewID()

	at := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": userID, "role": role, "jti": accessJTI,
		"exp": now.Add(s.access).Unix(), "iat": now.Unix(),
	})
	accessToken, err := at.SignedString(s.jwtKey)
	if err != nil {
		return "", "", err
	}

	rt := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": userID, "role": role, "jti": refreshJTI, "typ": "refresh",
		"exp": now.Add(s.refreshTTL).Unix(), "iat": now.Unix(),
	})
	refreshToken, err := rt.SignedString(s.jwtKey)
	if err != nil {
		return "", "", err
	}

	hash := util.SHA256Hex(refreshToken)
	if err := s.refresh.Create(ctx, util.NewID(), userID, hash, refreshJTI, now.Add(s.refreshTTL)); err != nil {
		return "", "", err
	}
	return accessToken, refreshToken, nil
}

func (s *AuthService) parse(token string) (jwt.MapClaims, error) {
	tok, err := jwt.Parse(token, func(t *jwt.Token) (any, error) {
		return s.jwtKey, nil
	})
	if err != nil || !tok.Valid {
		return nil, domain.ErrUnauthorized
	}
	claims, ok := tok.Claims.(jwt.MapClaims)
	if !ok {
		return nil, domain.ErrUnauthorized
	}
	return claims, nil
}

var emailRe = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

func validEmail(e string) bool { return emailRe.MatchString(e) }
