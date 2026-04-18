package events

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"
)

// FollowerNotificationConsumer consumes post.created events and creates
// a notification for each follower of the post author.
type FollowerNotificationConsumer struct {
	followStore  FanoutFollowStore
	notifCreator NotificationCreator
	prefsReader  SocialPrefsReader
}

// NewFollowerNotificationConsumer creates a new FollowerNotificationConsumer.
func NewFollowerNotificationConsumer(
	followStore FanoutFollowStore,
	notifCreator NotificationCreator,
	prefsReader SocialPrefsReader,
) *FollowerNotificationConsumer {
	return &FollowerNotificationConsumer{
		followStore:  followStore,
		notifCreator: notifCreator,
		prefsReader:  prefsReader,
	}
}

// Topic implements MessageHandler.
func (c *FollowerNotificationConsumer) Topic() string { return TopicPostCreated }

// Channel implements MessageHandler.
func (c *FollowerNotificationConsumer) Channel() string { return "follower-notifications" }

// Process implements MessageHandler.
func (c *FollowerNotificationConsumer) Process(body []byte) error {
	var evt rawEventEnvelope
	if err := json.Unmarshal(body, &evt); err != nil {
		return fmt.Errorf("unmarshal envelope: %w", err)
	}
	if evt.Version != 1 {
		slog.Warn("follower notification: unknown envelope version, skipping", "version", evt.Version)
		return nil
	}
	var payload PostCreatedPayload
	if err := json.Unmarshal(evt.Payload, &payload); err != nil {
		return fmt.Errorf("unmarshal post created payload: %w", err)
	}
	return c.process(payload)
}

// process fans out notifications to each follower of the post author.
func (c *FollowerNotificationConsumer) process(payload PostCreatedPayload) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	followerIDs, err := c.followStore.GetFollowerIDs(ctx, payload.UserID)
	if err != nil {
		return fmt.Errorf("get followers for user %s: %w", payload.UserID, err)
	}
	if len(followerIDs) == 0 {
		return nil
	}

	title := payload.AuthorName + " posted something"
	body := &payload.ContentPreview
	postID := payload.PostID

	for _, followerID := range followerIDs {
		prefs, err := c.prefsReader.GetPrefs(ctx, followerID)
		if err != nil {
			slog.Warn("follower notification: failed to get prefs, skipping",
				"follower_id", followerID, "error", err)
			continue
		}
		if prefs.MutePosts {
			continue
		}
		if err := c.notifCreator.CreateForUser(ctx,
			followerID, "social", "social.post.created", title, body, nil, &postID,
		); err != nil {
			slog.Warn("follower notification: failed to create notification",
				"follower_id", followerID, "post_id", postID, "error", err)
		}
	}
	return nil
}
