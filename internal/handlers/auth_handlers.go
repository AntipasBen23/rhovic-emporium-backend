package handlers

import (
	"errors"
	"net"
	"net/http"
	"strings"

	"rhovic/backend/internal/domain"
	"rhovic/backend/internal/httpjson"
	"rhovic/backend/internal/services"
)

type AuthHandlers struct {
	auth    *services.AuthService
	protect *services.AuthProtectionService
	maxBody int64
}

func NewAuthHandlers(auth *services.AuthService, protect *services.AuthProtectionService, maxBody int64) *AuthHandlers {
	return &AuthHandlers{auth: auth, protect: protect, maxBody: maxBody}
}

type registerReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Role     string `json:"role"`
	// Vendor profile fields (only used when role=vendor)
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name"`
	ShopName     string `json:"shop_name"`
	ShopURL      string `json:"shop_url"`
	Phone        string `json:"phone"`
	Street       string `json:"street"`
	Street2      string `json:"street2"`
	City         string `json:"city"`
	ZipCode      string `json:"zip_code"`
	Country      string `json:"country"`
	State        string `json:"state"`
	CompanyName  string `json:"company_name"`
	CompanyID    string `json:"company_id"`
	VatID        string `json:"vat_id"`
	BankName     string `json:"bank_name"`
	AccountIBAN  string `json:"account_iban"`
	CaptchaToken string `json:"captcha_token"`
}

func (h *AuthHandlers) Register(w http.ResponseWriter, r *http.Request) {
	var req registerReq
	if err := httpjson.DecodeStrict(r, &req, h.maxBody); err != nil {
		httpjson.Error(w, 400, "bad request", err.Error())
		return
	}
	role := domain.Role(req.Role)
	if role == "" {
		role = domain.RoleBuyer
	}
	if role == domain.RoleAdminSuper || role == domain.RoleAdminOps || role == domain.RoleAdminFin {
		httpjson.Error(w, 403, "forbidden", "cannot self-register as admin")
		return
	}
	ipAddress := requestIP(r)
	if err := h.protect.CheckEmailAction(r.Context(), "register", req.Email, ipAddress, r.URL.Path); err != nil {
		httpjson.Error(w, 429, "too many requests", "please try again later")
		return
	}
	if err := h.protect.VerifyCaptcha(r.Context(), "register", req.CaptchaToken, req.Email, ipAddress, r.URL.Path); err != nil {
		httpjson.Error(w, 403, "captcha required", err.Error())
		return
	}

	vendorProfile := domain.VendorRegisterProfile{
		FirstName:   req.FirstName,
		LastName:    req.LastName,
		ShopName:    req.ShopName,
		ShopURL:     req.ShopURL,
		Phone:       req.Phone,
		Street:      req.Street,
		Street2:     req.Street2,
		City:        req.City,
		ZipCode:     req.ZipCode,
		Country:     req.Country,
		State:       req.State,
		CompanyName: req.CompanyName,
		CompanyID:   req.CompanyID,
		VatID:       req.VatID,
		BankName:    req.BankName,
		AccountIBAN: req.AccountIBAN,
	}

	id, err := h.auth.Register(r.Context(), req.Email, req.Password, role, vendorProfile)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrConflict):
			httpjson.Error(w, 409, "account already exists", "An account with this email already exists. Log in or verify the email address to continue.")
		case errors.Is(err, domain.ErrEmailDeliveryFailed):
			httpjson.Error(w, 503, "verification email unavailable", "We created your account, but could not send the verification code right now. Please try resending the code shortly.")
		case errors.Is(err, domain.ErrInvalidInput):
			httpjson.Error(w, 400, "registration failed", "Please check your details and try again.")
		default:
			httpjson.Error(w, 400, "registration failed", "We could not complete sign up right now. Please try again.")
		}
		return
	}
	httpjson.Write(w, 201, map[string]any{
		"user_id": uidOrEmpty(id),
		"email": req.Email,
		"verification_required": true,
		"expires_in_minutes": 10,
	})
}

type loginReq struct {
	Email        string `json:"email"`
	Password     string `json:"password"`
	CaptchaToken string `json:"captcha_token"`
}

func (h *AuthHandlers) Login(w http.ResponseWriter, r *http.Request) {
	var req loginReq
	if err := httpjson.DecodeStrict(r, &req, h.maxBody); err != nil {
		httpjson.Error(w, 400, "bad request", err.Error())
		return
	}
	ipAddress := requestIP(r)
	if err := h.protect.CheckEmailAction(r.Context(), "login", req.Email, ipAddress, r.URL.Path); err != nil {
		httpjson.Error(w, 429, "too many requests", "please try again later")
		return
	}
	if err := h.protect.CheckLoginLock(r.Context(), req.Email, ipAddress, r.URL.Path); err != nil {
		httpjson.Error(w, 429, "too many requests", "too many failed login attempts. please wait and try again later")
		return
	}
	if err := h.protect.VerifyCaptcha(r.Context(), "login", req.CaptchaToken, req.Email, ipAddress, r.URL.Path); err != nil {
		httpjson.Error(w, 403, "captcha required", err.Error())
		return
	}
	at, rt, err := h.auth.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		if errors.Is(err, domain.ErrEmailUnverified) {
			httpjson.Error(w, 403, "email verification required", "please verify your email with the code we sent")
			return
		}
		h.protect.LogLoginFailure(r.Context(), req.Email, ipAddress, r.URL.Path)
		httpjson.Error(w, 401, "invalid credentials", "")
		return
	}
	h.protect.LogLoginSuccess(r.Context(), req.Email, "", ipAddress, r.URL.Path)
	setAuthCookies(w, r, at, rt)
	httpjson.Write(w, 200, map[string]any{"access_token": at, "refresh_token": rt})
}

type refreshReq struct {
	RefreshToken string `json:"refresh_token"`
}

func (h *AuthHandlers) Refresh(w http.ResponseWriter, r *http.Request) {
	var req refreshReq
	if err := httpjson.DecodeStrict(r, &req, h.maxBody); err != nil {
		// allow cookie-based refresh without a JSON body
		req.RefreshToken = ""
	}
	if strings.TrimSpace(req.RefreshToken) == "" {
		if c, err := r.Cookie("rhovic_refresh_token"); err == nil {
			req.RefreshToken = strings.TrimSpace(c.Value)
		}
	}
	if req.RefreshToken == "" {
		httpjson.Error(w, 400, "bad request", "missing refresh token")
		return
	}
	at, rt, err := h.auth.Refresh(r.Context(), req.RefreshToken)
	if err != nil {
		httpjson.Error(w, 401, "invalid refresh token", "")
		return
	}
	setAuthCookies(w, r, at, rt)
	httpjson.Write(w, 200, map[string]any{"access_token": at, "refresh_token": rt})
}

type logoutReq struct {
	RefreshToken string `json:"refresh_token"`
}

func (h *AuthHandlers) Logout(w http.ResponseWriter, r *http.Request) {
	var req logoutReq
	if err := httpjson.DecodeStrict(r, &req, h.maxBody); err != nil {
		// allow cookie-based logout without a JSON body
		req.RefreshToken = ""
	}
	if strings.TrimSpace(req.RefreshToken) == "" {
		if c, err := r.Cookie("rhovic_refresh_token"); err == nil {
			req.RefreshToken = strings.TrimSpace(c.Value)
		}
	}
	if req.RefreshToken == "" {
		httpjson.Error(w, 400, "bad request", "missing refresh token")
		return
	}
	if err := h.auth.Logout(r.Context(), req.RefreshToken); err != nil {
		httpjson.Error(w, 400, "logout failed", err.Error())
		return
	}
	clearAuthCookies(w, r)
	httpjson.Write(w, 200, map[string]any{"ok": true})
}

type forgotPasswordReq struct {
	Email        string `json:"email"`
	CaptchaToken string `json:"captcha_token"`
}

func (h *AuthHandlers) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	var req forgotPasswordReq
	if err := httpjson.DecodeStrict(r, &req, h.maxBody); err != nil {
		httpjson.Error(w, 400, "bad request", err.Error())
		return
	}
	ipAddress := requestIP(r)
	if err := h.protect.CheckEmailAction(r.Context(), "forgot_password", req.Email, ipAddress, r.URL.Path); err != nil {
		httpjson.Error(w, 429, "too many requests", "please try again later")
		return
	}
	if err := h.protect.VerifyCaptcha(r.Context(), "forgot_password", req.CaptchaToken, req.Email, ipAddress, r.URL.Path); err != nil {
		httpjson.Error(w, 403, "captcha required", err.Error())
		return
	}
	if err := h.auth.ForgotPassword(r.Context(), req.Email); err != nil {
		httpjson.Error(w, 400, "forgot password failed", err.Error())
		return
	}
	httpjson.Write(w, 200, map[string]any{"ok": true})
}

type resetPasswordReq struct {
	Token       string `json:"token"`
	NewPassword string `json:"new_password"`
}

func (h *AuthHandlers) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var req resetPasswordReq
	if err := httpjson.DecodeStrict(r, &req, h.maxBody); err != nil {
		httpjson.Error(w, 400, "bad request", err.Error())
		return
	}
	if err := h.auth.ResetPassword(r.Context(), req.Token, req.NewPassword); err != nil {
		httpjson.Error(w, 400, "reset password failed", err.Error())
		return
	}
	httpjson.Write(w, 200, map[string]any{"ok": true})
}

type verifyEmailReq struct {
	Email string `json:"email"`
	Code  string `json:"code"`
}

func (h *AuthHandlers) VerifyEmail(w http.ResponseWriter, r *http.Request) {
	var req verifyEmailReq
	if err := httpjson.DecodeStrict(r, &req, h.maxBody); err != nil {
		httpjson.Error(w, 400, "bad request", err.Error())
		return
	}
	if err := h.auth.VerifyEmailOTP(r.Context(), req.Email, req.Code); err != nil {
		httpjson.Error(w, 400, "verification failed", err.Error())
		return
	}
	httpjson.Write(w, 200, map[string]any{"ok": true})
}

type resendVerificationReq struct {
	Email        string `json:"email"`
	CaptchaToken string `json:"captcha_token"`
}

func (h *AuthHandlers) ResendVerification(w http.ResponseWriter, r *http.Request) {
	var req resendVerificationReq
	if err := httpjson.DecodeStrict(r, &req, h.maxBody); err != nil {
		httpjson.Error(w, 400, "bad request", err.Error())
		return
	}
	ipAddress := requestIP(r)
	if err := h.protect.CheckEmailAction(r.Context(), "resend_verification", req.Email, ipAddress, r.URL.Path); err != nil {
		httpjson.Error(w, 429, "too many requests", "please try again later")
		return
	}
	if err := h.protect.VerifyCaptcha(r.Context(), "resend_verification", req.CaptchaToken, req.Email, ipAddress, r.URL.Path); err != nil {
		httpjson.Error(w, 403, "captcha required", err.Error())
		return
	}
	if err := h.auth.ResendEmailOTP(r.Context(), req.Email); err != nil {
		httpjson.Error(w, 400, "resend failed", err.Error())
		return
	}
	httpjson.Write(w, 200, map[string]any{"ok": true, "expires_in_minutes": 10})
}

func cookieSecure(r *http.Request) bool {
	return r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

func cookieSameSite(r *http.Request) http.SameSite {
	if cookieSecure(r) {
		return http.SameSiteNoneMode
	}
	return http.SameSiteLaxMode
}

func setAuthCookies(w http.ResponseWriter, r *http.Request, access, refresh string) {
	secure := cookieSecure(r)
	sameSite := cookieSameSite(r)
	http.SetCookie(w, &http.Cookie{
		Name:     "rhovic_access_token",
		Value:    access,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: sameSite,
		MaxAge:   60 * 15,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     "rhovic_refresh_token",
		Value:    refresh,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: sameSite,
		MaxAge:   60 * 60 * 24 * 30,
	})
}

func clearAuthCookies(w http.ResponseWriter, r *http.Request) {
	secure := cookieSecure(r)
	sameSite := cookieSameSite(r)
	http.SetCookie(w, &http.Cookie{
		Name:     "rhovic_access_token",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: sameSite,
		MaxAge:   -1,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     "rhovic_refresh_token",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: sameSite,
		MaxAge:   -1,
	})
}

func requestIP(r *http.Request) string {
	for _, header := range []string{"CF-Connecting-IP", "X-Forwarded-For", "X-Real-IP"} {
		value := strings.TrimSpace(r.Header.Get(header))
		if value == "" {
			continue
		}
		if header == "X-Forwarded-For" {
			parts := strings.Split(value, ",")
			value = strings.TrimSpace(parts[0])
		}
		if net.ParseIP(value) != nil {
			return value
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

func uidOrEmpty(id string) string {
	return strings.TrimSpace(id)
}
