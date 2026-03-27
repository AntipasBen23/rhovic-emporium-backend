package mailer

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/smtp"
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
	SMTPHost          string
	SMTPPort          int
	SMTPUsername      string
	SMTPPassword      string
	SMTPFromEmail     string
	SMTPFromName      string
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
	case "smtp":
		return c.sendSMTP(ctx, to, subject, text, html)
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
	case "smtp":
		return c.sendSMTP(ctx, to, subject, text, html)
	case "resend":
		return c.sendResend(ctx, to, subject, text, html)
	case "sendgrid":
		return c.sendSendGrid(ctx, to, subject, text, html)
	default:
		return fmt.Errorf("unsupported email provider: %s", provider)
	}
}

func (c *Client) sendSMTP(ctx context.Context, to, subject, text, html string) error {
	if strings.TrimSpace(c.cfg.SMTPHost) == "" ||
		c.cfg.SMTPPort <= 0 ||
		strings.TrimSpace(c.cfg.SMTPUsername) == "" ||
		strings.TrimSpace(c.cfg.SMTPPassword) == "" ||
		strings.TrimSpace(c.cfg.SMTPFromEmail) == "" {
		return fmt.Errorf("smtp config incomplete")
	}

	fromName := strings.TrimSpace(c.cfg.SMTPFromName)
	if fromName == "" {
		fromName = "RHOVIC"
	}
	fromHeader := fmt.Sprintf("%s <%s>", fromName, c.cfg.SMTPFromEmail)
	boundary := fmt.Sprintf("rhovic-boundary-%d", time.Now().UnixNano())
	var msg strings.Builder
	msg.WriteString(fmt.Sprintf("From: %s\r\n", fromHeader))
	msg.WriteString(fmt.Sprintf("To: %s\r\n", to))
	msg.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	msg.WriteString("MIME-Version: 1.0\r\n")
	msg.WriteString(fmt.Sprintf("Content-Type: multipart/alternative; boundary=\"%s\"\r\n", boundary))
	msg.WriteString("\r\n")
	msg.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	msg.WriteString("Content-Type: text/plain; charset=\"UTF-8\"\r\n\r\n")
	msg.WriteString(text)
	msg.WriteString("\r\n")
	msg.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	msg.WriteString("Content-Type: text/html; charset=\"UTF-8\"\r\n\r\n")
	msg.WriteString(html)
	msg.WriteString("\r\n")
	msg.WriteString(fmt.Sprintf("--%s--\r\n", boundary))

	address := fmt.Sprintf("%s:%d", c.cfg.SMTPHost, c.cfg.SMTPPort)
	auth := smtp.PlainAuth("", c.cfg.SMTPUsername, c.cfg.SMTPPassword, c.cfg.SMTPHost)

	errCh := make(chan error, 1)
	go func() {
		if c.cfg.SMTPPort == 465 {
			errCh <- c.sendSMTPTLS(address, auth, to, []byte(msg.String()))
			return
		}
		errCh <- smtp.SendMail(address, auth, c.cfg.SMTPFromEmail, []string{to}, []byte(msg.String()))
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errCh:
		return err
	}
}

func (c *Client) sendSMTPTLS(address string, auth smtp.Auth, to string, msg []byte) error {
	conn, err := tls.Dial("tcp", address, &tls.Config{
		ServerName: c.cfg.SMTPHost,
		MinVersion: tls.VersionTLS12,
	})
	if err != nil {
		return err
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, c.cfg.SMTPHost)
	if err != nil {
		return err
	}
	defer client.Close()

	if ok, _ := client.Extension("AUTH"); ok {
		if err := client.Auth(auth); err != nil {
			return err
		}
	}
	if err := client.Mail(c.cfg.SMTPFromEmail); err != nil {
		return err
	}
	if err := client.Rcpt(to); err != nil {
		return err
	}
	writer, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := writer.Write(msg); err != nil {
		_ = writer.Close()
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}
	return client.Quit()
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
