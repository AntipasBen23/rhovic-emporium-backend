package repo

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type CategoriesRepo struct{ db *pgxpool.Pool }

func NewCategoriesRepo(db *pgxpool.Pool) *CategoriesRepo { return &CategoriesRepo{db: db} }

func (r *CategoriesRepo) List(ctx context.Context) ([]map[string]any, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, name
		FROM categories
		ORDER BY name ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []map[string]any
	for rows.Next() {
		var id, name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{
			"id":   id,
			"name": name,
		})
	}
	return out, nil
}
