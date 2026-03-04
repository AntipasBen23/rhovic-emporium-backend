package services

import (
	"context"

	"rhovic/backend/internal/db"
	"rhovic/backend/internal/domain"
	"rhovic/backend/internal/repo"
	"rhovic/backend/internal/util"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AdminService struct {
	pool     *pgxpool.Pool
	metrics  *repo.AdminMetricsRepo
	products *repo.ProductsRepo
	vendors  *repo.VendorsRepo
	settings *repo.SettingsRepo
	payouts  *repo.PayoutsRepo
	disputes *repo.DisputesRepo
	logs     *repo.AdminLogsRepo
	ledger   *repo.LedgerRepo
}

func NewAdminService(pool *pgxpool.Pool, metrics *repo.AdminMetricsRepo, products *repo.ProductsRepo, vendors *repo.VendorsRepo, settings *repo.SettingsRepo, payouts *repo.PayoutsRepo, disputes *repo.DisputesRepo, logs *repo.AdminLogsRepo, ledger *repo.LedgerRepo) *AdminService {
	return &AdminService{pool: pool, metrics: metrics, products: products, vendors: vendors, settings: settings, payouts: payouts, disputes: disputes, logs: logs, ledger: ledger}
}

func (s *AdminService) Metrics(ctx context.Context) (map[string]any, error) {
	return s.metrics.Metrics(ctx)
}

func (s *AdminService) SetDefaultCommission(ctx context.Context, adminID, rate string) error {
	return s.settings.Set(ctx, "commission_default_rate", rate)
}

func (s *AdminService) AdminListProducts(ctx context.Context, limit, offset int) ([]domain.Product, error) {
	return s.products.AdminListAll(ctx, limit, offset)
}

func (s *AdminService) UpdateProductCommission(ctx context.Context, adminID, productID string, rate *float64) error {
	return s.products.UpdateAdminCommission(ctx, productID, rate)
}

func (s *AdminService) ApprovePayout(ctx context.Context, adminID, payoutID string) error {
	return db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		// load payout
		var vendorID string
		var amount int64
		var status string
		if err := tx.QueryRow(ctx, `SELECT vendor_id,amount,status FROM payouts WHERE id=$1`, payoutID).Scan(&vendorID, &amount, &status); err != nil {
			return domain.ErrNotFound
		}
		if status != "pending" {
			return domain.ErrConflict
		}

		// verify balance again at approval time
		var available int64
		if err := tx.QueryRow(ctx, `
			WITH ledger AS (
			  SELECT
				COALESCE(SUM(CASE WHEN type='credit' THEN amount ELSE 0 END),0) AS credits,
				COALESCE(SUM(CASE WHEN type='debit' THEN amount ELSE 0 END),0) AS debits
			  FROM vendor_ledger WHERE vendor_id=$1
			)
			SELECT (ledger.credits - ledger.debits) AS available FROM ledger
		`, vendorID).Scan(&available); err != nil {
			return err
		}
		if amount > available {
			return domain.ErrInsufficient
		}

		// approve payout + debit ledger
		if err := s.payouts.UpdateStatus(ctx, tx, payoutID, "approved", nil); err != nil {
			return err
		}
		if err := s.ledger.Debit(ctx, tx, util.NewID(), vendorID, amount, "payout:"+payoutID); err != nil {
			return err
		}

		// audit log
		newV := "approved"
		return s.logs.Log(ctx, tx, util.NewID(), adminID, "payout_approved", "payout", payoutID, nil, &newV)
	})
}

func (s *AdminService) RejectPayout(ctx context.Context, adminID, payoutID, reason string) error {
	return db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		if err := s.payouts.UpdateStatus(ctx, tx, payoutID, "rejected", &reason); err != nil {
			return err
		}
		newV := "rejected"
		return s.logs.Log(ctx, tx, util.NewID(), adminID, "payout_rejected", "payout", payoutID, nil, &newV)
	})
}
