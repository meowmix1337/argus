package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/meowmix1337/argus/backend/internal/model"
	platformcache "github.com/meowmix1337/argus/backend/internal/platform/cache"
)

func TestRelativeTime(t *testing.T) {
	now := time.Now()
	cases := []struct {
		label string
		t     time.Time
		want  string
	}{
		{"zero time", time.Time{}, "recently"},
		{"10s ago", now.Add(-10 * time.Second), "just now"},
		{"30m ago", now.Add(-30 * time.Minute), "30m ago"},
		{"3h ago", now.Add(-3 * time.Hour), "3h ago"},
		{"2d ago", now.Add(-48 * time.Hour), "2d ago"},
	}
	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			if got := relativeTime(tc.t); got != tc.want {
				t.Errorf("relativeTime = %q, want %q", got, tc.want)
			}
		})
	}
}

// ---- NewsService.Fetch ----

func TestNewsService_Fetch_CacheHit(t *testing.T) {
	cache := platformcache.NewCacheService()
	cache.Set("news", []model.NewsCategory{{Name: "general"}}, time.Minute)

	svc := NewNewsService(
		&fakeHTTPClient{err: fmt.Errorf("HTTP must not be called on cache hit")},
		"key", cache,
	)
	cats, err := svc.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(cats) != 1 || cats[0].Name != "general" {
		t.Errorf("expected cached categories, got: %+v", cats)
	}
}

func TestNewsService_Fetch_NoAPIKey_ReturnsError(t *testing.T) {
	svc := NewNewsService(&fakeHTTPClient{}, "", platformcache.NewCacheService())
	if _, err := svc.Fetch(context.Background()); err == nil {
		t.Error("expected error when API key is empty")
	}
}

// TestNewsService_FetchCategory_Success calls the unexported fetchCategory directly
// to verify HTTP parsing without running the 8-second category loop.
func TestNewsService_FetchCategory_Success(t *testing.T) {
	resp := gNewsResponse{
		Articles: []struct {
			Title       string `json:"title"`
			URL         string `json:"url"`
			PublishedAt string `json:"publishedAt"`
			Source      struct {
				Name string `json:"name"`
			} `json:"source"`
		}{
			{Title: "Go 1.24 Released", URL: "https://example.com", PublishedAt: "2024-01-01T12:00:00Z",
				Source: struct {
					Name string `json:"name"`
				}{Name: "The Verge"}},
		},
	}
	svc := NewNewsService(&fakeHTTPClient{responseBody: resp}, "test-key", platformcache.NewCacheService())

	items, err := svc.fetchCategory(context.Background(), "technology")
	if err != nil {
		t.Fatalf("fetchCategory: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Title != "Go 1.24 Released" {
		t.Errorf("Title = %q, want %q", items[0].Title, "Go 1.24 Released")
	}
	if items[0].Source != "The Verge" {
		t.Errorf("Source = %q, want %q", items[0].Source, "The Verge")
	}
}

// TestNewsService_FetchAllCategories_ContextCancellation verifies that fetchAllCategories
// stops early when the context is cancelled, without sleeping through the full 8-second loop.
// The pre-cancelled context fires on the i=1 select immediately.
func TestNewsService_FetchAllCategories_ContextCancellation(t *testing.T) {
	svc := NewNewsService(&fakeHTTPClient{responseBody: gNewsResponse{}}, "test-key", platformcache.NewCacheService())
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancel; first iteration (i=0) has no sleep so it still runs
	_, err := svc.fetchAllCategories(ctx)
	if err == nil {
		t.Error("expected context error from cancelled context")
	}
}

func TestNewsService_FetchCategory_HTTPError_Propagates(t *testing.T) {
	svc := NewNewsService(&fakeHTTPClient{err: fmt.Errorf("timeout")}, "key", platformcache.NewCacheService())
	if _, err := svc.fetchCategory(context.Background(), "general"); err == nil {
		t.Error("expected error on HTTP failure")
	}
}

// TestNewsService_FetchAllCategories_FetchCategoryError_ContinuesWithEmpty covers the
// slog.Warn + items=[]model.NewsItem{} path when fetchCategory fails for the first category.
// Pre-cancelling the context avoids sleeping through the 8-category loop.
func TestNewsService_FetchAllCategories_FetchCategoryError_ContinuesWithEmpty(t *testing.T) {
	svc := NewNewsService(&fakeHTTPClient{err: fmt.Errorf("http error")}, "test-key", platformcache.NewCacheService())
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancel: i=0 runs (fetchCategory fails, items=[]), i=1 fires ctx.Done()
	_, err := svc.fetchAllCategories(ctx)
	if err == nil {
		t.Error("expected context error from cancelled context, got nil")
	}
}

// TestNewsService_Fetch_FetchAllCategoriesError_Propagates verifies that when
// fetchAllCategories returns an error (via pre-cancelled context), Fetch propagates it.
func TestNewsService_Fetch_FetchAllCategoriesError_Propagates(t *testing.T) {
	svc := NewNewsService(&fakeHTTPClient{responseBody: gNewsResponse{}}, "test-key", platformcache.NewCacheService())
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancel so fetchAllCategories returns ctx.Err() at i=1
	if _, err := svc.Fetch(ctx); err == nil {
		t.Error("expected error when fetchAllCategories fails, got nil")
	}
}

// TestNewsService_FetchCategory_RFC3339NanoFallback covers the second parse attempt
// when a publishedAt timestamp includes sub-second precision (fails RFC3339, passes RFC3339Nano).
func TestNewsService_FetchCategory_RFC3339NanoFallback(t *testing.T) {
	resp := gNewsResponse{
		Articles: []struct {
			Title       string `json:"title"`
			URL         string `json:"url"`
			PublishedAt string `json:"publishedAt"`
			Source      struct {
				Name string `json:"name"`
			} `json:"source"`
		}{
			{Title: "Nano Test", URL: "https://example.com",
				PublishedAt: "2024-01-01T12:00:00.123456789Z",
				Source: struct {
					Name string `json:"name"`
				}{Name: "AP"}},
		},
	}
	svc := NewNewsService(&fakeHTTPClient{responseBody: resp}, "key", platformcache.NewCacheService())
	items, err := svc.fetchCategory(context.Background(), "general")
	if err != nil {
		t.Fatalf("fetchCategory: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Time == "recently" {
		t.Error("expected a relative time (not 'recently') for a valid RFC3339Nano timestamp")
	}
}

// TestNewsService_FetchCategory_InvalidDate_FallsBackToRecently covers the path where
// both RFC3339 and RFC3339Nano parsing fail, resulting in a zero time → "recently".
func TestNewsService_FetchCategory_InvalidDate_FallsBackToRecently(t *testing.T) {
	resp := gNewsResponse{
		Articles: []struct {
			Title       string `json:"title"`
			URL         string `json:"url"`
			PublishedAt string `json:"publishedAt"`
			Source      struct {
				Name string `json:"name"`
			} `json:"source"`
		}{
			{Title: "Bad Date", URL: "https://example.com",
				PublishedAt: "not-a-date",
				Source: struct {
					Name string `json:"name"`
				}{Name: "BBC"}},
		},
	}
	svc := NewNewsService(&fakeHTTPClient{responseBody: resp}, "key", platformcache.NewCacheService())
	items, err := svc.fetchCategory(context.Background(), "general")
	if err != nil {
		t.Fatalf("fetchCategory: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Time != "recently" {
		t.Errorf("Time = %q, want %q for invalid date", items[0].Time, "recently")
	}
}
