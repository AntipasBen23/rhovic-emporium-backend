package repo

import (
	"context"

	"rhovic/backend/internal/domain"

	"github.com/jackc/pgx/v5/pgxpool"
)

type UsersRepo struct {
	db *pgxpool.Pool
}

func NewUsersRepo(db *pgxpool.Pool) *UsersRepo {
	return &UsersRepo{db: db}
}

func (r *UsersRepo) Create(ctx context.Context, u domain.User) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO users (id, email, password_hash, role, created_at)
		VALUES ($1, $2, $3, $4, NOW())
	`, u.ID, u.Email, u.PasswordHash, u.Role)
	return err
}

func (r *UsersRepo) GetByEmail(ctx context.Context, email string) (domain.User, error) {
	var u domain.User
	err := r.db.QueryRow(ctx, `
		SELECT id, email, password_hash, role, created_at
		FROM users
		WHERE email = $1
	`, email).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Role, &u.CreatedAt)
	return u, err
}

func (r *UsersRepo) GetByID(ctx context.Context, id string) (domain.User, error) {
	var u domain.User
	err := r.db.QueryRow(ctx, `
		SELECT id, email, password_hash, role, created_at
		FROM users
		WHERE id = $1
	`, id).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Role, &u.CreatedAt)
	return u, err
}
