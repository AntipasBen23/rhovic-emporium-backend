package repo

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"
)

type VendorProductsRepo struct{}

func NewVendorProductsRepo() *VendorProductsRepo { return &VendorProductsRepo{} }

func (r *VendorProductsRepo) CountByVendor(ctx context.Context, tx pgx.Tx, vendorID string) (int, error) {
	var n int
	err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM products WHERE vendor_id=$1`, vendorID).Scan(&n)
	return n, err
}

func (r *VendorProductsRepo) Create(ctx context.Context, tx pgx.Tx, id, vendorID string, categoryID *string, name, desc string, price int64, compareAtPrice *int64, unit string, stock string, status string, imageURL *string, imageURLs []string) error {
	imagesJSON, err := json.Marshal(imageURLs)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO products (id,vendor_id,category_id,name,description,price,compare_at_price,pricing_unit,stock_quantity,status,image_url,image_urls)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9::numeric,$10,$11,$12::jsonb)
	`, id, vendorID, categoryID, name, desc, price, compareAtPrice, unit, stock, status, imageURL, string(imagesJSON))
	return err
}

func (r *VendorProductsRepo) Update(ctx context.Context, tx pgx.Tx, id, vendorID string, categoryID *string, name, desc *string, price, compareAtPrice *int64, unit, stock, status *string, imageURL *string, imageURLs []string) error {
	imagesJSON, err := json.Marshal(imageURLs)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		UPDATE products SET
		  category_id = COALESCE($3,category_id),
		  name = COALESCE($4,name),
		  description = COALESCE($5,description),
		  price = COALESCE($6,price),
		  compare_at_price = COALESCE($7,compare_at_price),
		  pricing_unit = COALESCE($8,pricing_unit),
		  stock_quantity = COALESCE(($9)::numeric, stock_quantity),
		  status = COALESCE($10,status),
		  image_url = COALESCE($11,image_url),
		  image_urls = COALESCE($12::jsonb, image_urls)
		WHERE id=$1 AND vendor_id=$2
	`, id, vendorID, categoryID, name, desc, price, compareAtPrice, unit, stock, status, imageURL, string(imagesJSON))
	return err
}

func (r *VendorProductsRepo) ListByVendor(ctx context.Context, tx pgx.Tx, vendorID string, limit, offset int) ([]map[string]any, error) {
	rows, err := tx.Query(ctx, `
		SELECT id,category_id,name,description,price,compare_at_price,pricing_unit,stock_quantity,status,image_url,COALESCE(image_urls,'[]'::jsonb),created_at
		FROM products
		WHERE vendor_id=$1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`, vendorID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []map[string]any{}
	for rows.Next() {
		var id, name, desc, unit, status string
		var catID, imgURL *string
		var compareAtPrice *int64
		var price int64
		var stock float64
		var imageURLsJSON []byte
		var createdAt any
		if err := rows.Scan(&id, &catID, &name, &desc, &price, &compareAtPrice, &unit, &stock, &status, &imgURL, &imageURLsJSON, &createdAt); err != nil {
			return nil, err
		}
		var imageURLs []string
		if len(imageURLsJSON) > 0 {
			if err := json.Unmarshal(imageURLsJSON, &imageURLs); err != nil {
				return nil, err
			}
		}
		out = append(out, map[string]any{
			"id":               id,
			"category_id":      catID,
			"name":             name,
			"description":      desc,
			"price":            price,
			"compare_at_price": compareAtPrice,
			"pricing_unit":     unit,
			"stock_quantity":   stock,
			"status":           status,
			"image_url":        imgURL,
			"image_urls":       imageURLs,
			"created_at":       createdAt,
		})
	}
	return out, nil
}

func (r *VendorProductsRepo) EnsureOwned(ctx context.Context, tx pgx.Tx, id, vendorID string) (bool, error) {
	var ok bool
	err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM products WHERE id=$1 AND vendor_id=$2)`, id, vendorID).Scan(&ok)
	return ok, err
}

func (r *VendorProductsRepo) Delete(ctx context.Context, tx pgx.Tx, id, vendorID string) error {
	_, err := tx.Exec(ctx, `DELETE FROM products WHERE id=$1 AND vendor_id=$2`, id, vendorID)
	return err
}
