package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"regexp"
	"time"

	"rhovic/backend/internal/domain"
	"rhovic/backend/internal/repo"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	users   *repo.UsersRepo
	jwtKey  []byte
	access  time.Duration
	refresh time.Duration
}

func NewAuthService(users *repo.UsersRepo, jwtSecret string, accessTTL, refreshTTL time.Duration) *AuthService {
	return &AuthService{users: users, jwtKey: []byte(jwtSecret), access: accessTTL, refresh: refreshTTL}
}

func (s *AuthService) Register(ctx context.Context, email, password string, role domain.Role) (string, error) {
	if !validEmail(email) || len(password) < 8 {
		return "", domain.ErrInvalidInput
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), 12)
	uid := newID()
	err := s.users.Create(ctx, domain.User{
		ID: uid, Email: email, PasswordHash: string(hash), Role: role,
	})
	return uid, err
}

func (s *AuthService) Login(ctx context.Context, email, password string) (accessToken, refreshToken string, err error) {
	u, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		return "", "", domain.ErrUnauthorized
	}
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) != nil {
		return "", "", domain.ErrUnauthorized
	}
	return s.issueTokens(u.ID, string(u.Role))
}

func (s *AuthService) issueTokens(userID, role string) (string, string, error) {
	accessJTI := newID()
	refreshJTI := newID()

	now := time.Now()

	access := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":  userID,
		"role": role,
		"jti":  accessJTI,
		"exp":  now.Add(s.access).Unix(),
		"iat":  now.Unix(),
	})
	at, err := access.SignedString(s.jwtKey)
	if err != nil {
		return "", "", err
	}

	refresh := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":  userID,
		"role": role,
		"jti":  refreshJTI,
		"typ":  "refresh",
		"exp":  now.Add(s.refresh).Unix(),
		"iat":  now.Unix(),
	})
	rt, err := refresh.SignedString(s.jwtKey)
	if err != nil {
		return "", "", err
	}
	return at, rt, nil
}

func newID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

var emailRe = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

func validEmail(e string) bool { return emailRe.MatchString(e) }