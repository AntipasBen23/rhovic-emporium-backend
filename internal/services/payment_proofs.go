package services

import (
	"net/http"
	"path/filepath"
	"strings"

	"rhovic/backend/internal/domain"
)

func SanitizeProofExt(contentType string) string {
	ct := strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	switch ct {
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "application/pdf":
		return ".pdf"
	default:
		return ""
	}
}

func DetectProofContentType(raw []byte) (string, string, error) {
	if len(raw) == 0 {
		return "", "", domain.ErrInvalidInput
	}
	detected := strings.ToLower(strings.TrimSpace(strings.Split(http.DetectContentType(raw[:min(512, len(raw))]), ";")[0]))
	ext := SanitizeProofExt(detected)
	if ext == "" {
		return "", "", domain.ErrInvalidInput
	}
	return detected, ext, nil
}

func ProofStoragePath(fileURL string) (string, error) {
	const prefix = "/files/payment-proofs/"
	if !strings.HasPrefix(fileURL, prefix) {
		return "", domain.ErrInvalidInput
	}
	base := filepath.Base(fileURL)
	if base == "." || base == "/" || strings.Contains(base, "..") {
		return "", domain.ErrInvalidInput
	}
	return filepath.Join("uploads", "payment_proofs", base), nil
}
