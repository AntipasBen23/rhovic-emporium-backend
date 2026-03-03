package repo

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PlansRepo struct{ db *pgxpool.Pool }

func NewPlansRepo(db *pgxpool.Pool) *PlansRepo { return &PlansRepo{db: db} }

func (r *PlansRepo) GetCommissionRate(ctx context.Context, planID string) (string, error) {
	var rate string
	err := r.db.QueryRow(ctx, `SELECT commission_rate::text FROM subscription_plans WHERE id=$1`, planID).Scan(&rate)
	return rate, err
}

func (r *PlansRepo) GetProductLimit(ctx context.Context, planID string) (int, error) {
	var lim int
	err := r.db.QueryRow(ctx, `SELECT product_limit FROM subscription_plans WHERE id=$1`, planID).Scan(&lim)
	return lim, err
}