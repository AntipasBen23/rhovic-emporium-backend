package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type CaptchaService struct {
	provider  string
	secretKey string
	client    *http.Client
}

type captchaVerifyResponse struct {
	Success bool `json:"success"`
}

func NewCaptchaService(provider, secretKey string) *CaptchaService {
	return &CaptchaService{
		provider:  strings.TrimSpace(strings.ToLower(provider)),
		secretKey: strings.TrimSpace(secretKey),
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

func (s *CaptchaService) Enabled() bool {
	return s != nil && s.provider != "" && s.secretKey != ""
}

func (s *CaptchaService) Verify(ctx context.Context, token, remoteIP string) bool {
	if !s.Enabled() {
		return true
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return false
	}

	form := url.Values{}
	form.Set("secret", s.secretKey)
	form.Set("response", token)
	if strings.TrimSpace(remoteIP) != "" {
		form.Set("remoteip", strings.TrimSpace(remoteIP))
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.verifyURL(), strings.NewReader(form.Encode()))
	if err != nil {
		return false
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	var out captchaVerifyResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return false
	}
	return out.Success
}

func (s *CaptchaService) verifyURL() string {
	switch s.provider {
	case "turnstile":
		return "https://challenges.cloudflare.com/turnstile/v0/siteverify"
	case "hcaptcha":
		return "https://hcaptcha.com/siteverify"
	case "recaptcha":
		return "https://www.google.com/recaptcha/api/siteverify"
	default:
		return "https://challenges.cloudflare.com/turnstile/v0/siteverify"
	}
}
