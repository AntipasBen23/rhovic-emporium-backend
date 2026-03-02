package repo

import (
	"context"

	"github.com/jackc/pgx/v5"
)

type LedgerRepo struct{}

func NewLedgerRepo() *LedgerRepo { return &LedgerRepo{} }

func (r *LedgerRepo) Credit(ctx context.Context, tx pgx.Tx, id, vendorID string, amount int64, ref string) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO vendor_ledger (id,vendor_id,type,amount,reference)
		VALUES ($1,$2,'credit',$3,$4)
	`, id, vendorID, amount, ref)
	return err
}

func (r *LedgerRepo) Debit(ctx context.Context, tx pgx.Tx, id, vendorID string, amount int64, ref string) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO vendor_ledger (id,vendor_id,type,amount,reference)
		VALUES ($1,$2,'debit',$3,$4)
	`, id, vendorID, amount, ref)
	return err
}