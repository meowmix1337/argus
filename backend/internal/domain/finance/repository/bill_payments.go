package repository

import (
	"context"

	"github.com/meowmix1337/argus/backend/internal/model"
)

// BillPaymentStore defines the data-access contract for bill payments.
type BillPaymentStore interface {
	Create(ctx context.Context, p model.BillPaymentCreate) (model.BillPayment, error)
	Delete(ctx context.Context, id string, userID string) (int64, error)
	ListForYear(ctx context.Context, userID string, year int) ([]model.BillPayment, error)
	ListForMonth(ctx context.Context, userID string, year, month int) ([]model.BillPayment, error)
}
