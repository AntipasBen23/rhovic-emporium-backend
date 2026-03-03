package repo

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type AdminMetricsRepo struct{ db *pgxpool.Pool }

func NewAdminMetricsRepo(db *pgxpool.Pool) *AdminMetricsRepo { return &AdminMetricsRepo{db: db} }

func (r *AdminMetricsRepo) Metrics(ctx context.Context) (map[string]any, error) {
	out := map[string]any{}

	err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM vendors`).Scan(&out["total_vendors"])
	if err != nil { return nil, err }

	_ = r.db.QueryRow(ctx, `SELECT COUNT(*) FROM vendors WHERE status='pending'`).Scan(&out["pending_vendors"])
	_ = r.db.QueryRow(ctx, `SELECT COUNT(*) FROM products`).Scan(&out["total_products"])
	_ = r.db.QueryRow(ctx, `SELECT COUNT(*) FROM products WHERE status='published'`).Scan(&out["published_products"])

	_ = r.db.QueryRow(ctx, `SELECT COUNT(*) FROM orders`).Scan(&out["total_orders"])
	_ = r.db.QueryRow(ctx, `SELECT COALESCE(SUM(total_amount),0) FROM orders WHERE created_at >= date_trunc('month', now())`).Scan(&out["monthly_gmv"])

	_ = r.db.QueryRow(ctx, `SELECT COALESCE(SUM(commission_amount),0) FROM order_items`).Scan(&out["total_commission"])
	_ = r.db.QueryRow(ctx, `SELECT COALESCE(SUM(amount),0) FROM payouts WHERE status='pending'`).Scan(&out["pending_payout_amount"])

	_ = r.db.QueryRow(ctx, `SELECT COUNT(*) FROM payments WHERE status='failed'`).Scan(&out["failed_payments"])

	return out, nil
}