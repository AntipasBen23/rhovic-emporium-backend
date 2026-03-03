package repo

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type RefreshTokensRepo struct{ db *pgxpool.Pool }

func NewRefreshTokensRepo(db *pgxpool.Pool) *RefreshTokensRepo { return &RefreshTokensRepo{db: db} }

func (r *RefreshTokensRepo) Create(ctx context.Context, id, userID, tokenHash, jti string, expiresAt time.Time) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO refresh_tokens (id,user_id,token_hash,jti,expires_at)
		VALUES ($1,$2,$3,$4,$5)
	`, id, userID, tokenHash, jti, expiresAt)
	return err
}

func (r *RefreshTokensRepo) IsValid(ctx context.Context, tokenHash, jti string) (bool, error) {
	var ok bool
	err := r.db.QueryRow(ctx, `
		SELECT EXISTS(
		  SELECT 1 FROM refresh_tokens
		  WHERE token_hash=$1 AND jti=$2 AND revoked_at IS NULL AND expires_at > now()
		)
	`, tokenHash, jti).Scan(&ok)
	return ok, err
}

func (r *RefreshTokensRepo) Revoke(ctx context.Context, tokenHash string) error {
	_, err := r.db.Exec(ctx, `UPDATE refresh_tokens SET revoked_at=now() WHERE token_hash=$1 AND revoked_at IS NULL`, tokenHash)
	return err
}