package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/meowmix1337/argus/backend/internal/model"
	platformcache "github.com/meowmix1337/argus/backend/internal/platform/cache"
)

func TestQuotesService_Fetch_CacheHit(t *testing.T) {
	cache := platformcache.NewCacheService()
	cached := model.Quote{Text: "Be the change.", Author: "Gandhi"}
	cache.Set("quote", cached, time.Minute)

	// HTTP client errors on call — proves the cache short-circuits the API.
	svc := NewQuotesService(
		&fakeHTTPClient{err: fmt.Errorf("HTTP client must not be called on cache hit")},
		"key", cache,
	)

	q, err := svc.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if q.Text != "Be the change." || q.Author != "Gandhi" {
		t.Errorf("cache hit returned wrong quote: %+v", q)
	}
}

func TestQuotesService_Fetch_NoAPIKey(t *testing.T) {
	// Without an API key the service must fail fast rather than making a
	// request that will be rejected by the provider with a 401.
	svc := NewQuotesService(&fakeHTTPClient{}, "", platformcache.NewCacheService())
	_, err := svc.Fetch(context.Background())
	if err == nil {
		t.Fatal("expected error when API key is empty")
	}
}

func TestQuotesService_Fetch_EmptyAPIResponse(t *testing.T) {
	// API Ninjas may return an empty array on transient errors.
	svc := NewQuotesService(
		&fakeHTTPClient{responseBody: []apiNinjasQuote{}},
		"test-key",
		platformcache.NewCacheService(),
	)
	_, err := svc.Fetch(context.Background())
	if err == nil {
		t.Fatal("expected error when API returns empty quotes array")
	}
}

func TestQuotesService_Fetch_ReturnsFirstQuote(t *testing.T) {
	// The API may return multiple quotes; we only surface the first.
	quotes := []apiNinjasQuote{
		{Quote: "The only way is through.", Author: "Robert Frost"},
		{Quote: "This should not appear.", Author: "Nobody"},
	}
	svc := NewQuotesService(
		&fakeHTTPClient{responseBody: quotes},
		"test-key",
		platformcache.NewCacheService(),
	)

	q, err := svc.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if q.Text != "The only way is through." {
		t.Errorf("Text = %q, want first quote text", q.Text)
	}
	if q.Author != "Robert Frost" {
		t.Errorf("Author = %q, want %q", q.Author, "Robert Frost")
	}
}

func TestQuotesService_Fetch_PopulatesCache(t *testing.T) {
	quotes := []apiNinjasQuote{{Quote: "Test quote.", Author: "Author"}}
	cache := platformcache.NewCacheService()
	svc := NewQuotesService(&fakeHTTPClient{responseBody: quotes}, "test-key", cache)

	if _, err := svc.Fetch(context.Background()); err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if _, ok := cache.Get("quote"); !ok {
		t.Error("expected quote to be cached after a successful fetch")
	}
}
