package events

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"
)

// FollowNotificationConsumer consumes user.followed events and notifies
// the followed user that someone started following them.
type FollowNotificationConsumer struct {
	notifCreator NotificationCreator
	prefsReader  SocialPrefsReader
}

// NewFollowNotificationConsumer creates a new FollowNotificationConsumer.
func NewFollowNotificationConsumer(
	notifCreator NotificationCreator,
	prefsReader SocialPrefsReader,
) *FollowNotificationConsumer {
	return &FollowNotificationConsumer{
		notifCreator: notifCreator,
		prefsReader:  prefsReader,
	}
}

// Topic implements MessageHandler.
func (c *FollowNotificationConsumer) Topic() string { return TopicUserFollowed }

// Channel implements MessageHandler.
func (c *FollowNotificationConsumer) Channel() string { return "follow-notifications" }

// Process implements MessageHandler.
func (c *FollowNotificationConsumer) Process(body []byte) error {
	var evt rawEventEnvelope
	if err := json.Unmarshal(body, &evt); err != nil {
		return fmt.Errorf("unmarshal envelope: %w", err)
	}
	if evt.Version != 1 {
		slog.Warn("follow notification: unknown envelope version, skipping", "version", evt.Version)
		return nil
	}
	var payload UserFollowedPayload
	if err := json.Unmarshal(evt.Payload, &payload); err != nil {
		return fmt.Errorf("unmarshal user followed payload: %w", err)
	}
	return c.process(payload)
}

// process sends a notification to the followed user if they have not muted follow notifications.
func (c *FollowNotificationConsumer) process(payload UserFollowedPayload) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	prefs, err := c.prefsReader.GetPrefs(ctx, payload.FollowingID)
	if err != nil {
		return fmt.Errorf("get prefs for user %s: %w", payload.FollowingID, err)
	}
	if prefs.MuteFollows {
		return nil
	}

	title := payload.FollowerName + " started following you"
	followerID := payload.FollowerID

	if err := c.notifCreator.CreateForUser(ctx,
		payload.FollowingID, "social", "social.new_follower", title, nil, nil, &followerID,
	); err != nil {
		slog.Warn("follow notification: failed to create notification",
			"following_id", payload.FollowingID, "follower_id", followerID, "error", err)
	}
	return nil
}
