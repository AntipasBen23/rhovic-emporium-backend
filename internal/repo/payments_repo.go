package repo

import (
	"context"

	"github.com/jackc/pgx/v5"
)

type PaymentsRepo struct{}

func NewPaymentsRepo() *PaymentsRepo { return &PaymentsRepo{} }

func (r *PaymentsRepo) Create(ctx context.Context, tx pgx.Tx, id, orderID, provider, providerRef, status string, amount int64, idemKey string) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO payments (id,order_id,provider,provider_ref,status,amount,idempotency_key)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
	`, id, orderID, provider, providerRef, status, amount, idemKey)
	return err
}

func (r *PaymentsRepo) MarkSuccess(ctx context.Context, tx pgx.Tx, providerRef string) error {
	_, err := tx.Exec(ctx, `
		UPDATE payments SET status='success' WHERE provider_ref=$1
	`, providerRef)
	return err
}

func (r *PaymentsRepo) ExistsIdem(ctx context.Context, tx pgx.Tx, idemKey string) (bool, error) {
	var ok bool
	err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM payments WHERE idempotency_key=$1)`, idemKey).Scan(&ok)
	return ok, err
}