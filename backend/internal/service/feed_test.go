package service

import (
	"context"
	"errors"
	"testing"

	"github.com/meowmix1337/argus/backend/internal/model"
)

// fakeFeedStore is an in-memory FeedStore for service-layer tests.
type fakeFeedStore struct {
	posts        []model.Post
	materialRows []model.Post
	feedCount    int
	listErr      error
	countErr     error
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

func (f *fakeFeedStore) ListFeedMaterialized(_ context.Context, _ string, cursor *model.FeedCursor, limit int) ([]model.Post, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	out := f.materialRows
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

func (f *fakeFeedStore) CountUserFeedForUser(_ context.Context, _ string) (int, error) {
	if f.countErr != nil {
		return 0, f.countErr
	}
	return f.feedCount, nil
}

func (f *fakeFeedStore) BulkInsertUserFeed(_ context.Context, _ []model.UserFeedRow) error {
	return nil
}

// --- tests ---

func TestFeedService_ListFeed_FallbackLiveJoin(t *testing.T) {
	posts := []model.Post{
		{ID: "p1", UserID: "u1", Content: "first", CreatedAt: "2025-01-01T00:00:02.000Z"},
		{ID: "p2", UserID: "u2", Content: "second", CreatedAt: "2025-01-01T00:00:01.000Z"},
	}
	store := newFakeFeedStore(posts...)
	store.feedCount = 0 // no materialized rows → fallback
	svc := NewFeedService(store)

	result, err := svc.ListFeed(context.Background(), "viewer1", nil, 10)
	if err != nil {
		t.Fatalf("ListFeed: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("len(result) = %d, want 2", len(result))
	}
}

func TestFeedService_ListFeed_MaterializedPath(t *testing.T) {
	material := []model.Post{
		{ID: "p3", UserID: "u3", Content: "materialized", CreatedAt: "2025-02-01T00:00:00.000Z"},
	}
	store := newFakeFeedStore() // live-join returns nothing
	store.materialRows = material
	store.feedCount = 1 // has materialized rows → use primary path
	svc := NewFeedService(store)

	result, err := svc.ListFeed(context.Background(), "viewer1", nil, 10)
	if err != nil {
		t.Fatalf("ListFeed: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("len(result) = %d, want 1 (materialized)", len(result))
	}
	if result[0].ID != "p3" {
		t.Errorf("result[0].ID = %q, want p3", result[0].ID)
	}
}

func TestFeedService_ListFeed_WithCursor(t *testing.T) {
	posts := []model.Post{
		{ID: "p1", UserID: "u1", Content: "first", CreatedAt: "2025-01-01T00:00:02.000Z"},
		{ID: "p2", UserID: "u2", Content: "second", CreatedAt: "2025-01-01T00:00:01.000Z"},
	}
	store := newFakeFeedStore(posts...)
	store.feedCount = 0
	svc := NewFeedService(store)

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
	store := newFakeFeedStore()
	store.feedCount = 0
	svc := NewFeedService(store)

	result, err := svc.ListFeed(context.Background(), "viewer1", nil, 10)
	if err != nil {
		t.Fatalf("ListFeed: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("len(result) = %d, want 0", len(result))
	}
}

func TestFeedService_ListFeed_CountError_Propagates(t *testing.T) {
	store := newFakeFeedStore()
	store.countErr = errors.New("db failure")
	svc := NewFeedService(store)

	_, err := svc.ListFeed(context.Background(), "viewer1", nil, 10)
	if err == nil {
		t.Error("expected count error to propagate, got nil")
	}
}

func TestFeedService_ListFeed_StoreError_Propagates(t *testing.T) {
	store := newFakeFeedStore()
	store.listErr = errors.New("db failure")
	store.feedCount = 0
	svc := NewFeedService(store)

	_, err := svc.ListFeed(context.Background(), "viewer1", nil, 10)
	if err == nil {
		t.Error("expected store error to propagate, got nil")
	}
}
