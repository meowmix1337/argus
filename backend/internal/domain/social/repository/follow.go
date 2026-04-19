package repository

import (
	"context"

	"github.com/meowmix1337/argus/backend/internal/model"
)

// FollowStore defines the data-access contract for follow relationships.
type FollowStore interface {
	Follow(ctx context.Context, id, followerID, followingID string) error
	Unfollow(ctx context.Context, followerID, followingID string) (int64, error)
	IsFollowing(ctx context.Context, followerID, followingID string) (bool, error)
	ListFollowers(ctx context.Context, userID string, limit, offset int) ([]model.UserSummary, int, error)
	ListFollowing(ctx context.Context, userID string, limit, offset int) ([]model.UserSummary, int, error)
	GetFollowerIDs(ctx context.Context, userID string) ([]string, error)
}
