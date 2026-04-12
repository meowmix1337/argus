package service

import (
	"context"
	"fmt"

	"github.com/meowmix1337/argus/backend/internal/model"
)

// UserSearchStore defines the data-access contract for user search.
type UserSearchStore interface {
	SearchUsers(ctx context.Context, viewerID, q string, limit, offset int) ([]model.UserSummary, int, error)
}

// UserService provides user discovery operations.
type UserService struct {
	store UserSearchStore
}

// NewUserService creates a new UserService.
func NewUserService(store UserSearchStore) *UserService {
	return &UserService{store: store}
}

// Search returns users matching the query, excluding the viewer.
func (s *UserService) Search(ctx context.Context, viewerID, q string, limit, offset int) ([]model.UserSummary, int, error) {
	users, total, err := s.store.SearchUsers(ctx, viewerID, q, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("search users: %w", err)
	}
	return users, total, nil
}
