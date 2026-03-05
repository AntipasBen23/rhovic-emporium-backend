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
		SELECT id,user_id,business_name,phone,bank_name,account_number,status,commission_override,created_at
		FROM vendors WHERE user_id=$1
	`, userID).Scan(&v.ID, &v.UserID, &v.BusinessName, &v.Phone, &v.BankName, &v.AccountNumber, &v.Status, &v.CommissionOverride, &v.CreatedAt)
	return v, err
}

func (r *VendorsRepo) List(ctx context.Context, limit, offset int) ([]domain.Vendor, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id,user_id,business_name,phone,bank_name,account_number,status,commission_override,created_at
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
		if err := rows.Scan(&v.ID, &v.UserID, &v.BusinessName, &v.Phone, &v.BankName, &v.AccountNumber, &v.Status, &v.CommissionOverride, &v.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

func (r *VendorsRepo) CreateForUser(ctx context.Context, id, userID string, v domain.VendorRegisterProfile) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO vendors (
			id, user_id,
			first_name, last_name,
			business_name, shop_url, phone,
			street, street2, city, zip_code, country, state,
			company_name, company_id, vat_id,
			bank_name, account_number,
			status, created_at
		) VALUES (
			$1, $2,
			$3, $4,
			$5, $6, $7,
			$8, $9, $10, $11, $12, $13,
			$14, $15, $16,
			$17, $18,
			'pending', NOW()
		)
	`,
		id, userID,
		v.FirstName, v.LastName,
		v.ShopName, v.ShopURL, v.Phone,
		v.Street, v.Street2, v.City, v.ZipCode, v.Country, v.State,
		v.CompanyName, v.CompanyID, v.VatID,
		v.BankName, v.AccountIBAN,
	)
	return err
}

func (r *VendorsRepo) UpdateStatus(ctx context.Context, id, status string) error {
	_, err := r.db.Exec(ctx, `UPDATE vendors SET status=$2 WHERE id=$1`, id, status)
	return err
}

func (r *VendorsRepo) UpdateApplicationByUserID(ctx context.Context, userID string, v domain.VendorRegisterProfile) error {
	_, err := r.db.Exec(ctx, `
		UPDATE vendors
		SET
			first_name=$2,
			last_name=$3,
			business_name=$4,
			shop_url=$5,
			phone=$6,
			street=$7,
			street2=$8,
			city=$9,
			zip_code=$10,
			country=$11,
			state=$12,
			company_name=$13,
			company_id=$14,
			vat_id=$15,
			bank_name=$16,
			account_number=$17,
			status='pending'
		WHERE user_id=$1
	`,
		userID,
		v.FirstName, v.LastName,
		v.ShopName, v.ShopURL, v.Phone,
		v.Street, v.Street2, v.City, v.ZipCode, v.Country, v.State,
		v.CompanyName, v.CompanyID, v.VatID,
		v.BankName, v.AccountIBAN,
	)
	return err
}
