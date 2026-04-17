package events

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/meowmix1337/argus/backend/internal/model"
)

// fakeNotifCreator records CreateForUser calls for test assertions.
type fakeNotifCreator struct {
	calls []notifCall
	err   error
}

type notifCall struct {
	userID      string
	eventTypeID string
	title       string
	referenceID *string
}

func (f *fakeNotifCreator) CreateForUser(_ context.Context,
	userID, _, eventTypeID, title string,
	_, _, referenceID *string,
) error {
	if f.err != nil {
		return f.err
	}
	f.calls = append(f.calls, notifCall{
		userID:      userID,
		eventTypeID: eventTypeID,
		title:       title,
		referenceID: referenceID,
	})
	return nil
}

// fakePrefsReader returns configurable prefs per user ID.
type fakePrefsReader struct {
	prefs map[string]model.SocialNotificationPrefs
	err   error
}

func (f *fakePrefsReader) GetPrefs(_ context.Context, userID string) (model.SocialNotificationPrefs, error) {
	if f.err != nil {
		return model.SocialNotificationPrefs{}, f.err
	}
	if p, ok := f.prefs[userID]; ok {
		return p, nil
	}
	return model.SocialNotificationPrefs{UserID: userID}, nil
}

// Note: buildEnvelope is defined in feed_fanout_test.go (same package) and is reused here.

func TestFollowerNotificationConsumer_Process_ValidPayload(t *testing.T) {
	followStore := &fakeFollowStore{ids: []string{"follower-1", "follower-2"}}
	notifier := &fakeNotifCreator{}
	prefs := &fakePrefsReader{prefs: map[string]model.SocialNotificationPrefs{}}
	consumer := NewFollowerNotificationConsumer(followStore, notifier, prefs)

	body := buildEnvelope(t, PostCreatedPayload{
		PostID:         "post-1",
		UserID:         "author-1",
		AuthorName:     "Alice",
		ContentPreview: "Hello world",
	})
	if err := consumer.Process(body); err != nil {
		t.Fatalf("Process: %v", err)
	}
	if len(notifier.calls) != 2 {
		t.Fatalf("expected 2 notifications (one per follower), got %d", len(notifier.calls))
	}
	for _, call := range notifier.calls {
		if call.title != "Alice posted something" {
			t.Errorf("title = %q, want %q", call.title, "Alice posted something")
		}
		if call.eventTypeID != "social.post.created" {
			t.Errorf("eventTypeID = %q, want social.post.created", call.eventTypeID)
		}
		if call.referenceID == nil || *call.referenceID != "post-1" {
			t.Errorf("referenceID = %v, want post-1", call.referenceID)
		}
	}
}

func TestFollowerNotificationConsumer_Process_MutedFollowerSkipped(t *testing.T) {
	followStore := &fakeFollowStore{ids: []string{"muted-user", "active-user"}}
	notifier := &fakeNotifCreator{}
	prefs := &fakePrefsReader{prefs: map[string]model.SocialNotificationPrefs{
		"muted-user": {UserID: "muted-user", MutePosts: true},
	}}
	consumer := NewFollowerNotificationConsumer(followStore, notifier, prefs)

	body := buildEnvelope(t, PostCreatedPayload{
		PostID:         "post-2",
		UserID:         "author-1",
		AuthorName:     "Bob",
		ContentPreview: "content",
	})
	if err := consumer.Process(body); err != nil {
		t.Fatalf("Process: %v", err)
	}
	if len(notifier.calls) != 1 {
		t.Fatalf("expected 1 notification (muted user skipped), got %d", len(notifier.calls))
	}
	if notifier.calls[0].userID != "active-user" {
		t.Errorf("notification sent to %q, want active-user", notifier.calls[0].userID)
	}
}

func TestFollowerNotificationConsumer_Process_NoFollowers(t *testing.T) {
	followStore := &fakeFollowStore{ids: []string{}}
	notifier := &fakeNotifCreator{}
	consumer := NewFollowerNotificationConsumer(followStore, notifier, &fakePrefsReader{})

	body := buildEnvelope(t, PostCreatedPayload{PostID: "post-3", UserID: "author-1"})
	if err := consumer.Process(body); err != nil {
		t.Fatalf("Process: %v", err)
	}
	if len(notifier.calls) != 0 {
		t.Errorf("expected 0 notifications for no-follower post, got %d", len(notifier.calls))
	}
}

func TestFollowerNotificationConsumer_Process_MalformedJSON(t *testing.T) {
	consumer := NewFollowerNotificationConsumer(&fakeFollowStore{}, &fakeNotifCreator{}, &fakePrefsReader{})
	err := consumer.Process([]byte(`not-json`))
	if err == nil {
		t.Error("expected error for malformed JSON, got nil")
	}
}

func TestFollowerNotificationConsumer_Process_UnknownVersion(t *testing.T) {
	env := EventEnvelope{Version: 99, Type: TopicPostCreated, Payload: map[string]string{}}
	body, _ := json.Marshal(env)
	consumer := NewFollowerNotificationConsumer(&fakeFollowStore{}, &fakeNotifCreator{}, &fakePrefsReader{})
	if err := consumer.Process(body); err != nil {
		t.Errorf("unexpected error for unknown version: %v", err)
	}
}
