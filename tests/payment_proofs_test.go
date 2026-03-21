package tests

import (
	"testing"

	"rhovic/backend/internal/domain"
	"rhovic/backend/internal/services"
)

func TestDetectProofContentType_AllowsSupportedProofs(t *testing.T) {
	png := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0x00}
	contentType, ext, err := services.DetectProofContentType(png)
	if err != nil {
		t.Fatalf("expected PNG proof to be accepted, got error: %v", err)
	}
	if contentType != "image/png" || ext != ".png" {
		t.Fatalf("unexpected PNG detection result: %q %q", contentType, ext)
	}

	pdf := []byte("%PDF-1.4 sample")
	contentType, ext, err = services.DetectProofContentType(pdf)
	if err != nil {
		t.Fatalf("expected PDF proof to be accepted, got error: %v", err)
	}
	if contentType != "application/pdf" || ext != ".pdf" {
		t.Fatalf("unexpected PDF detection result: %q %q", contentType, ext)
	}
}

func TestDetectProofContentType_RejectsUnsupportedFiles(t *testing.T) {
	_, _, err := services.DetectProofContentType([]byte("<html>nope</html>"))
	if err != domain.ErrInvalidInput {
		t.Fatalf("expected invalid input for unsupported content, got %v", err)
	}
}

func TestProofStoragePath_BlocksInvalidPaths(t *testing.T) {
	got, err := services.ProofStoragePath("/files/payment-proofs/proof-123.png")
	if err != nil {
		t.Fatalf("expected valid proof path, got error: %v", err)
	}
	if got != "uploads\\payment_proofs\\proof-123.png" && got != "uploads/payment_proofs/proof-123.png" {
		t.Fatalf("unexpected proof storage path: %q", got)
	}

	if _, err := services.ProofStoragePath("/tmp/evil.png"); err != domain.ErrInvalidInput {
		t.Fatalf("expected invalid input for wrong prefix, got %v", err)
	}
}
