package repo

import (
	"context"

	"rhovic/backend/internal/domain"

	"github.com/jackc/pgx/v5/pgxpool"
)

type VendorsRepo struct{ db *pgxpool.Pool }

func NewVendorsRepo(db *pgxpool.Pool) *VendorsRepo { return &VendorsRepo{db: db} }

func (r *VendorsRepo) GetByUserID(ctx context.Context, userID string) (domain.Vendor, error) {
	var v domain.Vendor
	err := r.db.QueryRow(ctx, `
		SELECT id,user_id,business_name,phone,bank_name,account_number,status,subscription_plan_id,commission_override,created_at
		FROM vendors WHERE user_id=$1
	`, userID).Scan(&v.ID, &v.UserID, &v.BusinessName, &v.Phone, &v.BankName, &v.AccountNumber, &v.Status, &v.SubscriptionPlanID, &v.CommissionOverride, &v.CreatedAt)
	return v, err
}

func (r *VendorsRepo) List(ctx context.Context, limit, offset int) ([]domain.Vendor, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id,user_id,business_name,phone,bank_name,account_number,status,subscription_plan_id,commission_override,created_at
		FROM vendors ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.Vendor
	for rows.Next() {
		var v domain.Vendor
		if err := rows.Scan(&v.ID, &v.UserID, &v.BusinessName, &v.Phone, &v.BankName, &v.AccountNumber, &v.Status, &v.SubscriptionPlanID, &v.CommissionOverride, &v.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}