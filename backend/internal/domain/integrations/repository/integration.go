package repository

import (
	"context"

	"github.com/meowmix1337/argus/backend/internal/model"
)

// IntegrationStore defines the data-access contract for user integrations.
type IntegrationStore interface {
	Create(ctx context.Context, i model.IntegrationCreate) (model.UserIntegration, error)
	GetByUserAndProvider(ctx context.Context, userID, providerID string) (model.UserIntegration, error)
	GetByID(ctx context.Context, id, userID string) (model.UserIntegration, error)
	Delete(ctx context.Context, id, userID string) (int64, error)
}
