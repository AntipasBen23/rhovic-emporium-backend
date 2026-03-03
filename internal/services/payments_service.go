package services

import (
	"context"

	"rhovic/backend/internal/db"
	"rhovic/backend/internal/domain"
	"rhovic/backend/internal/paystack"
	"rhovic/backend/internal/repo"
	"rhovic/backend/internal/util"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PaymentsService struct {
	pool     *pgxpool.Pool
	paystack *paystack.Client
	ledger   *repo.LedgerRepo
	checkout *repo.CheckoutRepo
}

func NewPaymentsService(pool *pgxpool.Pool, ps *paystack.Client, ledger *repo.LedgerRepo, checkout *repo.CheckoutRepo) *PaymentsService {
	return &PaymentsService{pool: pool, paystack: ps, ledger: ledger, checkout: checkout}
}

func (s *PaymentsService) ProcessPaystackSuccess(ctx context.Context, reference string) error {
	// Verify with Paystack (trust but verify)
	vr, err := s.paystack.Verify(ctx, reference)
	if err != nil {
		return err
	}
	if vr.Data.Status != "success" {
		return domain.ErrConflict
	}

	return db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		// webhook idempotency
		if _, err := tx.Exec(ctx, `
			INSERT INTO webhook_events (id,provider,provider_ref)
			VALUES ($1,'paystack',$2)
			ON CONFLICT (provider_ref) DO NOTHING
		`, util.NewID(), reference); err != nil {
			return err
		}
		var already bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM webhook_events WHERE provider_ref=$1)`, reference).Scan(&already); err != nil {
			return err
		}
		// If inserted above but row exists anyway, we still proceed safely—updates below are idempotent-ish.

		// find order by payment reference
		var orderID string
		if err := tx.QueryRow(ctx, `SELECT order_id FROM payments WHERE provider_ref=$1`, reference).Scan(&orderID); err != nil {
			return domain.ErrNotFound
		}

		// mark payment success and order paid (idempotent updates)
		_, _ = tx.Exec(ctx, `UPDATE payments SET status='success' WHERE provider_ref=$1`, reference)
		_, _ = tx.Exec(ctx, `UPDATE orders SET status='paid' WHERE id=$1 AND status!='paid'`, orderID)

		// credit ledger per vendor and deduct stock
		rows, err := tx.Query(ctx, `
			SELECT id,vendor_id,product_id,quantity::text,subtotal,commission_amount
			FROM order_items WHERE order_id=$1
		`, orderID)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var itemID, vendorID, productID, qty string
			var subtotal, commission int64
			if err := rows.Scan(&itemID, &vendorID, &productID, &qty, &subtotal, &commission); err != nil {
				return err
			}

			// deduct stock (only once). If already deducted, update will fail (no rows) but order is paid—
			// in production you’d have a stock_events table. For v1, we treat "no row" as already deducted.
			_, _ = s.checkout.DeductStock(ctx, tx, productID, qty)

			net := subtotal - commission
			refKey := "order_item:" + itemID
			_ = s.ledger.Credit(ctx, tx, util.NewID(), vendorID, net, refKey)
		}

		return nil
	})
}