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

// FanoutFollowStore looks up follower IDs for a given user.
type FanoutFollowStore interface {
	GetFollowerIDs(ctx context.Context, userID string) ([]string, error)
}

// FanoutFeedStore writes rows to the materialized user_feed table.
type FanoutFeedStore interface {
	BulkInsertUserFeed(ctx context.Context, rows []model.UserFeedRow) error
}

// FeedFanoutConsumer consumes post.created events and fans out to user_feed.
type FeedFanoutConsumer struct {
	followStore FanoutFollowStore
	feedStore   FanoutFeedStore
}

// NewFeedFanoutConsumer creates a new FeedFanoutConsumer.
func NewFeedFanoutConsumer(followStore FanoutFollowStore, feedStore FanoutFeedStore) *FeedFanoutConsumer {
	return &FeedFanoutConsumer{followStore: followStore, feedStore: feedStore}
}

// Topic implements MessageHandler.
func (c *FeedFanoutConsumer) Topic() string { return TopicPostCreated }

// Channel implements MessageHandler.
func (c *FeedFanoutConsumer) Channel() string { return "feed-fanout" }

// Process implements MessageHandler. It unmarshals the envelope and fans out.
func (c *FeedFanoutConsumer) Process(body []byte) error {
	var env EventEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		return fmt.Errorf("unmarshal envelope: %w", err)
	}

	if env.Version != 1 {
		slog.Warn("feed fanout: unknown envelope version, skipping", "version", env.Version)
		return nil
	}

	// Re-marshal the generic payload map back to JSON, then decode into the concrete type.
	payloadBytes, err := json.Marshal(env.Payload)
	if err != nil {
		return fmt.Errorf("re-marshal payload: %w", err)
	}
	var payload PostCreatedPayload
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return fmt.Errorf("unmarshal post created payload: %w", err)
	}

	return c.process(payload)
}

// process performs the fan-out for a single PostCreatedPayload.
// It is unexported but called by Process; tests in the same package invoke it directly.
func (c *FeedFanoutConsumer) process(payload PostCreatedPayload) error {
	ctx := context.Background()

	followerIDs, err := c.followStore.GetFollowerIDs(ctx, payload.UserID)
	if err != nil {
		return fmt.Errorf("get followers of user %s: %w", payload.UserID, err)
	}

	// Fan out to followers plus the author themselves so their own posts appear
	// in their materialized feed (matching the live-join fallback behaviour).
	recipients := make([]string, 0, len(followerIDs)+1)
	recipients = append(recipients, followerIDs...)
	recipients = append(recipients, payload.UserID)

	now := time.Now().UTC().Format(time.RFC3339Nano)
	rows := make([]model.UserFeedRow, 0, len(recipients))
	for _, recipientID := range recipients {
		id, err := uuid.NewV7()
		if err != nil {
			slog.Error("feed fanout: failed to generate row id, skipping recipient",
				"recipient_id", recipientID, "error", err)
			continue
		}
		rows = append(rows, model.UserFeedRow{
			ID:        id.String(),
			UserID:    recipientID,
			PostID:    payload.PostID,
			CreatedAt: now,
		})
	}

	if len(rows) == 0 {
		return nil
	}

	if err := c.feedStore.BulkInsertUserFeed(ctx, rows); err != nil {
		return fmt.Errorf("bulk insert user feed for post %s: %w", payload.PostID, err)
	}
	return nil
}
