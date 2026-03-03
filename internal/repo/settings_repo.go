package repo

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type SettingsRepo struct{ db *pgxpool.Pool }

func NewSettingsRepo(db *pgxpool.Pool) *SettingsRepo { return &SettingsRepo{db: db} }

func (r *SettingsRepo) Get(ctx context.Context, key string) (string, error) {
	var v string
	err := r.db.QueryRow(ctx, `SELECT value FROM settings WHERE key=$1`, key).Scan(&v)
	return v, err
}

func (r *SettingsRepo) Set(ctx context.Context, key, val string) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO settings (key,value,updated_at) VALUES ($1,$2,now())
		ON CONFLICT (key) DO UPDATE SET value=EXCLUDED.value, updated_at=now()
	`, key, val)
	return err
}