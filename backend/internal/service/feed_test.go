package service

import (
	"context"
	"errors"
	"testing"

	"github.com/meowmix1337/argus/backend/internal/model"
)

// fakeFeedStore is an in-memory FeedStore for service-layer tests.
type fakeFeedStore struct {
	posts   []model.Post
	listErr error
}

func newFakeFeedStore(posts ...model.Post) *fakeFeedStore {
	return &fakeFeedStore{posts: posts}
}

func (f *fakeFeedStore) ListFeed(_ context.Context, _ string, cursor *model.FeedCursor, limit int) ([]model.Post, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	// Simple cursor simulation: skip posts until we find one past the cursor.
	out := f.posts
	if cursor != nil {
		var filtered []model.Post
		pastCursor := false
		for _, p := range out {
			if pastCursor {
				filtered = append(filtered, p)
			}
			if p.CreatedAt == cursor.CreatedAt && p.ID == cursor.ID {
				pastCursor = true
			}
		}
		out = filtered
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func TestFeedService_ListFeed_Success(t *testing.T) {
	posts := []model.Post{
		{ID: "p1", UserID: "u1", Content: "first", CreatedAt: "2025-01-01T00:00:02.000Z"},
		{ID: "p2", UserID: "u2", Content: "second", CreatedAt: "2025-01-01T00:00:01.000Z"},
	}
	svc := NewFeedService(newFakeFeedStore(posts...))

	result, err := svc.ListFeed(context.Background(), "viewer1", nil, 10)
	if err != nil {
		t.Fatalf("ListFeed: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("len(result) = %d, want 2", len(result))
	}
}

func TestFeedService_ListFeed_WithCursor(t *testing.T) {
	posts := []model.Post{
		{ID: "p1", UserID: "u1", Content: "first", CreatedAt: "2025-01-01T00:00:02.000Z"},
		{ID: "p2", UserID: "u2", Content: "second", CreatedAt: "2025-01-01T00:00:01.000Z"},
	}
	svc := NewFeedService(newFakeFeedStore(posts...))

	cursor := &model.FeedCursor{CreatedAt: "2025-01-01T00:00:02.000Z", ID: "p1"}
	result, err := svc.ListFeed(context.Background(), "viewer1", cursor, 10)
	if err != nil {
		t.Fatalf("ListFeed with cursor: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("len(result) = %d, want 1 (after cursor)", len(result))
	}
}

func TestFeedService_ListFeed_Empty(t *testing.T) {
	svc := NewFeedService(newFakeFeedStore())

	result, err := svc.ListFeed(context.Background(), "viewer1", nil, 10)
	if err != nil {
		t.Fatalf("ListFeed: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("len(result) = %d, want 0", len(result))
	}
}

func TestFeedService_ListFeed_StoreError_Propagates(t *testing.T) {
	store := newFakeFeedStore()
	store.listErr = errors.New("db failure")
	svc := NewFeedService(store)

	_, err := svc.ListFeed(context.Background(), "viewer1", nil, 10)
	if err == nil {
		t.Error("expected store error to propagate, got nil")
	}
}
