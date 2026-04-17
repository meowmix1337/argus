package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	apperrors "github.com/meowmix1337/argus/backend/internal/errors"
	"github.com/meowmix1337/argus/backend/internal/events"
	"github.com/meowmix1337/argus/backend/internal/model"
)

// FollowStore defines the data-access contract for follow relationships.
type FollowStore interface {
	Follow(ctx context.Context, id, followerID, followingID string) error
	Unfollow(ctx context.Context, followerID, followingID string) (int64, error)
	IsFollowing(ctx context.Context, followerID, followingID string) (bool, error)
	ListFollowers(ctx context.Context, userID string, limit, offset int) ([]model.UserSummary, int, error)
	ListFollowing(ctx context.Context, userID string, limit, offset int) ([]model.UserSummary, int, error)
}

// FollowService manages follow relationships.
type FollowService struct {
	store     FollowStore
	publisher events.Publisher
}

// NewFollowService creates a new FollowService.
func NewFollowService(store FollowStore, publisher events.Publisher) *FollowService {
	return &FollowService{store: store, publisher: publisher}
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

	if pubErr := s.publisher.PublishEvent(events.TopicUserFollowed, events.UserFollowedPayload{
		FollowerID:  followerID,
		FollowingID: followingID,
	}); pubErr != nil {
		slog.Error("failed to publish user.followed event", "error", pubErr)
	}

	if pubErr := s.publisher.PublishEvent(events.TopicFollowCreated, events.FollowCreatedPayload{
		FollowerID:   followerID,
		FollowingID:  followingID,
		FollowerName: followerName,
	}); pubErr != nil {
		slog.Error("failed to publish follow.created event", "error", pubErr)
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
