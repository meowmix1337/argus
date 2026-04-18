package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/meowmix1337/argus/backend/internal/model"
	platformevents "github.com/meowmix1337/argus/backend/internal/platform/events"
)

// fakeFollowStore implements FanoutFollowStore for tests.
type fakeFollowStore struct {
	ids []string
	err error
}

func (f *fakeFollowStore) GetFollowerIDs(_ context.Context, _ string) ([]string, error) {
	return f.ids, f.err
}

// fakeFanoutFeedStore implements FanoutFeedStore for tests.
type fakeFanoutFeedStore struct {
	inserted []model.UserFeedRow
	err      error
}

func (f *fakeFanoutFeedStore) BulkInsertUserFeed(_ context.Context, rows []model.UserFeedRow) error {
	if f.err != nil {
		return f.err
	}
	f.inserted = append(f.inserted, rows...)
	return nil
}

// buildEnvelope marshals a PostCreatedPayload into a raw EventEnvelope JSON message.
func buildEnvelope(t *testing.T, payload platformevents.PostCreatedPayload) []byte {
	t.Helper()
	env := platformevents.NewEnvelope(platformevents.TopicPostCreated, payload)
	b, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}
	return b
}

func TestFeedFanoutConsumer_Process_ValidPayload(t *testing.T) {
	followers := &fakeFollowStore{ids: []string{"follower-1", "follower-2"}}
	feedStore := &fakeFanoutFeedStore{}
	consumer := NewFeedFanoutConsumer(followers, feedStore)

	body := buildEnvelope(t, platformevents.PostCreatedPayload{PostID: "post-abc", UserID: "author-1"})
	if err := consumer.Process(body); err != nil {
		t.Fatalf("Process: %v", err)
	}

	// 2 followers + the author themselves = 3 rows.
	if len(feedStore.inserted) != 3 {
		t.Fatalf("inserted %d rows, want 3 (2 followers + author)", len(feedStore.inserted))
	}
	userIDs := map[string]bool{}
	for _, row := range feedStore.inserted {
		if row.PostID != "post-abc" {
			t.Errorf("row.PostID = %q, want post-abc", row.PostID)
		}
		if row.ID == "" {
			t.Error("row.ID is empty, expected a UUID")
		}
		userIDs[row.UserID] = true
	}
	for _, want := range []string{"follower-1", "follower-2", "author-1"} {
		if !userIDs[want] {
			t.Errorf("missing user ID %q in inserted rows", want)
		}
	}
}

func TestFeedFanoutConsumer_Process_NoFollowers(t *testing.T) {
	followers := &fakeFollowStore{ids: []string{}}
	feedStore := &fakeFanoutFeedStore{}
	consumer := NewFeedFanoutConsumer(followers, feedStore)

	body := buildEnvelope(t, platformevents.PostCreatedPayload{PostID: "post-xyz", UserID: "author-2"})
	if err := consumer.Process(body); err != nil {
		t.Fatalf("Process: %v", err)
	}
	// Author always gets their own row even with no followers.
	if len(feedStore.inserted) != 1 {
		t.Errorf("inserted %d rows, want 1 (author only)", len(feedStore.inserted))
	}
	if feedStore.inserted[0].UserID != "author-2" {
		t.Errorf("inserted row UserID = %q, want author-2", feedStore.inserted[0].UserID)
	}
}

func TestFeedFanoutConsumer_Process_MalformedJSON(t *testing.T) {
	consumer := NewFeedFanoutConsumer(&fakeFollowStore{}, &fakeFanoutFeedStore{})

	err := consumer.Process([]byte(`not-json`))
	if err == nil {
		t.Error("expected error for malformed JSON, got nil")
	}
}

func TestFeedFanoutConsumer_Process_UnknownVersion(t *testing.T) {
	env := platformevents.EventEnvelope{Version: 99, Type: platformevents.TopicPostCreated, Payload: map[string]string{}}
	body, _ := json.Marshal(env)

	consumer := NewFeedFanoutConsumer(&fakeFollowStore{}, &fakeFanoutFeedStore{})
	if err := consumer.Process(body); err != nil {
		t.Errorf("unexpected error for unknown version (should be skipped): %v", err)
	}
}

func TestFeedFanoutConsumer_Process_FollowStoreError(t *testing.T) {
	followers := &fakeFollowStore{err: errors.New("db error")}
	consumer := NewFeedFanoutConsumer(followers, &fakeFanoutFeedStore{})

	body := buildEnvelope(t, platformevents.PostCreatedPayload{PostID: "post-1", UserID: "author-1"})
	if err := consumer.Process(body); err == nil {
		t.Error("expected error from follow store, got nil")
	}
}

func TestFeedFanoutConsumer_Process_FeedStoreError(t *testing.T) {
	followers := &fakeFollowStore{ids: []string{"follower-1"}}
	feedStore := &fakeFanoutFeedStore{err: errors.New("insert failed")}
	consumer := NewFeedFanoutConsumer(followers, feedStore)

	body := buildEnvelope(t, platformevents.PostCreatedPayload{PostID: "post-1", UserID: "author-1"})
	if err := consumer.Process(body); err == nil {
		t.Error("expected error from feed store, got nil")
	}
}
