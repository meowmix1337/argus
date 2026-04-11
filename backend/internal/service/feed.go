package service

import (
	"context"
	"fmt"

	"github.com/meowmix1337/argus/backend/internal/model"
)

// FeedStore defines the data-access contract for the social feed.
type FeedStore interface {
	// ListFeed returns posts from users the viewer follows (plus their own),
	// ordered by created_at DESC with cursor-based pagination.
	// cursor may be nil for the first page.
	ListFeed(ctx context.Context, viewerID string, cursor *model.FeedCursor, limit int) ([]model.Post, error)
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
func (s *FeedService) ListFeed(ctx context.Context, viewerID string, cursor *model.FeedCursor, limit int) ([]model.Post, error) {
	posts, err := s.store.ListFeed(ctx, viewerID, cursor, limit)
	if err != nil {
		return nil, fmt.Errorf("list feed: %w", err)
	}
	return posts, nil
}
