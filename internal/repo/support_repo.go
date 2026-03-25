package repo

import (
	"context"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SupportRepo struct {
	db *pgxpool.Pool
}

type SupportThreadItem struct {
	ID             string     `json:"id"`
	CustomerID     string     `json:"customer_id"`
	CustomerEmail  string     `json:"customer_email,omitempty"`
	OrderID        *string    `json:"order_id,omitempty"`
	Subject        string     `json:"subject"`
	Status         string     `json:"status"`
	AssignedAdminID *string   `json:"assigned_admin_id,omitempty"`
	LastMessage    string     `json:"last_message"`
	LastMessageAt  time.Time  `json:"last_message_at"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	ClosedAt       *time.Time `json:"closed_at,omitempty"`
}

type SupportMessageItem struct {
	ID         string    `json:"id"`
	ThreadID   string    `json:"thread_id"`
	SenderID   string    `json:"sender_id"`
	SenderRole string    `json:"sender_role"`
	Message    string    `json:"message"`
	CreatedAt  time.Time `json:"created_at"`
}

type SupportThreadDetail struct {
	Thread   SupportThreadItem    `json:"thread"`
	Messages []SupportMessageItem `json:"messages"`
}

type SupportThreadListResult struct {
	Items []SupportThreadItem `json:"items"`
	Total int64               `json:"total"`
}

func NewSupportRepo(db *pgxpool.Pool) *SupportRepo {
	return &SupportRepo{db: db}
}

func (r *SupportRepo) CreateThread(ctx context.Context, tx pgx.Tx, id, customerID string, orderID *string, subject string) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO support_threads (id, customer_id, order_id, subject, status, created_at, updated_at)
		VALUES ($1,$2,$3,$4,'open',now(),now())
	`, id, customerID, orderID, subject)
	return err
}

func (r *SupportRepo) CreateMessage(ctx context.Context, tx pgx.Tx, id, threadID, senderID, senderRole, message string) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO support_messages (id, thread_id, sender_id, sender_role, message, created_at)
		VALUES ($1,$2,$3,$4,$5,now())
	`, id, threadID, senderID, senderRole, message)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		UPDATE support_threads
		SET updated_at = now(),
		    status = CASE WHEN status='closed' THEN status ELSE status END
		WHERE id=$1
	`, threadID)
	return err
}

func (r *SupportRepo) GetThreadForCustomer(ctx context.Context, customerID, threadID string) (SupportThreadDetail, error) {
	return r.getThread(ctx, `
		SELECT
			t.id, t.customer_id, u.email, t.order_id, t.subject, t.status, t.assigned_admin_id,
			COALESCE((
				SELECT m.message FROM support_messages m
				WHERE m.thread_id = t.id
				ORDER BY m.created_at DESC
				LIMIT 1
			), '') AS last_message,
			COALESCE((
				SELECT m.created_at FROM support_messages m
				WHERE m.thread_id = t.id
				ORDER BY m.created_at DESC
				LIMIT 1
			), t.created_at) AS last_message_at,
			t.created_at, t.updated_at, t.closed_at
		FROM support_threads t
		JOIN users u ON u.id = t.customer_id
		WHERE t.id = $1 AND t.customer_id = $2
	`, threadID, customerID)
}

func (r *SupportRepo) GetThreadForAdmin(ctx context.Context, threadID string) (SupportThreadDetail, error) {
	return r.getThread(ctx, `
		SELECT
			t.id, t.customer_id, u.email, t.order_id, t.subject, t.status, t.assigned_admin_id,
			COALESCE((
				SELECT m.message FROM support_messages m
				WHERE m.thread_id = t.id
				ORDER BY m.created_at DESC
				LIMIT 1
			), '') AS last_message,
			COALESCE((
				SELECT m.created_at FROM support_messages m
				WHERE m.thread_id = t.id
				ORDER BY m.created_at DESC
				LIMIT 1
			), t.created_at) AS last_message_at,
			t.created_at, t.updated_at, t.closed_at
		FROM support_threads t
		JOIN users u ON u.id = t.customer_id
		WHERE t.id = $1
	`, threadID)
}

func (r *SupportRepo) getThread(ctx context.Context, query string, args ...any) (SupportThreadDetail, error) {
	var detail SupportThreadDetail
	var thread SupportThreadItem
	err := r.db.QueryRow(ctx, query, args...).Scan(
		&thread.ID,
		&thread.CustomerID,
		&thread.CustomerEmail,
		&thread.OrderID,
		&thread.Subject,
		&thread.Status,
		&thread.AssignedAdminID,
		&thread.LastMessage,
		&thread.LastMessageAt,
		&thread.CreatedAt,
		&thread.UpdatedAt,
		&thread.ClosedAt,
	)
	if err != nil {
		return detail, err
	}

	rows, err := r.db.Query(ctx, `
		SELECT id, thread_id, sender_id, sender_role, message, created_at
		FROM support_messages
		WHERE thread_id = $1
		ORDER BY created_at ASC
	`, thread.ID)
	if err != nil {
		return detail, err
	}
	defer rows.Close()

	var messages []SupportMessageItem
	for rows.Next() {
		var item SupportMessageItem
		if err := rows.Scan(&item.ID, &item.ThreadID, &item.SenderID, &item.SenderRole, &item.Message, &item.CreatedAt); err != nil {
			return detail, err
		}
		messages = append(messages, item)
	}
	detail.Thread = thread
	detail.Messages = messages
	return detail, rows.Err()
}

func (r *SupportRepo) ListCustomerThreads(ctx context.Context, customerID string, limit, offset int) (SupportThreadListResult, error) {
	var total int64
	if err := r.db.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM support_threads
		WHERE customer_id = $1
	`, customerID).Scan(&total); err != nil {
		return SupportThreadListResult{}, err
	}

	rows, err := r.db.Query(ctx, `
		SELECT
			t.id, t.customer_id, u.email, t.order_id, t.subject, t.status, t.assigned_admin_id,
			COALESCE((
				SELECT m.message FROM support_messages m
				WHERE m.thread_id = t.id
				ORDER BY m.created_at DESC
				LIMIT 1
			), '') AS last_message,
			COALESCE((
				SELECT m.created_at FROM support_messages m
				WHERE m.thread_id = t.id
				ORDER BY m.created_at DESC
				LIMIT 1
			), t.created_at) AS last_message_at,
			t.created_at, t.updated_at, t.closed_at
		FROM support_threads t
		JOIN users u ON u.id = t.customer_id
		WHERE t.customer_id = $1
		ORDER BY last_message_at DESC
		LIMIT $2 OFFSET $3
	`, customerID, limit, offset)
	if err != nil {
		return SupportThreadListResult{}, err
	}
	defer rows.Close()
	items, err := scanThreads(rows)
	if err != nil {
		return SupportThreadListResult{}, err
	}
	return SupportThreadListResult{Items: items, Total: total}, nil
}

func (r *SupportRepo) ListAdminThreads(ctx context.Context, status, search string, limit, offset int) (SupportThreadListResult, error) {
	search = "%" + strings.TrimSpace(search) + "%"
	var total int64
	if err := r.db.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM support_threads t
		JOIN users u ON u.id = t.customer_id
		WHERE ($1 = '' OR t.status = $1)
		  AND ($2 = '%%' OR u.email ILIKE $2 OR t.subject ILIKE $2)
	`, status, search).Scan(&total); err != nil {
		return SupportThreadListResult{}, err
	}

	rows, err := r.db.Query(ctx, `
		SELECT
			t.id, t.customer_id, u.email, t.order_id, t.subject, t.status, t.assigned_admin_id,
			COALESCE((
				SELECT m.message FROM support_messages m
				WHERE m.thread_id = t.id
				ORDER BY m.created_at DESC
				LIMIT 1
			), '') AS last_message,
			COALESCE((
				SELECT m.created_at FROM support_messages m
				WHERE m.thread_id = t.id
				ORDER BY m.created_at DESC
				LIMIT 1
			), t.created_at) AS last_message_at,
			t.created_at, t.updated_at, t.closed_at
		FROM support_threads t
		JOIN users u ON u.id = t.customer_id
		WHERE ($1 = '' OR t.status = $1)
		  AND ($2 = '%%' OR u.email ILIKE $2 OR t.subject ILIKE $2)
		ORDER BY last_message_at DESC
		LIMIT $3 OFFSET $4
	`, status, search, limit, offset)
	if err != nil {
		return SupportThreadListResult{}, err
	}
	defer rows.Close()
	items, err := scanThreads(rows)
	if err != nil {
		return SupportThreadListResult{}, err
	}
	return SupportThreadListResult{Items: items, Total: total}, nil
}

func scanThreads(rows pgx.Rows) ([]SupportThreadItem, error) {
	var items []SupportThreadItem
	for rows.Next() {
		var item SupportThreadItem
		if err := rows.Scan(
			&item.ID,
			&item.CustomerID,
			&item.CustomerEmail,
			&item.OrderID,
			&item.Subject,
			&item.Status,
			&item.AssignedAdminID,
			&item.LastMessage,
			&item.LastMessageAt,
			&item.CreatedAt,
			&item.UpdatedAt,
			&item.ClosedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *SupportRepo) SetStatus(ctx context.Context, tx pgx.Tx, threadID, status string, assignedAdminID *string) error {
	_, err := tx.Exec(ctx, `
		UPDATE support_threads
		SET status = $2,
		    assigned_admin_id = COALESCE($3, assigned_admin_id),
		    updated_at = now(),
		    closed_at = CASE WHEN $2='closed' THEN now() ELSE NULL END
		WHERE id = $1
	`, threadID, status, assignedAdminID)
	return err
}

func (r *SupportRepo) GetStatus(ctx context.Context, threadID string) (string, error) {
	var status string
	err := r.db.QueryRow(ctx, `SELECT status FROM support_threads WHERE id=$1`, threadID).Scan(&status)
	return status, err
}
