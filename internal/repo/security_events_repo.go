package repo

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type SecurityEventsRepo struct {
	db *pgxpool.Pool
}

func NewSecurityEventsRepo(db *pgxpool.Pool) *SecurityEventsRepo {
	return &SecurityEventsRepo{db: db}
}

func (r *SecurityEventsRepo) Log(ctx context.Context, id, eventType, principalKey, email, userID, ipAddress, path, detailsJSON string) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO security_events (
			id, event_type, principal_key, email, user_id, ip_address, path, details_json, created_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,now())
	`, id, eventType, principalKey, email, userID, ipAddress, path, detailsJSON)
	return err
}
