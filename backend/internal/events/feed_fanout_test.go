package events

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/meowmix1337/argus/backend/internal/model"
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
func buildEnvelope(t *testing.T, payload PostCreatedPayload) []byte {
	t.Helper()
	env := NewEnvelope(TopicPostCreated, payload)
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

	body := buildEnvelope(t, PostCreatedPayload{PostID: "post-abc", UserID: "author-1"})
	if err := consumer.Process(body); err != nil {
		t.Fatalf("Process: %v", err)
	}

	if len(feedStore.inserted) != 2 {
		t.Fatalf("inserted %d rows, want 2", len(feedStore.inserted))
	}
	for _, row := range feedStore.inserted {
		if row.PostID != "post-abc" {
			t.Errorf("row.PostID = %q, want post-abc", row.PostID)
		}
		if row.ID == "" {
			t.Error("row.ID is empty, expected a UUID")
		}
	}
	userIDs := map[string]bool{feedStore.inserted[0].UserID: true, feedStore.inserted[1].UserID: true}
	if !userIDs["follower-1"] || !userIDs["follower-2"] {
		t.Errorf("unexpected user IDs in inserted rows: %v", feedStore.inserted)
	}
}

func TestFeedFanoutConsumer_Process_NoFollowers(t *testing.T) {
	followers := &fakeFollowStore{ids: []string{}}
	feedStore := &fakeFanoutFeedStore{}
	consumer := NewFeedFanoutConsumer(followers, feedStore)

	body := buildEnvelope(t, PostCreatedPayload{PostID: "post-xyz", UserID: "author-2"})
	if err := consumer.Process(body); err != nil {
		t.Fatalf("Process: %v", err)
	}
	if len(feedStore.inserted) != 0 {
		t.Errorf("inserted %d rows, want 0", len(feedStore.inserted))
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
	env := EventEnvelope{Version: 99, Type: TopicPostCreated, Payload: map[string]string{}}
	body, _ := json.Marshal(env)

	consumer := NewFeedFanoutConsumer(&fakeFollowStore{}, &fakeFanoutFeedStore{})
	if err := consumer.Process(body); err != nil {
		t.Errorf("unexpected error for unknown version (should be skipped): %v", err)
	}
}

func TestFeedFanoutConsumer_Process_FollowStoreError(t *testing.T) {
	followers := &fakeFollowStore{err: errors.New("db error")}
	consumer := NewFeedFanoutConsumer(followers, &fakeFanoutFeedStore{})

	body := buildEnvelope(t, PostCreatedPayload{PostID: "post-1", UserID: "author-1"})
	if err := consumer.Process(body); err == nil {
		t.Error("expected error from follow store, got nil")
	}
}

func TestFeedFanoutConsumer_Process_FeedStoreError(t *testing.T) {
	followers := &fakeFollowStore{ids: []string{"follower-1"}}
	feedStore := &fakeFanoutFeedStore{err: errors.New("insert failed")}
	consumer := NewFeedFanoutConsumer(followers, feedStore)

	body := buildEnvelope(t, PostCreatedPayload{PostID: "post-1", UserID: "author-1"})
	if err := consumer.Process(body); err == nil {
		t.Error("expected error from feed store, got nil")
	}
}
