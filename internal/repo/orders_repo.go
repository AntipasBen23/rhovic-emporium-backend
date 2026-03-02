package repo

import (
	"context"

	"github.com/jackc/pgx/v5"
)

type OrdersRepo struct{}

func NewOrdersRepo() *OrdersRepo { return &OrdersRepo{} }

func (r *OrdersRepo) CreateOrder(ctx context.Context, tx pgx.Tx, id, buyerID, status string, total int64) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO orders (id,buyer_id,total_amount,status)
		VALUES ($1,$2,$3,$4)
	`, id, buyerID, total, status)
	return err
}

func (r *OrdersRepo) CreateItem(ctx context.Context, tx pgx.Tx, id, orderID, vendorID, productID, qty string, unitPrice, subtotal, commission int64) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO order_items
		  (id,order_id,vendor_id,product_id,quantity,unit_price,subtotal,commission_amount)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
	`, id, orderID, vendorID, productID, qty, unitPrice, subtotal, commission)
	return err
}

func (r *OrdersRepo) MarkPaid(ctx context.Context, tx pgx.Tx, orderID string) error {
	_, err := tx.Exec(ctx, `UPDATE orders SET status='paid' WHERE id=$1`, orderID)
	return err
}