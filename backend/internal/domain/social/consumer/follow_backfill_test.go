package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/meowmix1337/argus/backend/internal/model"
	"github.com/meowmix1337/argus/backend/internal/platform/publisher"
)

// fakeBackfillPostStore implements BackfillPostStore for tests.
type fakeBackfillPostStore struct {
	refs []model.PostRef
	err  error
}

func (f *fakeBackfillPostStore) ListPostIDsByAuthor(_ context.Context, _ string, _ int) ([]model.PostRef, error) {
	return f.refs, f.err
}

// buildFollowEnvelope marshals a UserFollowedPayload into a raw EventEnvelope JSON message.
func buildFollowEnvelope(t *testing.T, payload publisher.UserFollowedPayload) []byte {
	t.Helper()
	env := publisher.NewEnvelope(publisher.TopicUserFollowed, payload)
	b, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("marshal follow envelope: %v", err)
	}
	return b
}

func TestFollowBackfill_NoPosts_NoRowsInserted(t *testing.T) {
	postStore := &fakeBackfillPostStore{refs: []model.PostRef{}}
	feedStore := &fakeFanoutFeedStore{}
	consumer := NewFollowBackfillConsumer(postStore, feedStore)

	body := buildFollowEnvelope(t, publisher.UserFollowedPayload{FollowerID: "u1", FollowingID: "u2"})
	if err := consumer.Process(body); err != nil {
		t.Fatalf("Process: %v", err)
	}
	if len(feedStore.inserted) != 0 {
		t.Errorf("inserted %d rows, want 0 (author has no posts)", len(feedStore.inserted))
	}
}

func TestFollowBackfill_BackfillsRecentPosts(t *testing.T) {
	posts := []model.PostRef{
		{ID: "post-1", CreatedAt: "2025-01-03T00:00:00Z"},
		{ID: "post-2", CreatedAt: "2025-01-02T00:00:00Z"},
		{ID: "post-3", CreatedAt: "2025-01-01T00:00:00Z"},
	}
	postStore := &fakeBackfillPostStore{refs: posts}
	feedStore := &fakeFanoutFeedStore{}
	consumer := NewFollowBackfillConsumer(postStore, feedStore)

	body := buildFollowEnvelope(t, publisher.UserFollowedPayload{FollowerID: "follower-1", FollowingID: "author-1"})
	if err := consumer.Process(body); err != nil {
		t.Fatalf("Process: %v", err)
	}

	if len(feedStore.inserted) != 3 {
		t.Fatalf("inserted %d rows, want 3", len(feedStore.inserted))
	}
	postIDs := map[string]bool{}
	for _, row := range feedStore.inserted {
		if row.UserID != "follower-1" {
			t.Errorf("row.UserID = %q, want follower-1", row.UserID)
		}
		if row.ID == "" {
			t.Error("row.ID is empty, expected a UUID")
		}
		postIDs[row.PostID] = true
	}
	for _, want := range []string{"post-1", "post-2", "post-3"} {
		if !postIDs[want] {
			t.Errorf("missing post ID %q in inserted rows", want)
		}
	}
}

func TestFollowBackfill_PostCreatedAtPreserved(t *testing.T) {
	posts := []model.PostRef{
		{ID: "post-1", CreatedAt: "2025-06-15T12:00:00Z"},
	}
	postStore := &fakeBackfillPostStore{refs: posts}
	feedStore := &fakeFanoutFeedStore{}
	consumer := NewFollowBackfillConsumer(postStore, feedStore)

	body := buildFollowEnvelope(t, publisher.UserFollowedPayload{FollowerID: "f1", FollowingID: "a1"})
	if err := consumer.Process(body); err != nil {
		t.Fatalf("Process: %v", err)
	}
	if feedStore.inserted[0].CreatedAt != "2025-06-15T12:00:00Z" {
		t.Errorf("CreatedAt = %q, want post's original created_at", feedStore.inserted[0].CreatedAt)
	}
}

func TestFollowBackfill_PostStoreError_Propagates(t *testing.T) {
	postStore := &fakeBackfillPostStore{err: errors.New("db failure")}
	consumer := NewFollowBackfillConsumer(postStore, &fakeFanoutFeedStore{})

	body := buildFollowEnvelope(t, publisher.UserFollowedPayload{FollowerID: "f1", FollowingID: "a1"})
	if err := consumer.Process(body); err == nil {
		t.Error("expected error from post store, got nil")
	}
}

func TestFollowBackfill_FeedStoreError_Propagates(t *testing.T) {
	postStore := &fakeBackfillPostStore{refs: []model.PostRef{{ID: "p1", CreatedAt: "2025-01-01T00:00:00Z"}}}
	feedStore := &fakeFanoutFeedStore{err: errors.New("insert failed")}
	consumer := NewFollowBackfillConsumer(postStore, feedStore)

	body := buildFollowEnvelope(t, publisher.UserFollowedPayload{FollowerID: "f1", FollowingID: "a1"})
	if err := consumer.Process(body); err == nil {
		t.Error("expected error from feed store, got nil")
	}
}

func TestFollowBackfill_MalformedJSON(t *testing.T) {
	consumer := NewFollowBackfillConsumer(&fakeBackfillPostStore{}, &fakeFanoutFeedStore{})
	if err := consumer.Process([]byte(`not-json`)); err == nil {
		t.Error("expected error for malformed JSON, got nil")
	}
}

func TestFollowBackfill_UnknownVersion(t *testing.T) {
	env := publisher.EventEnvelope{Version: 99, Type: publisher.TopicUserFollowed, Payload: map[string]string{}}
	body, _ := json.Marshal(env)
	consumer := NewFollowBackfillConsumer(&fakeBackfillPostStore{}, &fakeFanoutFeedStore{})
	if err := consumer.Process(body); err != nil {
		t.Errorf("unexpected error for unknown version (should skip): %v", err)
	}
}
