package domain

import "errors"

var (
	ErrNotFound       = errors.New("not found")
	ErrUnauthorized   = errors.New("unauthorized")
	ErrForbidden      = errors.New("forbidden")
	ErrInvalidInput   = errors.New("invalid input")
	ErrConflict       = errors.New("conflict")
	ErrTooMany        = errors.New("too many requests")
	ErrCaptchaFailed  = errors.New("captcha verification failed")
	ErrInsufficient   = errors.New("insufficient balance")
	ErrInvalidWebhook = errors.New("invalid webhook")
	ErrEmailUnverified = errors.New("email not verified")
	ErrEmailDeliveryFailed = errors.New("email delivery failed")
)
