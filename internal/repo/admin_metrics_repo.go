package repo

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type AdminMetricsRepo struct{ db *pgxpool.Pool }

func NewAdminMetricsRepo(db *pgxpool.Pool) *AdminMetricsRepo { return &AdminMetricsRepo{db: db} }

func (r *AdminMetricsRepo) Metrics(ctx context.Context) (map[string]any, error) {
	var totalVendors, pendingVendors, totalProducts, publishedProducts, totalOrders, failedPayments int64
	var monthlyGMV, totalCommission, pendingPayoutAmount int64

	err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM vendors`).Scan(&totalVendors)
	if err != nil {
		return nil, err
	}

	_ = r.db.QueryRow(ctx, `SELECT COUNT(*) FROM vendors WHERE status='pending'`).Scan(&pendingVendors)
	_ = r.db.QueryRow(ctx, `SELECT COUNT(*) FROM products`).Scan(&totalProducts)
	_ = r.db.QueryRow(ctx, `SELECT COUNT(*) FROM products WHERE status='published'`).Scan(&publishedProducts)

	_ = r.db.QueryRow(ctx, `SELECT COUNT(*) FROM orders`).Scan(&totalOrders)
	_ = r.db.QueryRow(ctx, `SELECT COALESCE(SUM(total_amount),0) FROM orders WHERE created_at >= date_trunc('month', now())`).Scan(&monthlyGMV)

	_ = r.db.QueryRow(ctx, `SELECT COALESCE(SUM(commission_amount),0) FROM order_items`).Scan(&totalCommission)
	_ = r.db.QueryRow(ctx, `SELECT COALESCE(SUM(amount),0) FROM payouts WHERE status='pending'`).Scan(&pendingPayoutAmount)

	_ = r.db.QueryRow(ctx, `SELECT COUNT(*) FROM payments WHERE status='failed'`).Scan(&failedPayments)

	return map[string]any{
		"total_vendors":         totalVendors,
		"pending_vendors":       pendingVendors,
		"total_products":        totalProducts,
		"published_products":    publishedProducts,
		"total_orders":          totalOrders,
		"monthly_gmv":           monthlyGMV,
		"total_commission":      totalCommission,
		"pending_payout_amount": pendingPayoutAmount,
		"failed_payments":       failedPayments,
	}, nil
}
