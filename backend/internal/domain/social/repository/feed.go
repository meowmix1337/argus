package repository

import (
	"context"

	"github.com/meowmix1337/argus/backend/internal/model"
)

// FeedStore defines the data-access contract for the social feed.
type FeedStore interface {
	// ListFeed returns posts via a live join (followers + own posts).
	// Used as the fallback when user_feed has no rows for the viewer.
	ListFeed(ctx context.Context, viewerID string, cursor *model.FeedCursor, limit int) ([]model.Post, error)
	// ListFeedMaterialized reads from the pre-computed user_feed table.
	ListFeedMaterialized(ctx context.Context, viewerID string, cursor *model.FeedCursor, limit int) ([]model.Post, error)
	// BulkInsertUserFeed inserts fanout rows into user_feed (INSERT OR IGNORE).
	BulkInsertUserFeed(ctx context.Context, rows []model.UserFeedRow) error
}
