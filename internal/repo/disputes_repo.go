package repo

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DisputesRepo struct{ db *pgxpool.Pool }

func NewDisputesRepo(db *pgxpool.Pool) *DisputesRepo { return &DisputesRepo{db: db} }

func (r *DisputesRepo) Create(ctx context.Context, tx pgx.Tx, id, orderID, openedBy string) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO disputes (id,order_id,opened_by,status) VALUES ($1,$2,$3,'open')
	`, id, orderID, openedBy)
	return err
}

func (r *DisputesRepo) Update(ctx context.Context, tx pgx.Tx, id, status string, notes, resolution string) error {
	_, err := tx.Exec(ctx, `
		UPDATE disputes
		SET status=$2, internal_notes=$3, resolution_notes=$4, updated_at=now()
		WHERE id=$1
	`, id, status, notes, resolution)
	return err
}

func (r *DisputesRepo) List(ctx context.Context, status string, limit, offset int) ([]map[string]any, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id,order_id,opened_by,status,internal_notes,resolution_notes,created_at,updated_at
		FROM disputes
		WHERE ($1='' OR status=$1)
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`, status, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []map[string]any
	for rows.Next() {
		var id, orderID, openedBy, st, notes, res string
		var cAt, uAt any
		if err := rows.Scan(&id, &orderID, &openedBy, &st, &notes, &res, &cAt, &uAt); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{
			"id": id, "order_id": orderID, "opened_by": openedBy, "status": st,
			"internal_notes": notes, "resolution_notes": res, "created_at": cAt, "updated_at": uAt,
		})
	}
	return out, nil
}