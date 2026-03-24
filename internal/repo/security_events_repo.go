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
