package repo

import (
	"context"
	"time"

	"rhovic/backend/internal/domain"
	"rhovic/backend/internal/util"

	"github.com/jackc/pgx/v5/pgxpool"
)

type UsersRepo struct {
	db *pgxpool.Pool
}

type AdminUserListItem struct {
	ID              string     `json:"id"`
	Email           string     `json:"email"`
	Role            string     `json:"role"`
	CreatedAt       time.Time  `json:"created_at"`
	VendorID        *string    `json:"vendor_id,omitempty"`
	VendorName      *string    `json:"vendor_name,omitempty"`
	VendorStatus    *string    `json:"vendor_status,omitempty"`
	VendorDeletedAt *time.Time `json:"vendor_deleted_at,omitempty"`
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

// CreateVendorProfile inserts a new vendors row for a user who just registered as a vendor.
func (r *UsersRepo) CreateVendorProfile(ctx context.Context, userID string, v domain.VendorRegisterProfile) error {
	id := util.NewID()
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

func (r *UsersRepo) GetByEmail(ctx context.Context, email string) (domain.User, error) {
	var u domain.User
	err := r.db.QueryRow(ctx, `
		SELECT id, email, password_hash, role, created_at
		FROM users
		WHERE email = $1 AND deleted_at IS NULL
	`, email).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Role, &u.CreatedAt)
	return u, err
}

func (r *UsersRepo) GetByID(ctx context.Context, id string) (domain.User, error) {
	var u domain.User
	err := r.db.QueryRow(ctx, `
		SELECT id, email, password_hash, role, created_at
		FROM users
		WHERE id = $1 AND deleted_at IS NULL
	`, id).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Role, &u.CreatedAt)
	return u, err
}

func (r *UsersRepo) UpdatePassword(ctx context.Context, id, passwordHash string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE users
		SET password_hash = $2
		WHERE id = $1
	`, id, passwordHash)
	return err
}

func (r *UsersRepo) AdminList(ctx context.Context, limit, offset int) ([]AdminUserListItem, error) {
	rows, err := r.db.Query(ctx, `
		SELECT
			u.id,
			u.email,
			u.role,
			u.created_at,
			v.id,
			v.business_name,
			v.status,
			v.deleted_at
		FROM users u
		LEFT JOIN vendors v ON v.user_id = u.id
		WHERE u.deleted_at IS NULL
		ORDER BY u.created_at DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []AdminUserListItem{}
	for rows.Next() {
		var item AdminUserListItem
		if err := rows.Scan(
			&item.ID,
			&item.Email,
			&item.Role,
			&item.CreatedAt,
			&item.VendorID,
			&item.VendorName,
			&item.VendorStatus,
			&item.VendorDeletedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *UsersRepo) SoftDelete(ctx context.Context, userID string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE users
		SET deleted_at = now()
		WHERE id = $1 AND deleted_at IS NULL
	`, userID)
	return err
}

func (r *UsersRepo) UpdateRole(ctx context.Context, userID, role string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE users
		SET role = $2
		WHERE id = $1 AND deleted_at IS NULL
	`, userID, role)
	return err
}
