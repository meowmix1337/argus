package events

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/meowmix1337/argus/backend/internal/model"
	"github.com/meowmix1337/argus/backend/internal/platform/publisher"
)

// buildUserFollowedEnvelope marshals a UserFollowedPayload into a raw EventEnvelope JSON message.
func buildUserFollowedEnvelope(t *testing.T, payload publisher.UserFollowedPayload) []byte {
	t.Helper()
	env := publisher.NewEnvelope(publisher.TopicUserFollowed, payload)
	b, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}
	return b
}

func TestFollowNotificationConsumer_Process_ValidPayload(t *testing.T) {
	notifier := &fakeNotifCreator{}
	prefs := &fakePrefsReader{prefs: map[string]model.SocialNotificationPrefs{}}
	consumer := NewFollowNotificationConsumer(notifier, prefs)

	body := buildUserFollowedEnvelope(t, publisher.UserFollowedPayload{
		FollowerID:   "follower-1",
		FollowingID:  "target-1",
		FollowerName: "Alice",
	})
	if err := consumer.Process(body); err != nil {
		t.Fatalf("Process: %v", err)
	}
	if len(notifier.calls) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(notifier.calls))
	}
	call := notifier.calls[0]
	if call.userID != "target-1" {
		t.Errorf("notification userID = %q, want target-1", call.userID)
	}
	if call.title != "Alice started following you" {
		t.Errorf("title = %q, want %q", call.title, "Alice started following you")
	}
	if call.eventTypeID != "social.new_follower" {
		t.Errorf("eventTypeID = %q, want social.new_follower", call.eventTypeID)
	}
	if call.referenceID == nil || *call.referenceID != "follower-1" {
		t.Errorf("referenceID = %v, want follower-1", call.referenceID)
	}
}

func TestFollowNotificationConsumer_Process_MutedUserSkipped(t *testing.T) {
	notifier := &fakeNotifCreator{}
	prefs := &fakePrefsReader{prefs: map[string]model.SocialNotificationPrefs{
		"target-1": {UserID: "target-1", MuteFollows: true},
	}}
	consumer := NewFollowNotificationConsumer(notifier, prefs)

	body := buildUserFollowedEnvelope(t, publisher.UserFollowedPayload{
		FollowerID:   "follower-1",
		FollowingID:  "target-1",
		FollowerName: "Bob",
	})
	if err := consumer.Process(body); err != nil {
		t.Fatalf("Process: %v", err)
	}
	if len(notifier.calls) != 0 {
		t.Errorf("expected 0 notifications (muted), got %d", len(notifier.calls))
	}
}

func TestFollowNotificationConsumer_Process_MalformedJSON(t *testing.T) {
	consumer := NewFollowNotificationConsumer(&fakeNotifCreator{}, &fakePrefsReader{})
	err := consumer.Process([]byte(`not-json`))
	if err == nil {
		t.Error("expected error for malformed JSON, got nil")
	}
}

func TestFollowNotificationConsumer_Process_UnknownVersion(t *testing.T) {
	env := publisher.EventEnvelope{Version: 99, Type: publisher.TopicUserFollowed, Payload: map[string]string{}}
	body, _ := json.Marshal(env)
	consumer := NewFollowNotificationConsumer(&fakeNotifCreator{}, &fakePrefsReader{})
	if err := consumer.Process(body); err != nil {
		t.Errorf("unexpected error for unknown version: %v", err)
	}
}

func TestFollowNotificationConsumer_Process_PrefsError_ReturnsError(t *testing.T) {
	prefs := &fakePrefsReader{}
	prefs.err = context.DeadlineExceeded
	consumer := NewFollowNotificationConsumer(&fakeNotifCreator{}, prefs)

	body := buildUserFollowedEnvelope(t, publisher.UserFollowedPayload{
		FollowerID:   "f1",
		FollowingID:  "t1",
		FollowerName: "Alice",
	})
	err := consumer.Process(body)
	if err == nil {
		t.Error("expected prefs error to propagate, got nil")
	}
}
