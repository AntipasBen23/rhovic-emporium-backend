package services

import (
	"context"
	"strconv"

	"rhovic/backend/internal/db"
	"rhovic/backend/internal/domain"
	"rhovic/backend/internal/repo"
	"rhovic/backend/internal/util"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type VendorService struct {
	pool    *pgxpool.Pool
	vendors *repo.VendorsRepo
	plans   *repo.PlansRepo
	vp      *repo.VendorProductsRepo
	payouts *repo.PayoutsRepo
}

func NewVendorService(pool *pgxpool.Pool, vendors *repo.VendorsRepo, plans *repo.PlansRepo, vp *repo.VendorProductsRepo, payouts *repo.PayoutsRepo) *VendorService {
	return &VendorService{pool: pool, vendors: vendors, plans: plans, vp: vp, payouts: payouts}
}

type CreateProductReq struct {
	CategoryID  *string `json:"category_id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Price       int64   `json:"price"`
	PricingUnit string  `json:"pricing_unit"`
	Stock       string  `json:"stock_quantity"`
	Status      string  `json:"status"` // draft/published
	ImageURL    *string `json:"image_url"`
}

func (s *VendorService) CreateProduct(ctx context.Context, userID string, req CreateProductReq) (string, error) {
	v, err := s.vendors.GetByUserID(ctx, userID)
	if err != nil {
		return "", domain.ErrForbidden
	}
	if v.Status != "approved" {
		return "", domain.ErrForbidden
	}
	if req.Name == "" || req.Price <= 0 {
		return "", domain.ErrInvalidInput
	}
	if req.Status == "" {
		req.Status = "draft"
	}
	if _, err := strconv.ParseFloat(req.Stock, 64); err != nil {
		return "", domain.ErrInvalidInput
	}

	id := util.NewID()
	err = db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		lim, _ := s.plans.GetProductLimit(ctx, v.SubscriptionPlanID)
		count, err := s.vp.CountByVendor(ctx, tx, v.ID)
		if err != nil {
			return err
		}
		if lim > 0 && count >= lim {
			return domain.ErrConflict
		}
		return s.vp.Create(ctx, tx, id, v.ID, req.CategoryID, req.Name, req.Description, req.Price, req.PricingUnit, req.Stock, req.Status, req.ImageURL)
	})
	return id, err
}

type UpdateProductReq struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	Price       *int64  `json:"price"`
	PricingUnit *string `json:"pricing_unit"`
	Stock       *string `json:"stock_quantity"`
	Status      *string `json:"status"`
	ImageURL    *string `json:"image_url"`
}

func (s *VendorService) UpdateProduct(ctx context.Context, userID, productID string, req UpdateProductReq) error {
	v, err := s.vendors.GetByUserID(ctx, userID)
	if err != nil || v.Status != "approved" {
		return domain.ErrForbidden
	}
	return db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		ok, err := s.vp.EnsureOwned(ctx, tx, productID, v.ID)
		if err != nil {
			return err
		}
		if !ok {
			return domain.ErrForbidden
		}
		return s.vp.Update(ctx, tx, productID, v.ID, req.Name, req.Description, req.Price, req.PricingUnit, req.Stock, req.Status, req.ImageURL)
	})
}

func (s *VendorService) ListProducts(ctx context.Context, userID string, limit, offset int) ([]map[string]any, error) {
	v, err := s.vendors.GetByUserID(ctx, userID)
	if err != nil || v.Status != "approved" {
		return nil, domain.ErrForbidden
	}
	var out []map[string]any
	err = db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		var err error
		out, err = s.vp.ListByVendor(ctx, tx, v.ID, limit, offset)
		return err
	})
	return out, err
}

func (s *VendorService) RequestPayout(ctx context.Context, userID string, amount int64) (string, error) {
	if amount <= 0 {
		return "", domain.ErrInvalidInput
	}
	v, err := s.vendors.GetByUserID(ctx, userID)
	if err != nil || v.Status != "approved" {
		return "", domain.ErrForbidden
	}
	id := util.NewID()

	// available balance = credits - debits - pending payouts
	var available int64
	err = s.pool.QueryRow(ctx, `
		WITH ledger AS (
		  SELECT
			COALESCE(SUM(CASE WHEN type='credit' THEN amount ELSE 0 END),0) AS credits,
			COALESCE(SUM(CASE WHEN type='debit' THEN amount ELSE 0 END),0) AS debits
		  FROM vendor_ledger WHERE vendor_id=$1
		),
		pending AS (
		  SELECT COALESCE(SUM(amount),0) AS p
		  FROM payouts WHERE vendor_id=$1 AND status IN ('pending','approved')
		)
		SELECT (ledger.credits - ledger.debits - pending.p) AS available
		FROM ledger, pending
	`, v.ID).Scan(&available)
	if err != nil {
		return "", err
	}
	if amount > available {
		return "", domain.ErrInsufficient
	}

	err = db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		return s.payouts.Create(ctx, tx, id, v.ID, amount)
	})
	return id, err
}
