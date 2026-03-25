package services

import (
	"context"
	"strings"

	"rhovic/backend/internal/db"
	"rhovic/backend/internal/domain"
	"rhovic/backend/internal/repo"
	"rhovic/backend/internal/util"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SupportService struct {
	pool    *pgxpool.Pool
	support *repo.SupportRepo
	logs    *repo.AdminLogsRepo
}

func NewSupportService(pool *pgxpool.Pool, support *repo.SupportRepo, logs *repo.AdminLogsRepo) *SupportService {
	return &SupportService{pool: pool, support: support, logs: logs}
}

func (s *SupportService) CreateThread(ctx context.Context, customerID string, orderID *string, subject, message string) (repo.SupportThreadDetail, error) {
	subject = strings.TrimSpace(subject)
	message = strings.TrimSpace(message)
	if subject == "" || message == "" {
		return repo.SupportThreadDetail{}, domain.ErrInvalidInput
	}
	if len(subject) > 160 || len(message) > 4000 {
		return repo.SupportThreadDetail{}, domain.ErrInvalidInput
	}
	if err := s.ensureOrderOwnership(ctx, customerID, orderID); err != nil {
		return repo.SupportThreadDetail{}, err
	}

	threadID := util.NewID()
	messageID := util.NewID()
	if err := db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		if err := s.support.CreateThread(ctx, tx, threadID, customerID, orderID, subject); err != nil {
			return err
		}
		return s.support.CreateMessage(ctx, tx, messageID, threadID, customerID, "buyer", message)
	}); err != nil {
		return repo.SupportThreadDetail{}, err
	}

	return s.support.GetThreadForCustomer(ctx, customerID, threadID)
}

func (s *SupportService) ListCustomerThreads(ctx context.Context, customerID string, limit, offset int) (repo.SupportThreadListResult, error) {
	return s.support.ListCustomerThreads(ctx, customerID, limit, offset)
}

func (s *SupportService) GetCustomerThread(ctx context.Context, customerID, threadID string) (repo.SupportThreadDetail, error) {
	return s.support.GetThreadForCustomer(ctx, customerID, threadID)
}

func (s *SupportService) AddCustomerMessage(ctx context.Context, customerID, threadID, message string) (repo.SupportThreadDetail, error) {
	message = strings.TrimSpace(message)
	if message == "" || len(message) > 4000 {
		return repo.SupportThreadDetail{}, domain.ErrInvalidInput
	}
	detail, err := s.support.GetThreadForCustomer(ctx, customerID, threadID)
	if err != nil {
		return repo.SupportThreadDetail{}, domain.ErrNotFound
	}
	if detail.Thread.Status == "closed" {
		return repo.SupportThreadDetail{}, domain.ErrConflict
	}

	if err := db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		return s.support.CreateMessage(ctx, tx, util.NewID(), threadID, customerID, "buyer", message)
	}); err != nil {
		return repo.SupportThreadDetail{}, err
	}
	return s.support.GetThreadForCustomer(ctx, customerID, threadID)
}

func (s *SupportService) ListAdminThreads(ctx context.Context, status, search string, limit, offset int) (repo.SupportThreadListResult, error) {
	return s.support.ListAdminThreads(ctx, status, search, limit, offset)
}

func (s *SupportService) GetAdminThread(ctx context.Context, threadID string) (repo.SupportThreadDetail, error) {
	return s.support.GetThreadForAdmin(ctx, threadID)
}

func (s *SupportService) AddAdminMessage(ctx context.Context, adminID, threadID, message string) (repo.SupportThreadDetail, error) {
	message = strings.TrimSpace(message)
	if message == "" || len(message) > 4000 {
		return repo.SupportThreadDetail{}, domain.ErrInvalidInput
	}
	detail, err := s.support.GetThreadForAdmin(ctx, threadID)
	if err != nil {
		return repo.SupportThreadDetail{}, domain.ErrNotFound
	}
	if detail.Thread.Status == "closed" {
		return repo.SupportThreadDetail{}, domain.ErrConflict
	}

	if err := db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		if err := s.support.SetStatus(ctx, tx, threadID, "in_progress", &adminID); err != nil {
			return err
		}
		if err := s.support.CreateMessage(ctx, tx, util.NewID(), threadID, adminID, "admin", message); err != nil {
			return err
		}
		status := "in_progress"
		return s.logs.Log(ctx, tx, util.NewID(), adminID, "support_replied", "support_thread", threadID, nil, &status)
	}); err != nil {
		return repo.SupportThreadDetail{}, err
	}
	return s.support.GetThreadForAdmin(ctx, threadID)
}

func (s *SupportService) CloseThread(ctx context.Context, adminID, threadID string) error {
	if _, err := s.support.GetThreadForAdmin(ctx, threadID); err != nil {
		return domain.ErrNotFound
	}
	return db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		if err := s.support.SetStatus(ctx, tx, threadID, "closed", &adminID); err != nil {
			return err
		}
		status := "closed"
		return s.logs.Log(ctx, tx, util.NewID(), adminID, "support_closed", "support_thread", threadID, nil, &status)
	})
}

func (s *SupportService) ensureOrderOwnership(ctx context.Context, customerID string, orderID *string) error {
	if orderID == nil || strings.TrimSpace(*orderID) == "" {
		return nil
	}
	var ok bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1
			FROM orders
			WHERE id = $1
			  AND (customer_id = $2 OR buyer_id = $2)
		)
	`, strings.TrimSpace(*orderID), customerID).Scan(&ok)
	if err != nil {
		return err
	}
	if !ok {
		return domain.ErrForbidden
	}
	return nil
}
