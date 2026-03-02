package repo

import (
	"context"

	"github.com/jackc/pgx/v5"
)

type AdminLogsRepo struct{}

func NewAdminLogsRepo() *AdminLogsRepo { return &AdminLogsRepo{} }

func (r *AdminLogsRepo) Log(ctx context.Context, tx pgx.Tx, id, adminID, action, entity, entityID string, oldV, newV *string) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO admin_logs (id,admin_id,action,entity_type,entity_id,old_value,new_value)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
	`, id, adminID, action, entity, entityID, oldV, newV)
	return err
}