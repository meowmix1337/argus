package service

import (
	"context"
	"fmt"

	"github.com/meowmix1337/argus/backend/internal/model"
)

// FeedStore defines the data-access contract for the social feed.
type FeedStore interface {
	// ListFeed returns posts via a live join (followers + own posts).
	// Used as the fallback when user_feed has no rows for the viewer.
	ListFeed(ctx context.Context, viewerID string, cursor *model.FeedCursor, limit int) ([]model.Post, error)
	// ListFeedMaterialized reads from the pre-computed user_feed table.
	ListFeedMaterialized(ctx context.Context, viewerID string, cursor *model.FeedCursor, limit int) ([]model.Post, error)
	// CountUserFeedForUser returns the number of rows in user_feed for the given user.
	CountUserFeedForUser(ctx context.Context, userID string) (int, error)
	// BulkInsertUserFeed inserts fanout rows into user_feed (INSERT OR IGNORE).
	BulkInsertUserFeed(ctx context.Context, rows []model.UserFeedRow) error
}

// FeedService provides the social feed timeline.
type FeedService struct {
	store FeedStore
}

// NewFeedService creates a new FeedService.
func NewFeedService(store FeedStore) *FeedService {
	return &FeedService{store: store}
}

// ListFeed returns the chronological feed for the viewer.
// It uses the materialized user_feed table when populated, falling back to a
// live join for new accounts and users who followed before fanout was deployed.
func (s *FeedService) ListFeed(ctx context.Context, viewerID string, cursor *model.FeedCursor, limit int) ([]model.Post, error) {
	count, err := s.store.CountUserFeedForUser(ctx, viewerID)
	if err != nil {
		return nil, fmt.Errorf("count user feed: %w", err)
	}

	if count > 0 {
		posts, err := s.store.ListFeedMaterialized(ctx, viewerID, cursor, limit)
		if err != nil {
			return nil, fmt.Errorf("list feed materialized: %w", err)
		}
		return posts, nil
	}

	// Fallback: live join query.
	posts, err := s.store.ListFeed(ctx, viewerID, cursor, limit)
	if err != nil {
		return nil, fmt.Errorf("list feed: %w", err)
	}
	return posts, nil
}
