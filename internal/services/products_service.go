 package services

import (
	"context"

	"rhovic/backend/internal/domain"
	"rhovic/backend/internal/repo"
)

type ProductsService struct {
	products *repo.ProductsRepo
}

func NewProductsService(products *repo.ProductsRepo) *ProductsService {
	return &ProductsService{products: products}
}

func (s *ProductsService) ListPublished(ctx context.Context, limit, offset int) ([]domain.Product, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	return s.products.ListPublished(ctx, limit, offset)
}

func (s *ProductsService) Get(ctx context.Context, id string) (domain.Product, error) {
	return s.products.Get(ctx, id)
}