package repo

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PasswordResetTokensRepo struct{ db *pgxpool.Pool }

func NewPasswordResetTokensRepo(db *pgxpool.Pool) *PasswordResetTokensRepo {
	return &PasswordResetTokensRepo{db: db}
}

func (r *PasswordResetTokensRepo) Create(ctx context.Context, id, userID, tokenHash string, expiresAt time.Time) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO password_reset_tokens (id, user_id, token_hash, expires_at)
		VALUES ($1, $2, $3, $4)
	`, id, userID, tokenHash, expiresAt)
	return err
}

func (r *PasswordResetTokensRepo) Consume(ctx context.Context, tokenHash string) (string, error) {
	var userID string
	err := r.db.QueryRow(ctx, `
		UPDATE password_reset_tokens
		SET used_at = NOW()
		WHERE token_hash = $1
		  AND used_at IS NULL
		  AND expires_at > NOW()
		RETURNING user_id
	`, tokenHash).Scan(&userID)
	return userID, err
}
