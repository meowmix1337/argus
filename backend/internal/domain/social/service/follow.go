package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"github.com/meowmix1337/argus/backend/internal/domain/social/repository"
	"github.com/meowmix1337/argus/backend/internal/model"
	apperrors "github.com/meowmix1337/argus/backend/internal/platform/errors"
	"github.com/meowmix1337/argus/backend/internal/platform/publisher"
)

// FollowService manages follow relationships.
type FollowService struct {
	store repository.FollowStore
	pub   publisher.Publisher
}

// NewFollowService creates a new FollowService.
func NewFollowService(store repository.FollowStore, pub publisher.Publisher) *FollowService {
	return &FollowService{store: store, pub: pub}
}

// Follow creates a follow relationship. Returns error if self-follow or already following.
func (s *FollowService) Follow(ctx context.Context, followerID, followingID, followerName string) error {
	if followerID == followingID {
		return apperrors.ErrSelfFollow
	}

	already, err := s.store.IsFollowing(ctx, followerID, followingID)
	if err != nil {
		return fmt.Errorf("check follow: %w", err)
	}
	if already {
		return apperrors.ErrAlreadyFollowing
	}

	id, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("generate follow id: %w", err)
	}

	if err := s.store.Follow(ctx, id.String(), followerID, followingID); err != nil {
		if errors.Is(err, apperrors.ErrAlreadyFollowing) {
			return apperrors.ErrAlreadyFollowing
		}
		return fmt.Errorf("follow: %w", err)
	}

	if pubErr := s.pub.PublishEvent(publisher.TopicUserFollowed, publisher.UserFollowedPayload{
		FollowerID:   followerID,
		FollowingID:  followingID,
		FollowerName: followerName,
	}); pubErr != nil {
		slog.Error("failed to publish user.followed event", "error", pubErr)
	}

	return nil
}

// Unfollow removes a follow relationship.
func (s *FollowService) Unfollow(ctx context.Context, followerID, followingID string) error {
	rows, err := s.store.Unfollow(ctx, followerID, followingID)
	if err != nil {
		return fmt.Errorf("unfollow: %w", err)
	}
	if rows == 0 {
		return apperrors.ErrNotFollowing
	}
	return nil
}

// IsFollowing checks whether followerID follows followingID.
func (s *FollowService) IsFollowing(ctx context.Context, followerID, followingID string) (bool, error) {
	return s.store.IsFollowing(ctx, followerID, followingID)
}

// ListFollowers returns users who follow the given user.
func (s *FollowService) ListFollowers(ctx context.Context, userID string, limit, offset int) ([]model.UserSummary, int, error) {
	return s.store.ListFollowers(ctx, userID, limit, offset)
}

// ListFollowing returns users the given user follows.
func (s *FollowService) ListFollowing(ctx context.Context, userID string, limit, offset int) ([]model.UserSummary, int, error) {
	return s.store.ListFollowing(ctx, userID, limit, offset)
}
