package events

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/meowmix1337/argus/backend/internal/model"
)

// backfillLimit is the maximum number of historical posts to backfill per follow event.
const backfillLimit = 50

// BackfillPostStore fetches recent post references for a given author.
type BackfillPostStore interface {
	ListPostIDsByAuthor(ctx context.Context, authorID string, limit int) ([]model.PostRef, error)
}

// FollowBackfillConsumer consumes user.followed events and backfills the
// follower's user_feed with the followed user's most recent posts.
type FollowBackfillConsumer struct {
	postStore BackfillPostStore
	feedStore FanoutFeedStore
}

// NewFollowBackfillConsumer creates a new FollowBackfillConsumer.
func NewFollowBackfillConsumer(postStore BackfillPostStore, feedStore FanoutFeedStore) *FollowBackfillConsumer {
	return &FollowBackfillConsumer{postStore: postStore, feedStore: feedStore}
}

// Topic implements MessageHandler.
func (c *FollowBackfillConsumer) Topic() string { return TopicUserFollowed }

// Channel implements MessageHandler.
func (c *FollowBackfillConsumer) Channel() string { return "feed-backfill" }

// Process implements MessageHandler. It unmarshals the envelope and runs the backfill.
func (c *FollowBackfillConsumer) Process(body []byte) error {
	var evt rawEventEnvelope
	if err := json.Unmarshal(body, &evt); err != nil {
		return fmt.Errorf("unmarshal envelope: %w", err)
	}
	if evt.Version != 1 {
		slog.Warn("follow backfill: unknown envelope version, skipping", "version", evt.Version)
		return nil
	}
	var payload UserFollowedPayload
	if err := json.Unmarshal(evt.Payload, &payload); err != nil {
		return fmt.Errorf("unmarshal user followed payload: %w", err)
	}
	return c.process(payload)
}

// process performs the backfill for a single UserFollowedPayload.
func (c *FollowBackfillConsumer) process(payload UserFollowedPayload) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	posts, err := c.postStore.ListPostIDsByAuthor(ctx, payload.FollowingID, backfillLimit)
	if err != nil {
		return fmt.Errorf("list posts for backfill (author %s): %w", payload.FollowingID, err)
	}
	if len(posts) == 0 {
		return nil
	}

	rows := make([]model.UserFeedRow, 0, len(posts))
	for _, p := range posts {
		id, err := uuid.NewV7()
		if err != nil {
			slog.Error("follow backfill: failed to generate row id, skipping post",
				"post_id", p.ID, "error", err)
			continue
		}
		rows = append(rows, model.UserFeedRow{
			ID:        id.String(),
			UserID:    payload.FollowerID,
			PostID:    p.ID,
			CreatedAt: p.CreatedAt,
		})
	}

	if len(rows) == 0 {
		return nil
	}

	if err := c.feedStore.BulkInsertUserFeed(ctx, rows); err != nil {
		return fmt.Errorf("bulk insert backfill rows for follower %s: %w", payload.FollowerID, err)
	}
	return nil
}
