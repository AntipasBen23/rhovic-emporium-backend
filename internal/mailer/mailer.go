package mailer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Sender interface {
	SendPasswordReset(ctx context.Context, to, token string) error
	SendSignupOTP(ctx context.Context, to, code string) error
}

type Config struct {
	Provider          string
	FrontendURL       string
	ResendAPIKey      string
	ResendFromEmail   string
	SendGridAPIKey    string
	SendGridFromEmail string
}

type Client struct {
	cfg  Config
	http *http.Client
}

func New(cfg Config) *Client {
	return &Client{
		cfg: cfg,
		http: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *Client) SendPasswordReset(ctx context.Context, to, token string) error {
	provider := strings.ToLower(strings.TrimSpace(c.cfg.Provider))
	if provider == "" {
		return fmt.Errorf("email provider not configured")
	}
	if strings.TrimSpace(to) == "" || strings.TrimSpace(token) == "" {
		return fmt.Errorf("missing recipient or token")
	}

	resetURL := strings.TrimRight(c.cfg.FrontendURL, "/") + "/reset-password?token=" + url.QueryEscape(token)
	subject := "Reset your RHOVIC password"
	text := "Use this link to reset your RHOVIC password: " + resetURL + "\nThis link expires in 30 minutes."
	html := fmt.Sprintf(
		"<p>We received a password reset request for your RHOVIC account.</p><p><a href=\"%s\">Reset password</a></p><p>This link expires in 30 minutes.</p>",
		resetURL,
	)

	switch provider {
	case "resend":
		return c.sendResend(ctx, to, subject, text, html)
	case "sendgrid":
		return c.sendSendGrid(ctx, to, subject, text, html)
	default:
		return fmt.Errorf("unsupported email provider: %s", provider)
	}
}

func (c *Client) SendSignupOTP(ctx context.Context, to, code string) error {
	provider := strings.ToLower(strings.TrimSpace(c.cfg.Provider))
	if provider == "" {
		return fmt.Errorf("email provider not configured")
	}
	if strings.TrimSpace(to) == "" || strings.TrimSpace(code) == "" {
		return fmt.Errorf("missing recipient or code")
	}

	subject := "Verify your RHOVIC account"
	text := "Your RHOVIC verification code is: " + code + "\nThis code expires in 10 minutes."
	html := fmt.Sprintf(
		"<p>Welcome to RHOVIC.</p><p>Your verification code is <strong style=\"font-size:24px;letter-spacing:4px;\">%s</strong>.</p><p>This code expires in 10 minutes.</p>",
		code,
	)

	switch provider {
	case "resend":
		return c.sendResend(ctx, to, subject, text, html)
	case "sendgrid":
		return c.sendSendGrid(ctx, to, subject, text, html)
	default:
		return fmt.Errorf("unsupported email provider: %s", provider)
	}
}

func (c *Client) sendResend(ctx context.Context, to, subject, text, html string) error {
	if strings.TrimSpace(c.cfg.ResendAPIKey) == "" || strings.TrimSpace(c.cfg.ResendFromEmail) == "" {
		return fmt.Errorf("resend config incomplete")
	}
	body := map[string]any{
		"from":    c.cfg.ResendFromEmail,
		"to":      []string{to},
		"subject": subject,
		"text":    text,
		"html":    html,
	}
	return c.postJSON(
		ctx,
		"https://api.resend.com/emails",
		body,
		map[string]string{"Authorization": "Bearer " + c.cfg.ResendAPIKey},
		200, 299,
	)
}

func (c *Client) sendSendGrid(ctx context.Context, to, subject, text, html string) error {
	if strings.TrimSpace(c.cfg.SendGridAPIKey) == "" || strings.TrimSpace(c.cfg.SendGridFromEmail) == "" {
		return fmt.Errorf("sendgrid config incomplete")
	}
	body := map[string]any{
		"personalizations": []map[string]any{
			{
				"to": []map[string]string{{"email": to}},
			},
		},
		"from": map[string]string{
			"email": c.cfg.SendGridFromEmail,
		},
		"subject": subject,
		"content": []map[string]string{
			{"type": "text/plain", "value": text},
			{"type": "text/html", "value": html},
		},
	}
	return c.postJSON(
		ctx,
		"https://api.sendgrid.com/v3/mail/send",
		body,
		map[string]string{"Authorization": "Bearer " + c.cfg.SendGridAPIKey},
		202, 202,
	)
}

func (c *Client) postJSON(ctx context.Context, endpoint string, payload any, extraHeaders map[string]string, minStatus, maxStatus int) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range extraHeaders {
		req.Header.Set(k, v)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < minStatus || resp.StatusCode > maxStatus {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("email provider returned %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return nil
}
