package repo

import (
	"context"

	"rhovic/backend/internal/domain"

	"github.com/jackc/pgx/v5/pgxpool"
)

type ProductsRepo struct{ db *pgxpool.Pool }

func NewProductsRepo(db *pgxpool.Pool) *ProductsRepo { return &ProductsRepo{db: db} }

func (r *ProductsRepo) ListPublished(ctx context.Context, limit, offset int) ([]domain.Product, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id,vendor_id,category_id,name,description,price,pricing_unit,stock_quantity,status,image_url,admin_commission_rate,created_at
		FROM products
		WHERE status='published'
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []domain.Product{}
	for rows.Next() {
		var p domain.Product
		if err := rows.Scan(&p.ID, &p.VendorID, &p.CategoryID, &p.Name, &p.Description, &p.Price, &p.PricingUnit, &p.StockQuantity, &p.Status, &p.ImageURL, &p.AdminCommissionRate, &p.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}

func (r *ProductsRepo) Get(ctx context.Context, id string) (domain.Product, error) {
	var p domain.Product
	err := r.db.QueryRow(ctx, `
		SELECT id,vendor_id,category_id,name,description,price,pricing_unit,stock_quantity,status,image_url,admin_commission_rate,created_at
		FROM products WHERE id=$1
	`, id).Scan(&p.ID, &p.VendorID, &p.CategoryID, &p.Name, &p.Description, &p.Price, &p.PricingUnit, &p.StockQuantity, &p.Status, &p.ImageURL, &p.AdminCommissionRate, &p.CreatedAt)
	return p, err
}

func (r *ProductsRepo) AdminListAll(ctx context.Context, limit, offset int) ([]domain.Product, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id,vendor_id,category_id,name,description,price,pricing_unit,stock_quantity,status,image_url,admin_commission_rate,created_at
		FROM products
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []domain.Product{}
	for rows.Next() {
		var p domain.Product
		if err := rows.Scan(&p.ID, &p.VendorID, &p.CategoryID, &p.Name, &p.Description, &p.Price, &p.PricingUnit, &p.StockQuantity, &p.Status, &p.ImageURL, &p.AdminCommissionRate, &p.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}

func (r *ProductsRepo) UpdateAdminCommission(ctx context.Context, id string, rate *float64) error {
	_, err := r.db.Exec(ctx, `UPDATE products SET admin_commission_rate=$2 WHERE id=$1`, id, rate)
	return err
}
