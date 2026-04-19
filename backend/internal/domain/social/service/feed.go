package service

import (
	"context"
	"fmt"

	"github.com/meowmix1337/argus/backend/internal/domain/social/repository"
	"github.com/meowmix1337/argus/backend/internal/model"
)

// FeedService provides the social feed timeline.
type FeedService struct {
	store repository.FeedStore
}

// NewFeedService creates a new FeedService.
func NewFeedService(store repository.FeedStore) *FeedService {
	return &FeedService{store: store}
}

// ListFeed returns the chronological feed for the viewer.
// It tries the materialized user_feed table first; if it returns no rows (new
// account, or viewer followed before fanout was deployed) it falls back to the
// live join query — one round-trip in the common steady-state case.
func (s *FeedService) ListFeed(ctx context.Context, viewerID string, cursor *model.FeedCursor, limit int) ([]model.Post, error) {
	posts, err := s.store.ListFeedMaterialized(ctx, viewerID, cursor, limit)
	if err != nil {
		return nil, fmt.Errorf("list feed materialized: %w", err)
	}
	if len(posts) > 0 {
		return posts, nil
	}

	// Fallback: live join query for new accounts / pre-fanout followers.
	posts, err = s.store.ListFeed(ctx, viewerID, cursor, limit)
	if err != nil {
		return nil, fmt.Errorf("list feed: %w", err)
	}
	return posts, nil
}
