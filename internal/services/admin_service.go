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
	users    *repo.UsersRepo
	refresh  *repo.RefreshTokensRepo
	security *repo.SecurityEventsRepo
	products *repo.ProductsRepo
	vendors  *repo.VendorsRepo
	settings *repo.SettingsRepo
	payouts  *repo.PayoutsRepo
	disputes *repo.DisputesRepo
	logs     *repo.AdminLogsRepo
	ledger   *repo.LedgerRepo
}

func NewAdminService(pool *pgxpool.Pool, metrics *repo.AdminMetricsRepo, users *repo.UsersRepo, refresh *repo.RefreshTokensRepo, security *repo.SecurityEventsRepo, products *repo.ProductsRepo, vendors *repo.VendorsRepo, settings *repo.SettingsRepo, payouts *repo.PayoutsRepo, disputes *repo.DisputesRepo, logs *repo.AdminLogsRepo, ledger *repo.LedgerRepo) *AdminService {
	return &AdminService{pool: pool, metrics: metrics, users: users, refresh: refresh, security: security, products: products, vendors: vendors, settings: settings, payouts: payouts, disputes: disputes, logs: logs, ledger: ledger}
}

func (s *AdminService) Metrics(ctx context.Context) (map[string]any, error) {
	return s.metrics.Metrics(ctx)
}

func (s *AdminService) ListUsers(ctx context.Context, search, role string, includeDeleted bool, limit, offset int) (repo.AdminUserListResult, error) {
	return s.users.AdminList(ctx, search, role, includeDeleted, limit, offset)
}

func (s *AdminService) ListSecurityEvents(ctx context.Context, eventType, search string, limit, offset int) (repo.SecurityEventListResult, error) {
	return s.security.List(ctx, eventType, search, limit, offset)
}

func (s *AdminService) LogoutUser(ctx context.Context, adminID, userID string) error {
	return db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		if err := s.refresh.RevokeAllForUser(ctx, userID); err != nil {
			return err
		}
		marker := "revoked"
		return s.logs.Log(ctx, tx, util.NewID(), adminID, "user_logged_out", "user", userID, nil, &marker)
	})
}

func (s *AdminService) DeleteUser(ctx context.Context, adminID, userID string) error {
	return db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		user, err := s.users.GetByID(ctx, userID)
		if err != nil {
			return domain.ErrNotFound
		}
		if string(user.Role) == string(domain.RoleAdminSuper) || string(user.Role) == string(domain.RoleAdminOps) || string(user.Role) == string(domain.RoleAdminFin) {
			return domain.ErrForbidden
		}

		if vendor, err := s.vendors.GetByUserID(ctx, userID); err == nil {
			if err := s.products.UnpublishByVendor(ctx, vendor.ID); err != nil {
				return err
			}
			if err := s.vendors.SoftDelete(ctx, vendor.ID); err != nil {
				return err
			}
		}

		if err := s.refresh.RevokeAllForUser(ctx, userID); err != nil {
			return err
		}
		if err := s.users.SoftDelete(ctx, userID); err != nil {
			return err
		}

		status := "deleted"
		return s.logs.Log(ctx, tx, util.NewID(), adminID, "user_deleted", "user", userID, nil, &status)
	})
}

func (s *AdminService) LogoutVendor(ctx context.Context, adminID, vendorID string) error {
	vendor, err := s.vendors.GetByID(ctx, vendorID)
	if err != nil {
		return domain.ErrNotFound
	}
	return db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		if err := s.refresh.RevokeAllForUser(ctx, vendor.UserID); err != nil {
			return err
		}
		marker := "revoked"
		return s.logs.Log(ctx, tx, util.NewID(), adminID, "vendor_logged_out", "vendor", vendorID, nil, &marker)
	})
}

func (s *AdminService) DeleteVendor(ctx context.Context, adminID, vendorID string) error {
	vendor, err := s.vendors.GetByID(ctx, vendorID)
	if err != nil {
		return domain.ErrNotFound
	}
	return db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		if err := s.products.UnpublishByVendor(ctx, vendorID); err != nil {
			return err
		}
		if err := s.refresh.RevokeAllForUser(ctx, vendor.UserID); err != nil {
			return err
		}
		if err := s.vendors.SoftDelete(ctx, vendorID); err != nil {
			return err
		}
		if err := s.users.UpdateRole(ctx, vendor.UserID, string(domain.RoleBuyer)); err != nil {
			return err
		}

		status := "deleted"
		return s.logs.Log(ctx, tx, util.NewID(), adminID, "vendor_deleted", "vendor", vendorID, nil, &status)
	})
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

func (s *AdminService) ApproveVendor(ctx context.Context, adminID, vendorID string) error {
	return db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		var current string
		if err := tx.QueryRow(ctx, `SELECT status FROM vendors WHERE id=$1`, vendorID).Scan(&current); err != nil {
			return domain.ErrNotFound
		}
		if current == "approved" {
			return nil
		}
		if _, err := tx.Exec(ctx, `UPDATE vendors SET status='approved' WHERE id=$1`, vendorID); err != nil {
			return err
		}
		newV := "approved"
		oldV := current
		return s.logs.Log(ctx, tx, util.NewID(), adminID, "vendor_approved", "vendor", vendorID, &oldV, &newV)
	})
}

func (s *AdminService) RejectVendor(ctx context.Context, adminID, vendorID string) error {
	return db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		var current string
		if err := tx.QueryRow(ctx, `SELECT status FROM vendors WHERE id=$1`, vendorID).Scan(&current); err != nil {
			return domain.ErrNotFound
		}
		if current == "rejected" {
			return nil
		}
		if _, err := tx.Exec(ctx, `UPDATE vendors SET status='rejected' WHERE id=$1`, vendorID); err != nil {
			return err
		}
		newV := "rejected"
		oldV := current
		return s.logs.Log(ctx, tx, util.NewID(), adminID, "vendor_rejected", "vendor", vendorID, &oldV, &newV)
	})
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
