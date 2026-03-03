package repo

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type VendorOrdersRepo struct{ db *pgxpool.Pool }

func NewVendorOrdersRepo(db *pgxpool.Pool) *VendorOrdersRepo { return &VendorOrdersRepo{db: db} }

func (r *VendorOrdersRepo) ListByVendor(ctx context.Context, vendorID string, limit, offset int) ([]map[string]any, error) {
	rows, err := r.db.Query(ctx, `
		SELECT
		  o.id, o.status, o.total_amount, o.created_at,
		  oi.id, oi.product_id, oi.quantity::text, oi.unit_price, oi.subtotal, oi.commission_amount
		FROM order_items oi
		JOIN orders o ON o.id = oi.order_id
		WHERE oi.vendor_id=$1
		ORDER BY o.created_at DESC
		LIMIT $2 OFFSET $3
	`, vendorID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []map[string]any
	for rows.Next() {
		var orderID, orderStatus, itemID, productID, qty string
		var total, unitPrice, subtotal, commission int64
		var createdAt any
		if err := rows.Scan(&orderID, &orderStatus, &total, &createdAt, &itemID, &productID, &qty, &unitPrice, &subtotal, &commission); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{
			"order_id": orderID, "order_status": orderStatus, "order_total": total, "created_at": createdAt,
			"item_id": itemID, "product_id": productID, "quantity": qty, "unit_price": unitPrice,
			"subtotal": subtotal, "commission": commission,
		})
	}
	return out, nil
}