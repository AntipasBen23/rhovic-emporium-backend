package repo

import (
	"context"

	"github.com/jackc/pgx/v5"
)

type CheckoutRepo struct{}

func NewCheckoutRepo() *CheckoutRepo { return &CheckoutRepo{} }

type CheckoutItemRow struct {
	ProductID      string
	VendorID       string
	Price          int64
	Status         string
	VendorStatus   string
	StockQtyText   string
	PlanID         string
	OverrideRate   *float64
	PlanRateText   string
}

func (r *CheckoutRepo) LoadItem(ctx context.Context, tx pgx.Tx, productID string) (CheckoutItemRow, error) {
	var row CheckoutItemRow
	err := tx.QueryRow(ctx, `
		SELECT
		  p.id,
		  p.vendor_id,
		  p.price,
		  p.status,
		  v.status,
		  p.stock_quantity::text,
		  v.subscription_plan_id,
		  v.commission_override,
		  sp.commission_rate::text
		FROM products p
		JOIN vendors v ON v.id = p.vendor_id
		LEFT JOIN subscription_plans sp ON sp.id = v.subscription_plan_id
		WHERE p.id=$1
	`, productID).Scan(&row.ProductID, &row.VendorID, &row.Price, &row.Status, &row.VendorStatus, &row.StockQtyText, &row.PlanID, &row.OverrideRate, &row.PlanRateText)
	return row, err
}

func (r *CheckoutRepo) DeductStock(ctx context.Context, tx pgx.Tx, productID string, qty string) (bool, error) {
	ct, err := tx.Exec(ctx, `
		UPDATE products
		SET stock_quantity = stock_quantity - ($2::numeric)
		WHERE id=$1 AND stock_quantity >= ($2::numeric)
	`, productID, qty)
	if err != nil {
		return false, err
	}
	return ct.RowsAffected() == 1, nil
}