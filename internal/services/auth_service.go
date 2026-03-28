package services

import (
	"crypto/rand"
	"context"
	"errors"
	"log"
	"regexp"
	"strings"
	"time"

	"rhovic/backend/internal/domain"
	"rhovic/backend/internal/mailer"
	"rhovic/backend/internal/repo"
	"rhovic/backend/internal/util"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	users      *repo.UsersRepo
	refresh    *repo.RefreshTokensRepo
	resets     *repo.PasswordResetTokensRepo
	verifications *repo.EmailVerificationTokensRepo
	mailer     mailer.Sender
	jwtKey     []byte
	access     time.Duration
	refreshTTL time.Duration
}

type EmailVerificationStatus struct {
	Email     string
	OtpSentAt *time.Time
	ExpiresAt *time.Time
	Verified  bool
}

func NewAuthService(users *repo.UsersRepo, refresh *repo.RefreshTokensRepo, resets *repo.PasswordResetTokensRepo, verifications *repo.EmailVerificationTokensRepo, sender mailer.Sender, jwtSecret string, accessTTL, refreshTTL time.Duration) *AuthService {
	return &AuthService{
		users: users, refresh: refresh, resets: resets, verifications: verifications, mailer: sender,
		jwtKey: []byte(jwtSecret), access: accessTTL, refreshTTL: refreshTTL,
	}
}

func (s *AuthService) Register(ctx context.Context, email, password string, role domain.Role, vendor domain.VendorRegisterProfile) (string, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if !validEmail(email) || !validPassword(password) {
		return "", domain.ErrInvalidInput
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), 12)

	existing, err := s.users.GetByEmail(ctx, email)
	if err == nil {
		if existing.EmailVerifiedAt != nil {
			return "", domain.ErrConflict
		}
		if err := s.users.UpdatePasswordAndRole(ctx, existing.ID, string(hash), role); err != nil {
			return existing.ID, err
		}
		if err := s.queueVerificationOTP(ctx, existing.ID, email); err != nil {
			return existing.ID, err
		}
		return existing.ID, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}

	uid := util.NewID()
	err = s.users.Create(ctx, domain.User{ID: uid, Email: email, PasswordHash: string(hash), Role: role})
	if err != nil {
		return uid, err
	}

	// If registering as vendor, create the vendor profile row immediately.
	if role == domain.RoleVendor {
		if err := s.users.CreateVendorProfile(ctx, uid, vendor); err != nil {
			return uid, err
		}
	}
	if err := s.queueVerificationOTP(ctx, uid, email); err != nil {
		return uid, err
	}
	return uid, nil
}

func (s *AuthService) Login(ctx context.Context, email, password string) (string, string, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	u, err := s.users.GetByEmail(ctx, email)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) != nil {
		return "", "", domain.ErrUnauthorized
	}
	if u.EmailVerifiedAt == nil {
		return "", "", domain.ErrEmailUnverified
	}
	if err := s.users.UpdateLastLogin(ctx, u.ID); err != nil {
		return "", "", err
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

func (s *AuthService) ForgotPassword(ctx context.Context, email string) error {
	email = strings.TrimSpace(strings.ToLower(email))
	if !validEmail(email) {
		return domain.ErrInvalidInput
	}

	resetToken := util.NewID() + util.NewID()

	u, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	}

	tokenHash := util.SHA256Hex(resetToken)
	expiresAt := time.Now().Add(30 * time.Minute)
	if err := s.resets.Create(ctx, util.NewID(), u.ID, tokenHash, expiresAt); err != nil {
		return err
	}
	if s.mailer == nil {
		return errors.New("password reset email provider not configured")
	}
	if err := s.mailer.SendPasswordReset(ctx, email, resetToken); err != nil {
		log.Printf("WARN: password reset email send failed for %s: %v", email, err)
		// Prevent account enumeration by returning success even on send errors.
		return nil
	}
	return nil
}

func (s *AuthService) ResetPassword(ctx context.Context, token, newPassword string) error {
	if token == "" || !validPassword(newPassword) {
		return domain.ErrInvalidInput
	}
	tokenHash := util.SHA256Hex(token)
	userID, err := s.resets.Consume(ctx, tokenHash)
	if err != nil {
		return domain.ErrInvalidInput
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), 12)
	if err != nil {
		return err
	}
	return s.users.UpdatePassword(ctx, userID, string(hash))
}

func (s *AuthService) VerifyEmailOTP(ctx context.Context, email, code string) error {
	email = strings.TrimSpace(strings.ToLower(email))
	if !validEmail(email) || !validOTP(code) {
		return domain.ErrInvalidInput
	}
	u, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		return domain.ErrInvalidInput
	}
	if u.EmailVerifiedAt != nil {
		return nil
	}
	if err := s.verifications.Consume(ctx, u.ID, util.SHA256Hex(code)); err != nil {
		return domain.ErrInvalidInput
	}
	return s.users.MarkEmailVerified(ctx, u.ID)
}

func (s *AuthService) ResendEmailOTP(ctx context.Context, email string) error {
	email = strings.TrimSpace(strings.ToLower(email))
	if !validEmail(email) {
		return domain.ErrInvalidInput
	}
	u, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	}
	if u.EmailVerifiedAt != nil {
		return nil
	}
	return s.sendVerificationOTP(ctx, u.ID, u.Email)
}

func (s *AuthService) GetVerificationStatus(ctx context.Context, email string) (EmailVerificationStatus, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if !validEmail(email) {
		return EmailVerificationStatus{}, domain.ErrInvalidInput
	}
	u, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return EmailVerificationStatus{Email: email}, nil
		}
		return EmailVerificationStatus{}, err
	}
	status := EmailVerificationStatus{
		Email:    email,
		Verified: u.EmailVerifiedAt != nil,
	}
	if status.Verified {
		return status, nil
	}
	latest, err := s.verifications.GetLatestActiveForUser(ctx, u.ID)
	if err != nil {
		return EmailVerificationStatus{}, err
	}
	if latest != nil {
		status.OtpSentAt = &latest.SentAt
		status.ExpiresAt = &latest.ExpiresAt
	}
	return status, nil
}

func (s *AuthService) queueVerificationOTP(ctx context.Context, userID, email string) error {
	code, err := s.prepareVerificationOTP(ctx, userID)
	if err != nil {
		return err
	}
	if s.mailer == nil {
		return errors.New("email provider not configured")
	}
	go func(emailAddress, otpCode string) {
		sendCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := s.mailer.SendSignupOTP(sendCtx, emailAddress, otpCode); err != nil {
			log.Printf("WARN: async signup otp email send failed for %s: %v", emailAddress, err)
		}
	}(email, code)
	return nil
}

func (s *AuthService) sendVerificationOTP(ctx context.Context, userID, email string) error {
	code, err := s.prepareVerificationOTP(ctx, userID)
	if err != nil {
		return err
	}
	if s.mailer == nil {
		return errors.New("email provider not configured")
	}
	if err := s.mailer.SendSignupOTP(ctx, email, code); err != nil {
		log.Printf("WARN: signup otp email send failed for %s: %v", email, err)
		return domain.ErrEmailDeliveryFailed
	}
	return nil
}

func (s *AuthService) prepareVerificationOTP(ctx context.Context, userID string) (string, error) {
	if s.verifications == nil {
		return "", errors.New("email verification store not configured")
	}
	code, err := generateOTPCode(6)
	if err != nil {
		return "", err
	}
	if err := s.verifications.RevokeActiveForUser(ctx, userID); err != nil {
		return "", err
	}
	if err := s.verifications.Create(ctx, util.NewID(), userID, util.SHA256Hex(code), time.Now().Add(10*time.Minute)); err != nil {
		return "", err
	}
	return code, nil
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
var upperRe = regexp.MustCompile(`[A-Z]`)
var lowerRe = regexp.MustCompile(`[a-z]`)
var numberRe = regexp.MustCompile(`[0-9]`)
var specialRe = regexp.MustCompile(`[^A-Za-z0-9\s]`)

func validEmail(e string) bool { return emailRe.MatchString(e) }
func validPassword(p string) bool {
	return len(p) >= 8 &&
		upperRe.MatchString(p) &&
		lowerRe.MatchString(p) &&
		numberRe.MatchString(p) &&
		specialRe.MatchString(p)
}

func validOTP(code string) bool {
	matched, _ := regexp.MatchString(`^\d{6}$`, code)
	return matched
}

func generateOTPCode(length int) (string, error) {
	if length <= 0 {
		return "", domain.ErrInvalidInput
	}
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	out := make([]byte, length)
	for i, b := range bytes {
		out[i] = '0' + (b % 10)
	}
	return string(out), nil
}
