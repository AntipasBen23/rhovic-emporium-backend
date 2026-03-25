package repo

import (
	"context"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type SecurityEventsRepo struct {
	db *pgxpool.Pool
}

func NewSecurityEventsRepo(db *pgxpool.Pool) *SecurityEventsRepo {
	return &SecurityEventsRepo{db: db}
}

type SecurityEventListItem struct {
	ID           string    `json:"id"`
	EventType    string    `json:"event_type"`
	PrincipalKey string    `json:"principal_key"`
	Email        string    `json:"email"`
	UserID       string    `json:"user_id"`
	IPAddress    string    `json:"ip_address"`
	Path         string    `json:"path"`
	DetailsJSON  string    `json:"details_json"`
	CreatedAt    time.Time `json:"created_at"`
}

type SecurityEventListResult struct {
	Items []SecurityEventListItem `json:"items"`
	Total int64                   `json:"total"`
}

func (r *SecurityEventsRepo) Log(ctx context.Context, id, eventType, principalKey, email, userID, ipAddress, path, detailsJSON string) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO security_events (
			id, event_type, principal_key, email, user_id, ip_address, path, details_json, created_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,now())
	`, id, eventType, principalKey, email, userID, ipAddress, path, detailsJSON)
	return err
}

func (r *SecurityEventsRepo) List(ctx context.Context, eventType, search string, limit, offset int) (SecurityEventListResult, error) {
	search = "%" + strings.TrimSpace(search) + "%"

	var total int64
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM security_events
		WHERE ($1 = '' OR event_type = $1)
		  AND ($2 = '%%' OR email ILIKE $2 OR principal_key ILIKE $2 OR ip_address ILIKE $2)
	`, strings.TrimSpace(eventType), search).Scan(&total)
	if err != nil {
		return SecurityEventListResult{}, err
	}

	rows, err := r.db.Query(ctx, `
		SELECT id, event_type, principal_key, email, user_id, ip_address, path, details_json, created_at
		FROM security_events
		WHERE ($1 = '' OR event_type = $1)
		  AND ($2 = '%%' OR email ILIKE $2 OR principal_key ILIKE $2 OR ip_address ILIKE $2)
		ORDER BY created_at DESC
		LIMIT $3 OFFSET $4
	`, strings.TrimSpace(eventType), search, limit, offset)
	if err != nil {
		return SecurityEventListResult{}, err
	}
	defer rows.Close()

	items := []SecurityEventListItem{}
	for rows.Next() {
		var item SecurityEventListItem
		if err := rows.Scan(&item.ID, &item.EventType, &item.PrincipalKey, &item.Email, &item.UserID, &item.IPAddress, &item.Path, &item.DetailsJSON, &item.CreatedAt); err != nil {
			return SecurityEventListResult{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return SecurityEventListResult{}, err
	}
	return SecurityEventListResult{Items: items, Total: total}, nil
}

func (r *SecurityEventsRepo) CountByTypesSince(ctx context.Context, eventTypes []string, since time.Time) (int64, error) {
	var count int64
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM security_events
		WHERE event_type = ANY($1)
		  AND created_at >= $2
	`, eventTypes, since).Scan(&count)
	return count, err
}

func (r *SecurityEventsRepo) Recent(ctx context.Context, limit int) ([]SecurityEventListItem, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, event_type, principal_key, email, user_id, ip_address, path, details_json, created_at
		FROM security_events
		ORDER BY created_at DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []SecurityEventListItem
	for rows.Next() {
		var item SecurityEventListItem
		if err := rows.Scan(&item.ID, &item.EventType, &item.PrincipalKey, &item.Email, &item.UserID, &item.IPAddress, &item.Path, &item.DetailsJSON, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *SecurityEventsRepo) CountLoginFailuresSinceLastSuccess(ctx context.Context, email string, since time.Time) (int64, error) {
	var count int64
	err := r.db.QueryRow(ctx, `
		WITH last_success AS (
			SELECT MAX(created_at) AS created_at
			FROM security_events
			WHERE email = $1
			  AND event_type = 'login_success'
		)
		SELECT COUNT(*)
		FROM security_events
		WHERE email = $1
		  AND event_type = 'login_failed'
		  AND created_at >= GREATEST(
			$2,
			COALESCE((SELECT created_at FROM last_success), $2)
		  )
	`, strings.ToLower(strings.TrimSpace(email)), since).Scan(&count)
	return count, err
}
