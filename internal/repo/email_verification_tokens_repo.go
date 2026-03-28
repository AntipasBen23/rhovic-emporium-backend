package repo

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type EmailVerificationTokensRepo struct{ db *pgxpool.Pool }

type VerificationStatus struct {
	SentAt    time.Time
	ExpiresAt time.Time
}

func NewEmailVerificationTokensRepo(db *pgxpool.Pool) *EmailVerificationTokensRepo {
	return &EmailVerificationTokensRepo{db: db}
}

func (r *EmailVerificationTokensRepo) Create(ctx context.Context, id, userID, codeHash string, expiresAt time.Time) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO email_verification_tokens (id, user_id, code_hash, expires_at)
		VALUES ($1, $2, $3, $4)
	`, id, userID, codeHash, expiresAt)
	return err
}

func (r *EmailVerificationTokensRepo) RevokeActiveForUser(ctx context.Context, userID string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE email_verification_tokens
		SET used_at = now()
		WHERE user_id = $1
		  AND used_at IS NULL
		  AND expires_at > now()
	`, userID)
	return err
}

func (r *EmailVerificationTokensRepo) Consume(ctx context.Context, userID, codeHash string) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE email_verification_tokens
		SET used_at = now()
		WHERE user_id = $1
		  AND code_hash = $2
		  AND used_at IS NULL
		  AND expires_at > now()
	`, userID, codeHash)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r *EmailVerificationTokensRepo) GetLatestActiveForUser(ctx context.Context, userID string) (*VerificationStatus, error) {
	var status VerificationStatus
	err := r.db.QueryRow(ctx, `
		SELECT created_at, expires_at
		FROM email_verification_tokens
		WHERE user_id = $1
		  AND used_at IS NULL
		  AND expires_at > now()
		ORDER BY created_at DESC
		LIMIT 1
	`, userID).Scan(&status.SentAt, &status.ExpiresAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &status, nil
}
