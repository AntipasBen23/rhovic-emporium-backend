package repo

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PayoutsRepo struct{ db *pgxpool.Pool }

func NewPayoutsRepo(db *pgxpool.Pool) *PayoutsRepo { return &PayoutsRepo{db: db} }

func (r *PayoutsRepo) Create(ctx context.Context, tx pgx.Tx, id, vendorID string, amount int64) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO payouts (id,vendor_id,amount,status) VALUES ($1,$2,$3,'pending')
	`, id, vendorID, amount)
	return err
}

func (r *PayoutsRepo) UpdateStatus(ctx context.Context, tx pgx.Tx, payoutID, status string, reason *string) error {
	_, err := tx.Exec(ctx, `
		UPDATE payouts SET status=$2, reason=$3, updated_at=now() WHERE id=$1
	`, payoutID, status, reason)
	return err
}

func (r *PayoutsRepo) List(ctx context.Context, status, vendorID string, limit, offset int) ([]map[string]any, error) {
	q := `
		SELECT id,vendor_id,amount,status,reason,created_at,updated_at
		FROM payouts
		WHERE ($1='' OR status=$1) AND ($2='' OR vendor_id=$2)
		ORDER BY created_at DESC
		LIMIT $3 OFFSET $4
	`
	rows, err := r.db.Query(ctx, q, status, vendorID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []map[string]any
	for rows.Next() {
		var id, vID, st string
		var amount int64
		var reason *string
		var cAt, uAt any
		if err := rows.Scan(&id, &vID, &amount, &st, &reason, &cAt, &uAt); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{
			"id": id, "vendor_id": vID, "amount": amount, "status": st,
			"reason": reason, "created_at": cAt, "updated_at": uAt,
		})
	}
	return out, nil
}