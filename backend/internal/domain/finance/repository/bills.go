package repository

import (
	"context"

	"github.com/meowmix1337/argus/backend/internal/model"
)

// BillStore defines the data-access contract for bills.
type BillStore interface {
	List(ctx context.Context, userID string, limit, offset int) ([]model.Bill, int, error)
	Get(ctx context.Context, id string, userID string) (model.Bill, error)
	Create(ctx context.Context, b model.BillCreate) (model.Bill, error)
	Update(ctx context.Context, id string, userID string, u model.BillUpdate) (int64, error)
	Delete(ctx context.Context, id string, userID string) (int64, error)
	ListActive(ctx context.Context, userID string) ([]model.Bill, error)
}
